package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/exp/slices"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	tele "gopkg.in/telebot.v3"
)

func SetupStates(db *pgxpool.Pool) *FSM {
	fsm := NewFSM(db)
	setupSurveyStateGroup(fsm)
	setupSettingsStateGroup(fsm)
	return fsm
}

const (
	SurveySGStartSurveyReqName = "SurveyStateGroup_StartSurveyReqName"
	surveySGSetAge             = "SurveyStateGroup_SetAge"
	surveySGSetCity            = "SurveyStateGroup_SetCity"
	surveySGSetTimezone        = "SurveyStateGroup_SetTimezone"
	surveySGSetExperience      = "SurveyStateGroup_SetExperience"

	surveySGVarName        = "Name"
	surveySGVarAge         = "Age"
	surveySGVarCity        = "City"
	surveySGVarTimezoneRaw = "TimezoneRaw"
	surveySGVarTimezoneStr = "TimezoneTxt"

	SettingsSGSetName       = "SettingsStateGroup_SetName"
	SettingsSGSetAge        = "SettingsStateGroup_SetAge"
	SettingsSGSetCity       = "SettingsStateGroup_SetCity"
	SettingsSGSetTimezone   = "SettingsStateGroup_SetTimezone"
	SettingsSGSetExperience = "SettingsStateGroup_SetExperience"
)

var (
	matchingPatternName = regexp.MustCompile("[ЁёА-яA-Za-z ]{2,50}")
	matchingPatternCity = regexp.MustCompile("[ЁёА-яA-Za-z ]{2,50}")
	matchingPatternTime = regexp.MustCompile("[0-9]?[0-9]:[0-9][0-9]")

	SurveySGSetExperiencePossibleVariants = []string{"без опыта", "менее 1 года", "1-2 года", "2-3 года", "3-5 лет", "более 5 лет"}
	SurveySGSetExperienceMenu             = ReplyMenuConstructor(SurveySGSetExperiencePossibleVariants, 2, true)
)

func setupSettingsStateGroup(fsm *FSM) {
	fsm.AddState(SettingsSGSetName,
		"Введи новое имя",
		func(c tele.Context) (nextState string, err error) {
			name, s, err, exit := validateName(c)
			if exit {
				return s, err
			}

			_, err = DB.Exec(context.Background(), `
				UPDATE users
				SET username = $1
				WHERE user_id = $2
				`, name, c.Sender().ID)

			if err != nil {
				fmt.Println(fmt.Errorf("state %s[%d]: Can't exec insert into db", SettingsSGSetName, c.Sender().ID))
			}

			return ResetState, c.Send("Имя изменено")
		})

	fsm.AddState(SettingsSGSetAge,
		"Введи новый возраст",
		func(c tele.Context) (nextState string, err error) {
			ageText, s, err, exit := validateAge(c)
			if exit {
				return s, err
			}

			_, err = DB.Exec(context.Background(), `
				UPDATE users
				SET age = $1
				WHERE user_id = $2
				`, ageText, c.Sender().ID)

			if err != nil {
				fmt.Println(fmt.Errorf("state %s[%d]: Can't exec insert into db", SettingsSGSetAge, c.Sender().ID))
			}

			return ResetState, c.Send("Возраст изменен")
		})

	fsm.AddState(SettingsSGSetCity,
		"Введи новый город",
		func(c tele.Context) (nextState string, err error) {
			city, s, err, exit := validateCity(c)
			if exit {
				return s, err
			}

			_, err = DB.Exec(context.Background(), `
				UPDATE users
				SET city = $1
				WHERE user_id = $2
				`, city, c.Sender().ID)

			if err != nil {
				fmt.Println(fmt.Errorf("state %s[%d]: Can't exec insert into db", SettingsSGSetCity, c.Sender().ID))
			}

			return ResetState, c.Send("Город изменен")
		})

	fsm.AddState(SettingsSGSetTimezone,
		"Введи свое время (в формате ЧЧ:ММ, например 12:15 или 9:15)",
		func(c tele.Context) (nextState string, err error) {
			utcTimezone, utcMinutesShift, s, err, exit := validateTimezone(c)
			if exit {
				return s, err
			}

			_, err = DB.Exec(context.Background(), `
				UPDATE users
				SET timezone_txt = $1,
				    timezone_raw = $2
				WHERE user_id = $3
				`, utcTimezone, utcMinutesShift, c.Sender().ID)

			if err != nil {
				fmt.Println(fmt.Errorf("state %s[%d]: Can't exec insert into db", SettingsSGSetTimezone, c.Sender().ID))
			}

			return ResetState, c.Send(fmt.Sprintf("Получается, твой часовой пояс - %s", utcTimezone))
		})

	fsm.AddState(SettingsSGSetExperience,
		"Сколько уже занимаешься вокалом?",
		func(c tele.Context) (nextState string, err error) {
			expVariant, s, err, exit := validateExperience(c)
			if exit {
				return s, err
			}

			_, err = DB.Exec(context.Background(), `
				UPDATE users
				SET experience = $1
				WHERE user_id = $2
				`, expVariant, c.Sender().ID)

			if err != nil {
				fmt.Println(fmt.Errorf("state %s[%d]: Can't exec insert into db", SettingsSGSetExperience, c.Sender().ID))
			}

			return ResetState, c.Send("Опыт обновлен", &tele.ReplyMarkup{RemoveKeyboard: true}, MainUserMenu)
		}, SurveySGSetExperienceMenu)
}

