package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"vocal_training_bot/BotExt"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	tele "gopkg.in/telebot.v3"
)

const (
	SurveySGStartSurveyReqName = "SurveyStateGroup_StartSurveyReqName"
	// surveySGSetAge             = "SurveyStateGroup_SetAge"
	surveySGSetCity     = "SurveyStateGroup_SetCity"
	surveySGSetTimezone = "SurveyStateGroup_SetTimezone"
	// surveySGSetExperience      = "SurveyStateGroup_SetExperience"

	surveySGVarName = "Name"
	// surveySGVarAge         = "Age"
	surveySGVarCity = "City"
	// surveySGVarTimezoneRaw = "TimezoneRaw"
	// surveySGVarTimezoneStr = "TimezoneTxt"

	SettingsSGSetName = "SettingsStateGroup_SetName"
	//SettingsSGSetAge        = "SettingsStateGroup_SetAge"
	SettingsSGSetCity     = "SettingsStateGroup_SetCity"
	SettingsSGSetTimezone = "SettingsStateGroup_SetTimezone"
	// SettingsSGSetExperience = "SettingsStateGroup_SetExperience"

	NotificationSGSetTime = "NotificationStateGroup_SetTime"

	WannabeStudentSGSendReq = "WannabeStudentSG_SendReq"
)

