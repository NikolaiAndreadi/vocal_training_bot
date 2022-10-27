package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/exp/slices"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	tele "gopkg.in/telebot.v3"
)

func SetupStates(db *pgxpool.Pool) *FSM {
	fsm := NewFSM(db)
	setupSurveyStateGroup(fsm)
	return fsm
}

const (
	SurveySGStartSurveyReqName = "SurveyStateGroup_StartSurveyReqName"
	SurveySGSetAge             = "SurveyStateGroup_SetAge"
	SurveySGSetCity            = "SurveyStateGroup_SetCity"
	SurveySGSetTimezone        = "SurveyStateGroup_SetTimezone"
	SurveySGSetExperience      = "SurveyStateGroup_SetExperience"

	SurveySGVarName        = "Name"
	SurveySGVarAge         = "Age"
	SurveySGVarCity        = "City"
	SurveySGVarTimezoneRaw = "TimezoneRaw"
	SurveySGVarTimezoneStr = "TimezoneTxt"
)

var (
	matchingPatternName = regexp.MustCompile("[ЁёА-яA-Za-z ]{2,50}")
	matchingPatternCity = regexp.MustCompile("[ЁёА-яA-Za-z ]{2,50}")
	matchingPatternTime = regexp.MustCompile("[0-9]?[0-9]:[0-9][0-9]")

	SurveySGSetExperiencePossibleVariants = []string{"без опыта", "менее 1 года", "1-2 года", "2-3 года", "3-5 лет", "более 5 лет"}
	SurveySGSetExperienceMenu             = ReplyMenuConstructor(SurveySGSetExperiencePossibleVariants, 2)
)

func calcTimezoneByTimeShift(userHours, userMinutes int) (utcTimezone string, utcMinutesShift string, err error) {
	userMinutes = userMinutes + userHours*60

	utcTime := time.Now().UTC()
	utcMinutes := utcTime.Minute() + utcTime.Hour()*60

	deltaMinutes, err := time.ParseDuration(fmt.Sprintf("%dm", userMinutes-utcMinutes))
	if err != nil {
		err = fmt.Errorf("calcTimezoneByTimeshift(%d, %d): %w", userHours, userMinutes, err)
		return
	}
	deltaMinutes = deltaMinutes.Round(30 * time.Minute)
	utcMinutesShift = fmt.Sprintf("%f.0f", deltaMinutes.Minutes()) // save output

	// utcTimezone representation
	var offsetSign rune
	if deltaMinutes.Minutes() < 0 {
		offsetSign = '-'
	} else {
		offsetSign = '+'
	}

	deltaHoursFmt := deltaMinutes / time.Hour
	deltaMinutes -= deltaHoursFmt * time.Hour
	deltaMinutesFmt := deltaMinutes / time.Minute
	utcTimezone = fmt.Sprintf("UTC%c%02d:%02d", offsetSign, deltaHoursFmt.Abs(), deltaMinutesFmt.Abs())

	return
}