func setupSurveyStateGroup(fsm *FSM) {
	fsm.AddState(SurveySGStartSurveyReqName,
		"Привет! Чтобы пользоваться ботом надо сначала пройти опрос из нескольких вопросов.\n\n(1/5) Назови, пожалуйста, своё имя?",
		func(c tele.Context) (nextState string, err error) {
			name, s, err, exit := validateName(c)
			if exit {
				return s, err
			}

			err = fsm.SetStateVar(c, surveySGVarName, name)
			// TODO: add log here
			if err != nil {
				fmt.Println(err)
			}

			err = c.Send(fmt.Sprintf("Приятно познакомиться, %s", name))

			return surveySGSetAge, err
		})

	fsm.AddState(surveySGSetAge,
		"(2/5) Теперь скажи, сколько тебе лет?",
		func(c tele.Context) (nextState string, err error) {
			ageText, s, err, exit := validateAge(c)
			if exit {
				return s, err
			}

			err = fsm.SetStateVar(c, surveySGVarAge, ageText)
			return surveySGSetCity, err
		})

	fsm.AddState(surveySGSetCity,
		"(3/5) Отлично! А в каком городе живешь?",
		func(c tele.Context) (nextState string, err error) {
			city, s, err, exit := validateCity(c)
			if exit {
				return s, err
			}

			err = fsm.SetStateVar(c, surveySGVarCity, city)
			return surveySGSetTimezone, err
		})

	fsm.AddState(surveySGSetTimezone,
		"(4/5) Сколько сейчас времени по твоим часам? Надо написать часы:минуты, например, 23:15. Это надо чтобы понять в каком часовом поясе ты находишься.",
		func(c tele.Context) (nextState string, err error) {
			utcTimezone, utcMinutesShift, s, err, exit := validateTimezone(c)
			if exit {
				return s, err
			}

			if err = fsm.SetStateVar(c, surveySGVarTimezoneRaw, utcMinutesShift); err != nil {
				// TODO: logger here
				fmt.Println(fmt.Errorf("%s[%d]: can't save %s", surveySGSetTimezone, c.Sender().ID, surveySGVarTimezoneRaw))
			}
			if err = fsm.SetStateVar(c, surveySGVarTimezoneStr, utcTimezone); err != nil {
				// TODO: logger here
				fmt.Println(fmt.Errorf("%s[%d]: can't save %s", surveySGSetTimezone, c.Sender().ID, surveySGVarTimezoneStr))
			}

			err = c.Send(fmt.Sprintf("Получается, твой часовой пояс - %s", utcTimezone))

			return surveySGSetExperience, err
		})

	fsm.AddState(surveySGSetExperience,
		"(5/5) Сколько занимаешься вокалом?",
		func(c tele.Context) (nextState string, err error) {
			expVariant, s, err, exit := validateExperience(c)
			if exit {
				return s, err
			}

			// CLOSE SURVEY
			values, err := fsm.GetStateVars(c)
			if err != nil {
				fmt.Println(fmt.Errorf("state %s[%d]: Can't extract vars", surveySGSetExperience, c.Sender().ID))
			}

			name, ok := values[surveySGVarName]
			if !ok {
				fmt.Println(fmt.Errorf("state %s[%d]: Can't decode %s", surveySGSetExperience, c.Sender().ID, surveySGVarName))
			}
			ageStr, ok := values[surveySGVarAge]
			if !ok {
				fmt.Println(fmt.Errorf("state %s[%d]: Can't decode %s", surveySGSetExperience, c.Sender().ID, surveySGVarAge))
			}
			age, err := strconv.Atoi(ageStr)
			if err != nil {
				fmt.Println(fmt.Errorf("state %s[%d]: Can't atoi %s", surveySGSetExperience, c.Sender().ID, surveySGVarAge))
			}
			city, ok := values[surveySGVarCity]
			if !ok {
				fmt.Println(fmt.Errorf("state %s[%d]: Can't decode %s", surveySGSetExperience, c.Sender().ID, surveySGVarCity))
			}
			timezoneTxt, ok := values[surveySGVarTimezoneStr]
			if !ok {
				fmt.Println(fmt.Errorf("state %s[%d]: Can't decode %s", surveySGSetExperience, c.Sender().ID, surveySGVarTimezoneStr))
			}
			timezoneRawStr, ok := values[surveySGVarTimezoneRaw]
			if !ok {
				fmt.Println(fmt.Errorf("state %s[%d]: Can't decode %s", surveySGSetExperience, c.Sender().ID, surveySGVarTimezoneRaw))
			}
			timezoneRaw, err := strconv.Atoi(timezoneRawStr)
			if err != nil {
				fmt.Println(fmt.Errorf("state %s[%d]: Can't atoi %s", surveySGSetExperience, c.Sender().ID, surveySGVarTimezoneRaw))
			}
			joinTime := time.Now().UTC()

			_, err = DB.Exec(context.Background(), `
				INSERT INTO users(user_id, username, age, city, timezone_raw, timezone_txt, experience, join_dt)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				`, c.Sender().ID, name, age, city, timezoneRaw, timezoneTxt, expVariant, joinTime)

			if err != nil {
				fmt.Println(fmt.Errorf("state %s[%d]: Can't exec insert into db", surveySGSetExperience, c.Sender().ID))
			}

			err = c.Send("Спасибо! Ты зарегистрирован в системе бота и теперь тебе доступна его функциональность!",
				&tele.ReplyMarkup{RemoveKeyboard: true}, MainUserMenu)

			return ResetState, err
		}, SurveySGSetExperienceMenu)
}