func SetupUserStates(fsm *BotExt.FSM) {
	err := fsm.RegisterStateChain([]*BotExt.State{
		{
			Name: SurveySGStartSurveyReqName,
			OnTrigger: `Привет 🤍 рад наконец-то видеть тебя здесь! Я - вокальный бот, буду помогать и
поддерживать тебя на твоём вокальном пути!

Перед началом надо ответь на несколько моих вопросов…

(1/3) Напиши своё имя и фамилию 👩‍🎤
`,
			Validator:   nameValidator,
			Manipulator: nameSaver,
		},
		/*{
			Name:        surveySGSetAge,
			OnTrigger:   "(2/5) Теперь скажи, сколько тебе лет?",
			Validator:   ageValidator,
			Manipulator: ageSaver,
		},*/
		{
			Name:        surveySGSetCity,
			OnTrigger:   "(2/3) Приятно познакомиться 🤓 Из какого ты города?",
			Validator:   cityValidator,
			Manipulator: citySaver,
		},
		{
			Name:        surveySGSetTimezone,
			OnTrigger:   "(3/3) Сколько сейчас времени по твоим часам? Надо написать часы:минуты, например, 23:15. Это надо чтобы понять в каком часовом поясе ты находишься.",
			Validator:   timeValidator,
			Manipulator: timezoneSaver,
			OnSuccess: `Спасибо! Ты зарегистрирован в системе бота и теперь тебе доступна его функциональность!
В главном меню ты найдёшь упражнения, распевки, напоминания и полезные материалы 🤍
⚠️ Если главное меню не открывается, нажми на иконку 🎛 в правом нижнем углу`,
			OnQuitExtra: []interface{}{MainUserMenu},
		},
		/*{
			Name:           surveySGSetExperience,
			OnTrigger:      "(5/5) Сколько занимаешься вокалом?",
			Validator:      experienceValidator,
			Manipulator:    saveSurveyRegisterUser,
			OnTriggerExtra: []interface{}{experienceReplyMenu},
			OnSuccess:      "Спасибо! Ты зарегистрирован в системе бота и теперь тебе доступна его функциональность!",
			OnQuitExtra:    []interface{}{MainUserMenu},
		},*/
	})
	if err != nil {
		panic(err)
	}

	err = fsm.RegisterOneShotState(&BotExt.State{
		Name:      SettingsSGSetName,
		OnTrigger: "Введи новое имя",
		Validator: nameValidator,
		Manipulator: func(c tele.Context) (err error) {
			_, err = DB.Exec(context.Background(), `
				UPDATE users
				SET username = $1
				WHERE user_id = $2
				`, c.Text(), c.Sender().ID)
			return
		},
		OnSuccess: "Имя изменено",
	})
	if err != nil {
		panic(err)
	}

	/* err = fsm.RegisterOneShotState(&BotExt.State{
		Name:      SettingsSGSetAge,
		OnTrigger: "Введи новый возраст",
		Validator: ageValidator,
		Manipulator: func(c tele.Context) (err error) {
			_, err = DB.Exec(context.Background(), `
				UPDATE users
				SET age = $1
				WHERE user_id = $2
				`, c.Text(), c.Sender().ID)
			return
		},
		OnSuccess: "Возраст обновлен",
	})
	if err != nil {
		panic(err)
	}
	*/
	err = fsm.RegisterOneShotState(&BotExt.State{
		Name:      SettingsSGSetCity,
		OnTrigger: "Введи новый город",
		Validator: cityValidator,
		Manipulator: func(c tele.Context) (err error) {
			_, err = DB.Exec(context.Background(), `
				UPDATE users
				SET city = $1
				WHERE user_id = $2
				`, c.Text(), c.Sender().ID)
			return
		},
		OnSuccess: "Город обновлен",
	})
	if err != nil {
		panic(err)
	}

	err = fsm.RegisterOneShotState(&BotExt.State{
		Name:      SettingsSGSetTimezone,
		OnTrigger: "Введи свое время (в формате ЧЧ:ММ, например 12:15 или 9:15)",
		Validator: timeValidator,
		Manipulator: func(c tele.Context) (err error) {
			userHoursMinutes := strings.Split(c.Text(), ":")
			userHours, _ := strconv.Atoi(userHoursMinutes[0])
			userMinutes, _ := strconv.Atoi(userHoursMinutes[1])
			utcTimezone, utcMinutesShift, err := calcTimezoneByTimeShift(userHours, userMinutes)
			_, err = DB.Exec(context.Background(), `
				UPDATE users
				SET timezone_txt = $1, timezone_raw = $2
				WHERE user_id = $3
				`, utcTimezone, utcMinutesShift, c.Sender().ID)
			if err != nil {
				return
			}
			return c.Send(fmt.Sprintf("Получается, твой часовой пояс - %s", utcTimezone))
		},
	})
	if err != nil {
		panic(err)
	}

	/*
		err = fsm.RegisterOneShotState(&BotExt.State{
			Name:           SettingsSGSetExperience,
			OnTrigger:      "Сколько уже занимаешься вокалом?",
			OnTriggerExtra: []interface{}{experienceReplyMenu},
			Validator:      experienceValidator,
			Manipulator: func(c tele.Context) (err error) {
				_, err = DB.Exec(context.Background(), `
					UPDATE users
					SET experience = $1
					WHERE user_id = $2
					`, c.Text(), c.Sender().ID)
				return
			},
			OnSuccess:   "Опыт вокала изменен",
			OnQuitExtra: []interface{}{MainUserMenu},
		})
		if err != nil {
			panic(err)
		}
	*/

	err = fsm.RegisterOneShotState(&BotExt.State{
		Name:      NotificationSGSetTime,
		OnTrigger: "Введи время, в которое ты хочешь получать напоминание о занятиях. Напиши в формате чч:мм, например, 14:00",
		Validator: timeValidator,
		Manipulator: func(c tele.Context) error {
			userID := c.Sender().ID

			day, ok := BotExt.GetStateVar(userID, "day")
			if !ok {
				return fmt.Errorf("can't fetch variable 'day' from states table")
			}

			_, err := DB.Exec(context.Background(), `
			UPDATE warmup_notifications
			SET trigger_time = $1
			WHERE (user_id = $2) AND (day_of_week = $3)`, c.Message().Text, userID, day)
			if err != nil {
				return err
			}

			ts, err := getNearestNotificationFromPg(userID)
			if err != nil {
				return fmt.Errorf("state NotificationSGSetTime: getNearestNotificationFromPg: %w", err)
			}
			if err = notificationService.DelUser(userID); err != nil {
				return fmt.Errorf("state NotificationSGSetTime: DelUser: %w", err)
			}
			if err = notificationService.addUser(userID, ts); err != nil {
				return fmt.Errorf("state NotificationSGSetTime: addUser: %w", err)
			}

			return err
		},
		OnSuccess: "Отлично! Буду на связи в это время 🤓",
	})
	if err != nil {
		panic(err)
	}

	err = fsm.RegisterOneShotState(&BotExt.State{
		Name: WannabeStudentSGSendReq,
		OnTrigger: `Я преподаю вокал в Москве и онлайн в любой точке мира

В первой части урока мы уделяем время тренировке голосовых мышц, координации голоса, теории, вопросам, изучению новых приемов и возможностей нашего голоса 🤓
Во второй части урока мы поем, кайфуем, разбираем песни, импровизируем и творим музыку здесь и сейчас ✨🎶🤍

Одеваемся на занятия удобно, так как мы много работаем с телом + берём с собой
бутылочку воды, готовим несколько песен, тексты и, конечно, open mind 🪐🤍

🏢 Занятие в Москве 🏢
Адрес для занятий в Москве: Красный Октябрь, Берсеневская набережная 6 с2, we play music rooms
Урок длится 60 мин

💻 Занятие онлайн 💻
Урок длится 90 мин. В онлайне работаем дольше, чем на студии из-за особенностей формата и взаимодействия + закладываем время на косяки связи. Созваниваемся по фейстайм/скайп

🍨 Цены 🍨
2000р - стартовое занятие
3000р - разовое занятие
10000р - абонемент на 4 занятия
*цены на онлайн и оффлайн занятия одинаковы

Чтобы записаться на урок, напишите мне в личные сообщения в телеграме! @vershkovaaa

🎁 Сертификаты 🎁

Декабрь - время милых подарков для своих близких! Если вы хотите их порадовать и дать волшебный пинок для развития своего голоса, проявленности и открытости, вы можете подарить им занятия вокалом со мной 🤍
Также вы можете попросить их положить вам под ёлку сертификат на одно или несколько занятий к новому году 🎅❤️
Сертификат работает для всех форматов обучения: онлайн и оффлайн. Действует в течение двух месяцев.
`,
		// OnTriggerExtra: []interface{}{wannabeStudentMenu},
		//Validator:   wannabeStudentValidator,
		//Manipulator: wannabeStudentManipulator,
		//OnSuccess:   "Готово!",
		//OnQuitExtra: []interface{}{MainUserMenu},
	})
}