func setupSurveyStateGroup(fsm *FSM) {
	fsm.AddState(SurveySGStartSurveyReqName,
		"Привет! Чтобы пользоваться ботом надо сначала пройти опрос из нескольких вопросов.\n\n(1/5) Назови, пожалуйста, своё имя?",
		func(c tele.Context) (nextState string, err error) {
			// TODO: Refactor this as FSM.state.messageChan == text, img, audio...
			name := c.Message().Text
			if name == "" {
				return ResumeState, c.Send("Не могу распознать ответ. Попробуй еще раз =)")
			}

			// VALIDATION
			if ok := matchingPatternName.MatchString(name); !ok {
				return ResumeState, c.Send("Имя должно включать только русские или английские буквы и быть 2 - 50 символов")
			}
			name = cases.Title(language.Tag{}).String(name)
			err = fsm.SetStateVar(c, SurveySGVarName, name)

			err = multierror.Append(err, c.Send("Приятно познакомиться, %s", name))
			return SurveySGSetAge, err
		})

	fsm.AddState(SurveySGSetAge,
		"(2/5) Теперь скажи, сколько тебе лет?",
		func(c tele.Context) (nextState string, err error) {
			ageText := c.Message().Text
			if ageText == "" {
				return ResumeState, c.Send("Не могу распознать ответ. Попробуй еще раз =)")
			}

			// VALIDATION
			age, err := strconv.Atoi(ageText)
			if err != nil {
				return ResumeState, c.Send("Возраст должен быть числом больше нуля, попробуй еще раз =)")
			}
			if age <= 0 {
				return ResumeState, c.Send("Возраст должен быть числом больше нуля, попробуй еще раз =)")
			}
			if age > 100 {
				return ResumeState, c.Send("Да ты совсем взрослый! Давай по-чесноку, сколько лет?")
			}

			err = fsm.SetStateVar(c, SurveySGVarAge, ageText)
			return SurveySGSetCity, err
		})

	fsm.AddState(SurveySGSetCity,
		"(3/5) Отлично! А в каком городе живешь?",
		func(c tele.Context) (nextState string, err error) {
			city := c.Message().Text
			if city == "" {
				return ResumeState, c.Send("Не могу распознать ответ. Попробуй еще раз =)")
			}

			if ok := matchingPatternCity.MatchString(city); !ok {
				err := c.Send("Не могу распознать ответ. Попробуй еще раз!")
				return ResumeState, err
			}
			city = cases.Title(language.Tag{}).String(city)

			err = fsm.SetStateVar(c, SurveySGVarCity, city)
			return SurveySGSetTimezone, err
		})

	fsm.AddState(SurveySGSetTimezone,
		"(4/5) Сколько сейчас времени по твоим часам? Надо написать часы:минуты, например, 23:15. Это надо чтобы понять в каком часовом поясе ты находишься.",
		func(c tele.Context) (nextState string, err error) {
			userTimeTxt := c.Message().Text
			if userTimeTxt == "" {
				return ResumeState, c.Send("Не могу распознать ответ. Попробуй еще раз =)")
			}

			// VALIDATION
			if ok := matchingPatternTime.MatchString(userTimeTxt); !ok {
				return ResumeState, c.Send("Не могу распознать ответ. Надо написать в формате ЧЧ:ММ, например, 20:55")
			}
			userHoursMinutes := strings.Split(userTimeTxt, ":")
			if len(userHoursMinutes) != 2 {
				return ResumeState, c.Send("Не могу распознать ответ. Надо указать только часы и минуты формате ЧЧ:ММ, например, 20:55")
			}

			// str -> int
			userHours, err := strconv.Atoi(userHoursMinutes[0])
			if err != nil {
				return ResumeState, c.Send("Не могу распознать ответ. Надо написать в формате ЧЧ:ММ, например, 20:55")
			}
			userMinutes, err := strconv.Atoi(userHoursMinutes[1])
			if err != nil {
				return ResumeState, c.Send("Не могу распознать ответ. Надо написать в формате ЧЧ:ММ, например, 20:55")
			}

			// post validation
			if userHours > 23 {
				return ResumeState, c.Send("Максимальный час - 23. Надо написать в формате ЧЧ:ММ, например, 20:55")
			}
			if userMinutes > 59 {
				return ResumeState, c.Send("Максимальная минута - 59. Надо написать в формате ЧЧ:ММ, например, 20:55")
			}

			// calculations and saving data
			utcTimezone, utcMinutesShift, err := calcTimezoneByTimeShift(userHours, userMinutes)
			if err != nil {
				return ResumeState, c.Send("Не могу распознать ответ. Надо написать в формате ЧЧ:ММ, например, 20:55")
			}

			err1 := fsm.SetStateVar(c, SurveySGVarTimezoneRaw, utcMinutesShift)
			err2 := fsm.SetStateVar(c, SurveySGVarTimezoneStr, utcTimezone)

			err3 := c.Send(fmt.Sprintf("Получается, твой часовой пояс - %s", utcTimezone))

			return SurveySGSetExperience, multierror.Append(err1, err2, err3)
		})

	fsm.AddState(SurveySGSetExperience,
		"(5/5) Сколько занимаешься вокалом?",
		func(c tele.Context) (nextState string, err error) {
			expVariant := c.Text()
			if expVariant == "" {
				return ResumeState, c.Send("Не могу распознать ответ. Выбери вариант из списка")
			}
			expVariant = strings.ToLower(expVariant)
			if ok := slices.Contains(SurveySGSetExperiencePossibleVariants, expVariant); !ok {
				return ResumeState, c.Send("Не могу распознать ответ. Выбери вариант из списка")
			}

			// CLOSE SURVEY
			values, mErr := fsm.GetStateVars(c)

			name, ok := values[SurveySGVarName]
			if !ok {
				mErr = multierror.Append(mErr, fmt.Errorf("state %s[%d]: Can't decode %s", SurveySGSetExperience, c.Sender().ID, SurveySGVarName))
			}
			ageStr, ok := values[SurveySGVarAge]
			if !ok {
				mErr = multierror.Append(mErr, fmt.Errorf("state %s[%d]: Can't decode %s", SurveySGSetExperience, c.Sender().ID, SurveySGVarAge))
			}
			age, err := strconv.Atoi(ageStr)
			if err != nil {
				mErr = multierror.Append(mErr, fmt.Errorf("state %s[%d]: Can't atoi %s", SurveySGSetExperience, c.Sender().ID, SurveySGVarAge))
			}
			city, ok := values[SurveySGVarCity]
			if !ok {
				mErr = multierror.Append(mErr, fmt.Errorf("state %s[%d]: Can't decode %s", SurveySGSetExperience, c.Sender().ID, SurveySGVarCity))
			}
			timezoneTxt, ok := values[SurveySGVarTimezoneStr]
			if !ok {
				mErr = multierror.Append(mErr, fmt.Errorf("state %s[%d]: Can't decode %s", SurveySGSetExperience, c.Sender().ID, SurveySGVarTimezoneStr))
			}
			timezoneRawStr, ok := values[SurveySGVarTimezoneRaw]
			if !ok {
				mErr = multierror.Append(mErr, fmt.Errorf("state %s[%d]: Can't decode %s", SurveySGSetExperience, c.Sender().ID, SurveySGVarTimezoneRaw))
			}
			timezoneRaw, err := strconv.Atoi(timezoneRawStr)
			if err != nil {
				mErr = multierror.Append(mErr, fmt.Errorf("state %s[%d]: Can't atoi %s", SurveySGSetExperience, c.Sender().ID, SurveySGVarTimezoneRaw))
			}
			joinTime := time.Now().UTC()

			_, err = DB.Exec(context.Background(), `
				INSERT INTO users(user_id, username, age, city, timezone_raw, timezone_txt, experience, join_dt)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				`, c.Sender().ID, name, age, city, timezoneRaw, timezoneTxt, expVariant, joinTime)

			if err != nil {
				mErr = multierror.Append(mErr, fmt.Errorf("state %s[%d]: Can't exec insert into db", SurveySGSetExperience, c.Sender().ID))
			}

			err = c.Send("Спасибо! Ты зарегистрирован в системе бота и теперь тебе доступна его функциональность!",
				&tele.ReplyMarkup{RemoveKeyboard: true})
			if err != nil {
				mErr = multierror.Append(mErr, fmt.Errorf("state %s[%d]: Can't send a message", SurveySGSetExperience, c.Sender().ID))
			}

			return ResetState, mErr
		}, SurveySGSetExperienceMenu)
}