func validateExperience(c tele.Context) (expVariant string, state string, err error, exit bool) {
	expVariant = c.Text()
	if expVariant == "" {
		return "", ResumeState, c.Send("Не могу распознать ответ. Выбери вариант из списка"), true
	}
	expVariant = strings.ToLower(expVariant)
	if ok := slices.Contains(SurveySGSetExperiencePossibleVariants, expVariant); !ok {
		return "", ResumeState, c.Send("Не могу распознать ответ. Выбери вариант из списка"), true
	}
	return expVariant, "", nil, false
}

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
	utcMinutesShift = strconv.Itoa(int(deltaMinutes.Minutes())) // save output

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

func validateTimezone(c tele.Context) (utcTimezone string, utcMinutesShift string, state string, err error, exit bool) {
	userTimeTxt := c.Message().Text
	if userTimeTxt == "" {
		return "", "", ResumeState, c.Send("Не могу распознать ответ. Попробуй еще раз =)"), true
	}

	// VALIDATION
	if ok := matchingPatternTime.MatchString(userTimeTxt); !ok {
		return "", "", ResumeState, c.Send("Не могу распознать ответ. Надо написать в формате ЧЧ:ММ, например, 20:55"), true
	}
	userHoursMinutes := strings.Split(userTimeTxt, ":")
	if len(userHoursMinutes) != 2 {
		return "", "", ResumeState, c.Send("Не могу распознать ответ. Надо указать только часы и минуты формате ЧЧ:ММ, например, 20:55"), true
	}

	// str -> int
	userHours, err := strconv.Atoi(userHoursMinutes[0])
	if err != nil {
		return "", "", ResumeState, c.Send("Не могу распознать ответ. Надо написать в формате ЧЧ:ММ, например, 20:55"), true
	}
	userMinutes, err := strconv.Atoi(userHoursMinutes[1])
	if err != nil {
		return "", "", ResumeState, c.Send("Не могу распознать ответ. Надо написать в формате ЧЧ:ММ, например, 20:55"), true
	}

	// post validation
	if userHours > 23 {
		return "", "", ResumeState, c.Send("Максимальный час - 23. Надо написать в формате ЧЧ:ММ, например, 20:55"), true
	}
	if userMinutes > 59 {
		return "", "", ResumeState, c.Send("Максимальная минута - 59. Надо написать в формате ЧЧ:ММ, например, 20:55"), true
	}

	// calculations and saving data
	utcTimezone, utcMinutesShift, err = calcTimezoneByTimeShift(userHours, userMinutes)
	if err != nil {
		return "", "", ResumeState, c.Send("Не могу распознать ответ. Надо написать в формате ЧЧ:ММ, например, 20:55"), true
	}
	return utcTimezone, utcMinutesShift, "", nil, false
}