var (
	matchingPatternName = regexp.MustCompile("[ЁёА-яA-Za-z ]{2,50}")
	matchingPatternCity = regexp.MustCompile("[ЁёА-яA-Za-z ]{2,50}")
	matchingPatternTime = regexp.MustCompile("[0-9]?[0-9]:[0-9][0-9]")
)

func nameValidator(c tele.Context) string {
	name := strings.TrimSpace(c.Text())
	if ok := matchingPatternName.MatchString(name); !ok {
		return "Имя должно включать только русские или английские буквы и быть 2 - 50 символов"
	}
	return ""
}

/*
func ageValidator(c tele.Context) string {
	ageText := c.Text()
	age, err := strconv.Atoi(ageText)
	if err != nil {
		return "Возраст должен быть числом больше нуля, попробуй еще раз =)"
	}
	if age <= 0 {
		return "Возраст должен быть числом больше нуля, попробуй еще раз =)"
	}
	if age > 100 {
		return "Да ты совсем взрослый! Давай по-чесноку, сколько лет?"
	}
	return ""
}
*/

func cityValidator(c tele.Context) string {
	city := strings.TrimSpace(c.Text())
	if ok := matchingPatternCity.MatchString(city); !ok {
		return "Не могу распознать ответ. Попробуй еще раз!"
	}
	return ""
}

