package main

import (
	"context"
	"fmt"
	"golang.org/x/exp/slices"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	tele "gopkg.in/telebot.v3"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	fsm = NewFSM()

	matchingPatternName = regexp.MustCompile("[ЁёА-яA-Za-z ]{2,50}")
	matchingPatternCity = regexp.MustCompile("[ЁёА-яA-Za-z ]{2,50}")
	matchingPatternTime = regexp.MustCompile("[0-9]?[0-9]:[0-9][0-9]")
)

func InitBot(cfg Config) *tele.Bot {
	teleCfg := tele.Settings{
		Token: cfg.Bot.Token,
	}
	bot, err := tele.NewBot(teleCfg)
	if err != nil {
		panic(fmt.Errorf("InitBot: %w", err))
	}

	setupStates()
	setupHandlers(bot)

	return bot
}

func setupStates() {
	// STATE GROUP 1 - SURVEY
	fsm.AddState("StartSurvey",
		"Привет! Чтобы пользоваться ботом надо сначала пройти опрос из нескольких вопросов.\n\n(1/5) Назови, пожалуйста, своё имя?",
		func(c tele.Context) (nextState string, err error) {
			name := c.Message().Text
			if name == "" {
				return ResumeState, c.Send("Не могу распознать ответ. Попробуй еще раз =)")
			}
			if ok := matchingPatternName.MatchString(name); !ok {
				err := c.Send("Имя должно включать только русские или английские буквы и быть 2 - 50 символов")
				return ResumeState, err
			}
			err = SetStateVar(c, "Name", name)
			return "SetAge", err
		})

	fsm.AddState("SetAge",
		"(2/5) Отлично! Теперь скажи, сколько тебе лет?",
		func(c tele.Context) (nextState string, err error) {
			ageText := c.Message().Text
			if ageText == "" {
				return ResumeState, c.Send("Не могу распознать ответ. Попробуй еще раз =)")
			}

			age, err := strconv.Atoi(ageText)
			if err != nil {
				return ResumeState, c.Send("Возраст должен быть числом больше нуля, попробуй еще раз =)")
			}
			if age <= 0 {
				return ResumeState, c.Send("Возраст должен быть числом больше нуля, попробуй еще раз =)")
			}
			if age > 150 {
				return ResumeState, c.Send("Да ты совсем взрослый! Давай по-чесноку, сколько лет?")
			}
			err = SetStateVar(c, "Age", ageText)
			return "SetCity", err
		})

	fsm.AddState("SetCity",
		"(3/5) В каком городе живешь?",
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
			err = SetStateVar(c, "City", city)

			return "SetTimezone", err
		})

	fsm.AddState("SetTimezone",
		"(4/5) Сколько сейчас времени по твоим часам? Надо написать часы:минуты, например, 23:15. Это надо чтобы понять в каком часовом поясе ты находишься",
		func(c tele.Context) (nextState string, err error) {
			userTimeTxt := c.Message().Text
			if userTimeTxt == "" {
				return ResumeState, c.Send("Не могу распознать ответ. Попробуй еще раз =)")
			}

			if ok := matchingPatternTime.MatchString(userTimeTxt); !ok {
				return ResumeState, c.Send("Не могу распознать ответ. Надо написать в формате ЧЧ:ММ, например, 20:55")
			}
			userHoursMinutes := strings.Split(userTimeTxt, ":")
			if len(userHoursMinutes) != 2 {
				return ResumeState, c.Send("Не могу распознать ответ. Надо указать только часы и минуты формате ЧЧ:ММ, например, 20:55")
			}
			userHours, err := strconv.Atoi(userHoursMinutes[0])
			if err != nil {
				return ResumeState, c.Send("Не могу распознать ответ. Надо написать в формате ЧЧ:ММ, например, 20:55")
			}
			userMinutes, err := strconv.Atoi(userHoursMinutes[1])
			if err != nil {
				return ResumeState, c.Send("Не могу распознать ответ. Надо написать в формате ЧЧ:ММ, например, 20:55")
			}
			if userHours > 23 {
				return ResumeState, c.Send("Максимальный час - 23. Надо написать в формате ЧЧ:ММ, например, 20:55")
			}
			if userMinutes > 59 {
				return ResumeState, c.Send("Максимальная минута - 59. Надо написать в формате ЧЧ:ММ, например, 20:55")
			}

			userMinutes = userMinutes + userHours*60
			utcTime := time.Now().UTC()
			utcMinutes := utcTime.Minute() + utcTime.Hour()*60
			deltaMinutes := userMinutes - utcMinutes

			deltaDuration, _ := time.ParseDuration(fmt.Sprintf("%dm", deltaMinutes))
			deltaDuration = deltaDuration.Round(30 * time.Minute)

			deltaDurationMinutes := int(deltaDuration.Minutes())
			err = SetStateVar(c, "TimezoneRaw", strconv.Itoa(deltaDurationMinutes))

			var sign rune
			if deltaDurationMinutes < 0 {
				sign = '-'
			} else {
				sign = '+'
			}

			deltaHoursFmt := deltaDuration / time.Hour
			deltaDuration -= deltaHoursFmt * time.Hour
			deltaMinutesFmt := deltaDuration / time.Minute
			strUtcTimeZone := fmt.Sprintf("UTC%c%02d:%02d", sign, deltaHoursFmt.Abs(), deltaMinutesFmt.Abs())
			err = SetStateVar(c, "TimezoneTxt", strUtcTimeZone)
			if err != nil {
				fmt.Println(err.Error())
			}
			timeZoneMessage := fmt.Sprintf("Получается, твой часовой пояс - %s", strUtcTimeZone)
			c.Send(timeZoneMessage)

			return "SetExperience", err
		})

	possibleVariants := []string{"без опыта", "менее 1 года", "1-2 года", "2-3 года", "3-5 лет", "более 5 лет"}
	expSelector := ReplyMenuConstructor(possibleVariants, 2)
	fsm.AddState("SetExperience",
		"(5/5) Сколько занимаешься вокалом?",
		func(c tele.Context) (nextState string, err error) {
			expVariant := c.Text()
			if expVariant == "" {
				return ResumeState, c.Send("Не могу распознать ответ. Выбери вариант из списка")
			}
			expVariant = strings.ToLower(expVariant)
			if ok := slices.Contains(possibleVariants, expVariant); !ok {
				return ResumeState, c.Send("Не могу распознать ответ. Выбери вариант из списка")
			}

			values, err := GetStateVars(c)

			name, _ := values["Name"]
			ageStr, _ := values["Age"]
			age, _ := strconv.Atoi(ageStr)
			city, _ := values["City"]
			timezoneTxt, _ := values["TimezoneTxt"]
			timezoneRawStr, _ := values["TimezoneRaw"]
			timezoneRaw, _ := strconv.Atoi(timezoneRawStr)
			joinTime := time.Now().UTC()

			DB.Exec(context.Background(), `
				INSERT INTO users(user_id, username, age, city, timezone_raw, timezone_txt, experience, join_dt)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				`, c.Sender().ID, name, age, city, timezoneRaw, timezoneTxt, expVariant, joinTime)

			err = c.Send("Спасибо! Ты зарегистрирован в системе бота и теперь тебе доступна его функциональность!",
				&tele.ReplyMarkup{RemoveKeyboard: true})
			return ResetState, err
		}, expSelector)
}

func setupHandlers(bot *tele.Bot) {
	bot.Handle("/start", func(c tele.Context) error {
		userID := c.Sender().ID
		if UserIsInDatabase(userID) {
			return c.Reply("Welcome back!")
		}
		return fsm.TriggerState(c, "StartSurvey")
	})

	bot.Handle(tele.OnText, func(c tele.Context) error {
		return fsm.UpdateState(c)
	})
}