func validateCity(c tele.Context) (city string, state string, err error, exit bool) {
	city = c.Message().Text
	if city == "" {
		return "", ResumeState, c.Send("Не могу распознать ответ. Попробуй еще раз =)"), true
	}

	if ok := matchingPatternCity.MatchString(city); !ok {
		err := c.Send("Не могу распознать ответ. Попробуй еще раз!")
		return "", ResumeState, err, true
	}
	city = cases.Title(language.Tag{}).String(city)
	return city, "", nil, false
}

func validateAge(c tele.Context) (ageText string, state string, err error, exit bool) {
	ageText = c.Message().Text
	if ageText == "" {
		return "", ResumeState, c.Send("Не могу распознать ответ. Попробуй еще раз =)"), true
	}

	// VALIDATION
	age, err := strconv.Atoi(ageText)
	if err != nil {
		return "", ResumeState, c.Send("Возраст должен быть числом больше нуля, попробуй еще раз =)"), true
	}
	if age <= 0 {
		return "", ResumeState, c.Send("Возраст должен быть числом больше нуля, попробуй еще раз =)"), true
	}
	if age > 100 {
		return "", ResumeState, c.Send("Да ты совсем взрослый! Давай по-чесноку, сколько лет?"), true
	}
	return ageText, "", nil, false
}

func validateName(c tele.Context) (name string, state string, err error, exit bool) {
	// TODO: Refactor this as FSM.state.messageChan == text, img, audio...
	name = c.Message().Text
	if name == "" {
		return "", ResumeState, c.Send("Не могу распознать ответ. Попробуй еще раз =)"), true
	}

	// VALIDATION
	if ok := matchingPatternName.MatchString(name); !ok {
		return "", ResumeState, c.Send("Имя должно включать только русские или английские буквы и быть 2 - 50 символов"), true
	}
	name = cases.Title(language.Tag{}).String(name)
	return name, "", nil, false
}