func timeValidator(c tele.Context) string {
	userTimeTxt := c.Text()
	errStr := "Не могу распознать ответ. Надо написать в формате ЧЧ:ММ, например, 20:55"
	if ok := matchingPatternTime.MatchString(userTimeTxt); !ok {
		return errStr
	}
	userHoursMinutes := strings.Split(userTimeTxt, ":")
	if len(userHoursMinutes) != 2 {
		return errStr
	}

	// str -> int
	userHours, err := strconv.Atoi(userHoursMinutes[0])
	if err != nil {
		return errStr
	}
	userMinutes, err := strconv.Atoi(userHoursMinutes[1])
	if err != nil {
		return errStr
	}

	// post validation
	if userHours > 23 {
		errStr = "Максимальный час - 23. Надо написать в формате ЧЧ:ММ, например, 20:55"
		return errStr
	}
	if userMinutes > 59 {
		errStr = "Максимальная минута - 59. Надо написать в формате ЧЧ:ММ, например, 20:55"
		return errStr
	}
	return ""
}

/*
func experienceValidator(c tele.Context) string {
	xpVariant := strings.ToLower(strings.TrimSpace(c.Text()))
	if ok := slices.Contains(experienceAllowedAnswers, xpVariant); !ok {
		return "Не могу распознать ответ. Выбери вариант из списка"
	}
	return ""
}
*/

func nameSaver(c tele.Context) error {
	name := strings.TrimSpace(c.Text())
	name = cases.Title(language.Tag{}).String(name)
	BotExt.SetStateVar(c.Sender().ID, surveySGVarName, name)
	return nil
}

/*
func ageSaver(c tele.Context) error {
	age := c.Text()
	BotExt.SetStateVar(c.Sender().ID, surveySGVarAge, age)
	return nil
}
*/

func citySaver(c tele.Context) error {
	city := strings.TrimSpace(c.Text())
	city = cases.Title(language.Tag{}).String(city)
	BotExt.SetStateVar(c.Sender().ID, surveySGVarCity, city)
	return nil
}

func timezoneSaver(c tele.Context) error {
	userHoursMinutes := strings.Split(c.Text(), ":")
	userHours, _ := strconv.Atoi(userHoursMinutes[0])
	userMinutes, _ := strconv.Atoi(userHoursMinutes[1])
	utcTimezone, utcMinutesShift, err := calcTimezoneByTimeShift(userHours, userMinutes)
	if err != nil {
		return err
	}
	_ = c.Send(fmt.Sprintf("Получается, твой часовой пояс - %s", utcTimezone))

	userID := c.Sender().ID

	values := BotExt.GetStateVars(userID)
	name, _ := values[surveySGVarName]
	city, _ := values[surveySGVarCity]

	joinTime := time.Now().UTC()

	_, err = DB.Exec(context.Background(), `
				INSERT INTO users(user_id, username, city, timezone_raw, timezone_txt, join_dt)
				VALUES ($1, $2, $3, $4, $5, $6)
				`, userID, name, city, utcMinutesShift, utcTimezone, joinTime)
	if err != nil {
		return err
	}

	err = initUserDBs(c.Sender().ID)
	return err
}

func calcTimezoneByTimeShift(userHours, userMinutes int) (utcTimezone string, utcMinutesShift string, err error) {
	userMinutes = userMinutes + userHours*60

	utcTime := time.Now().UTC()
	utcMinutes := utcTime.Minute() + utcTime.Hour()*60

	deltaMinutes := userMinutes - utcMinutes
	// corner cases - on the day edge
	// e.g. UTC 22:30 27 jan; UTC+3 01:30 28 jan; delta 180, not -1260
	// e.g. UTC 01:00 27 jan; UTC-2 23:00 26 jan; delta -120, not 1320
	if deltaMinutes < -720 {
		deltaMinutes = 1440 + deltaMinutes
	}
	if deltaMinutes > 840 {
		deltaMinutes = deltaMinutes - 1440
	}

	deltaMinutesDur, err := time.ParseDuration(fmt.Sprintf("%dm", deltaMinutes))
	if err != nil {
		err = fmt.Errorf("calcTimezoneByTimeshift(%d, %d): %w", userHours, userMinutes, err)
		return
	}
	deltaMinutesDur = deltaMinutesDur.Round(30 * time.Minute)
	utcMinutesShift = strconv.Itoa(int(deltaMinutesDur.Minutes())) // save output

	// utcTimezone representation
	var offsetSign rune
	if deltaMinutesDur.Minutes() < 0 {
		offsetSign = '-'
	} else {
		offsetSign = '+'
	}

	deltaHoursFmt := deltaMinutesDur / time.Hour
	deltaMinutesDur -= deltaHoursFmt * time.Hour
	deltaMinutesFmt := deltaMinutesDur / time.Minute
	utcTimezone = fmt.Sprintf("UTC%c%02d:%02d", offsetSign, deltaHoursFmt.Abs(), deltaMinutesFmt.Abs())

	return
}

/*
func saveSurveyRegisterUser(c tele.Context) error {
	userID := c.Sender().ID

	xp := strings.ToLower(strings.TrimSpace(c.Text()))
	values := BotExt.GetStateVars(userID)
	name, _ := values[surveySGVarName]
	ageTxt, _ := values[surveySGVarAge]
	age, _ := strconv.Atoi(ageTxt)
	city, _ := values[surveySGVarCity]
	tzStr, _ := values[surveySGVarTimezoneStr]
	tzRawTxt, _ := values[surveySGVarTimezoneRaw]
	tzRaw, _ := strconv.Atoi(tzRawTxt)

	joinTime := time.Now().UTC()

	_, err := DB.Exec(context.Background(), `
				INSERT INTO users(user_id, username, age, city, timezone_raw, timezone_txt, experience, join_dt)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				`, userID, name, age, city, tzRaw, tzStr, xp, joinTime)
	if err != nil {
		return err
	}

	err = initUserDBs(c.Sender().ID)
	return err
}
*/

/*
func wannabeStudentValidator(c tele.Context) string {
	t := c.Text()
	if t == "" && c.Message().ReplyTo != nil {
		if c.Message().ReplyTo.Sender.ID != c.Bot().Me.ID {
			return "Нажми на кнопку 'Позвонить' чтобы поделиться своим контактом"
		}
		if c.Message().Contact != nil {
			return "" // handle contact
		}
	}
	if t == "Отмена" || t == "Написать в личку в телеграм" {
		return ""
	}
	return "Не могу распознать ответ... Выбери вариант из списка"
}

func wannabeStudentManipulator(c tele.Context) error {
	if c.Text() == "Отмена" {
		return nil
	}
	userID := c.Sender().ID
	phone := ""
	userName := c.Sender().Username

	if c.Text() != "Написать в личку в телеграм" {
		phone = c.Message().Contact.PhoneNumber
	}

	var resolved bool
	err := DB.QueryRow(context.Background(), `
	SELECT resolved FROM wannabe_student
	WHERE user_id = $1`, userID).Scan(&resolved)
	if err != pgx.ErrNoRows {
		if resolved == true {
			_ = c.Send("Вы уже подавали заявку и ее рассмотрели... Напиши, пожалуйста, напрямую @vershkovaaa для записи на занятие")
			return nil
		}
		if resolved == false {
			_ = c.Send("Заявка уже подана, ее рассмотрят в ближайшее время!")
			return nil
		}
		return err
	}

	_, err = DB.Exec(context.Background(), `
	INSERT INTO wannabe_student(user_id, user_name, phone_num)
	VALUES ($1, $2, $3)
	`, userID, userName, phone)
	return err
}
*/
