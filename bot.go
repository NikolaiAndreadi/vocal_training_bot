package main

import (
	"context"
	"fmt"

	"vocal_training_bot/BotExt"

	tele "gopkg.in/telebot.v3"
)

var (
	MainUserMenuOptions = []string{
		"Распевки",
		"Напоминания",
		"Записаться на урок",
		"Обо мне",
		"Настройки аккаунта",
	}
	MainUserMenu = BotExt.ReplyMenuConstructor(MainUserMenuOptions, 2, false)
)

const (
	AccountSettingsMenu     = "AccountSettingsMenu"
	WarmupNotificationsMenu = "WarmupNotificationsMenu"
)

func InitBot(cfg Config) *tele.Bot {
	teleCfg := tele.Settings{
		Token: cfg.Bot.Token,
	}
	bot, err := tele.NewBot(teleCfg)
	if err != nil {
		panic(fmt.Errorf("InitBot: %w", err))
	}

	inlineMenus := BotExt.NewInlineMenus()
	fsm := BotExt.NewFiniteStateMachine(DB, inlineMenus)
	SetupInlineMenus(bot, fsm, inlineMenus)
	SetupStates(fsm)
	SetupHandlers(bot, fsm, inlineMenus)

	return bot
}

func SetupHandlers(bot *tele.Bot, fsm *BotExt.FSM, ims *BotExt.InlineMenusType) {
	bot.Handle("/start", func(c tele.Context) error {
		userID := c.Sender().ID

		if ok := UserIsInDatabase(userID); ok {
			return c.Reply("Привет! Ты зарегистрирован в боте, тебе доступна его функциональность!", MainUserMenu)
		}

		fsm.Trigger(c, SurveySGStartSurveyReqName)
		return nil
	})

	bot.Handle(tele.OnText, func(c tele.Context) error {
		userID := c.Sender().ID

		if ok := BotExt.HasState(userID); ok {
			fsm.Update(c)
			return nil
		}

		if ok := UserIsInDatabase(userID); !ok {
			return c.Send("Сначала надо пройти опрос.")
		}

		switch c.Text() {
		case "Распевки":
		case "Напоминания":
			return ims.Show(c, WarmupNotificationsMenu)
		case "Записаться на урок":
		case "Обо мне":
		case "Настройки аккаунта":
			return ims.Show(c, AccountSettingsMenu)
		}
		return nil
	})
}

func SetupInlineMenus(bot *tele.Bot, fsm *BotExt.FSM, ims *BotExt.InlineMenusType) {
	cancelButton := &BotExt.InlineButtonTemplate{
		Unique:         "Cancel",
		TextOnCreation: "Отмена",
		OnClick: func(c tele.Context) error {
			BotExt.ResetState(c)
			if err := c.Send("OK", MainUserMenu); err != nil {
				fmt.Println(err)
			}
			return c.Respond()
		},
	}

	im := BotExt.NewInlineMenu(
		AccountSettingsMenu,
		"Текущие настройки: нажми на пункт, чтобы изменить",
		1,
		func(c tele.Context) (map[string]string, error) {
			var name, age, city, tz, xp string
			err := DB.QueryRow(context.Background(),
				"SELECT username, text(age), city, timezone_txt, experience FROM users WHERE user_id = $1",
				c.Sender().ID).Scan(&name, &age, &city, &tz, &xp)
			if err != nil {
				return nil, err
			}
			data := map[string]string{
				"name":       name,
				"age":        age,
				"city":       city,
				"timezone":   tz,
				"experience": xp,
			}
			return data, nil
		})
	im.AddButtons([]*BotExt.InlineButtonTemplate{
		{
			Unique: "ChangeName",
			TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["name"]
				if !ok {
					return "Имя неизвестно", fmt.Errorf("can't fetch name")
				}
				return "Имя: " + s, nil
			},
			OnClick: func(c tele.Context) error {
				fsm.Trigger(c, SettingsSGSetName, AccountSettingsMenu)
				return c.Respond()
			},
		},
		{
			Unique: "ChangeAge",
			TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["age"]
				if !ok {
					return "Возраст неизвестен", fmt.Errorf("can't fetch age")
				}
				return "Возраст: " + s, nil
			},
			OnClick: func(c tele.Context) error {
				fsm.Trigger(c, SettingsSGSetAge, AccountSettingsMenu)
				return c.Respond()
			},
		},
		{
			Unique: "ChangeCity",
			TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["city"]
				if !ok {
					return "Город неизвестен", fmt.Errorf("can't fetch city")
				}
				return "Город: " + s, nil
			},
			OnClick: func(c tele.Context) error {
				fsm.Trigger(c, SettingsSGSetCity, AccountSettingsMenu)
				return c.Respond()
			},
		},
		{
			Unique: "ChangeTimezone",
			TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["timezone"]
				if !ok {
					return "Часовой пояс неизвестен", fmt.Errorf("can't fetch timezone")
				}
				return "Часовой пояс: " + s, nil
			},
			OnClick: func(c tele.Context) error {
				fsm.Trigger(c, SettingsSGSetTimezone, AccountSettingsMenu)
				return c.Respond()
			},
		},
		{
			Unique: "ChangeExperience",
			TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["experience"]
				if !ok {
					return "Опыт вокала неизвестен", fmt.Errorf("can't fetch experience")
				}
				return "Опыт вокала: " + s, nil
			},
			OnClick: func(c tele.Context) error {
				fsm.Trigger(c, SettingsSGSetExperience, AccountSettingsMenu)
				return c.Respond()
			},
		},
		cancelButton,
	})
	err := ims.RegisterMenu(bot, im)
	if err != nil {
		panic(err)
	}

	im = BotExt.NewInlineMenu(
		WarmupNotificationsMenu,
		"Настройки напоминаний о распевках:",
		2,
		WarmupNotificationsMenuDataFetcher,
	)
	mon := NotificationButtonFabric(fsm, ims, "mon", "Понедельник")
	tue := NotificationButtonFabric(fsm, ims, "tue", "Вторник")
	wed := NotificationButtonFabric(fsm, ims, "wed", "Среда")
	thu := NotificationButtonFabric(fsm, ims, "thu", "Четверг")
	fri := NotificationButtonFabric(fsm, ims, "fri", "Пятница")
	sat := NotificationButtonFabric(fsm, ims, "sat", "Суббота")
	sun := NotificationButtonFabric(fsm, ims, "sun", "Воскресенье")
	im.AddButtons([]*BotExt.InlineButtonTemplate{
		mon[0], mon[1],
		tue[0], tue[1],
		wed[0], wed[1],
		thu[0], thu[1],
		fri[0], fri[1],
		sat[0], sat[1],
		sun[0], sun[1],
		{Unique: BotExt.RowSplitterButton},
		{
			Unique: "GlobalSwitch",
			TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
				s := "Глобальный выключатель: "
				v, ok := dc["globalOn"]
				if !ok {
					return s + "???", fmt.Errorf("can't fetch globalOn")
				}
				if v == "true" {
					return s + "🔔", nil
				}
				return s + "🔕", nil
			},
			OnClick: func(c tele.Context) error {
				var res bool
				err := DB.QueryRow(context.Background(), `
					UPDATE warmup_notification_global
					SET global_switch = NOT global_switch
					WHERE user_id = $1
					RETURNING global_switch`, c.Sender().ID).Scan(&res)
				if err != nil {
					fmt.Println(fmt.Errorf("can't switch global notifications: %w", err))
				}
				ims.Update(c, WarmupNotificationsMenu)
				if res {
					err = notificationService.AddUser(c.Sender().ID)
				} else {
					err = notificationService.DelUser(c.Sender().ID)
				}
				if err != nil {
					fmt.Println(fmt.Errorf("can't switch global notifications: %w", err))
				}
				return c.Respond()
			}},
		cancelButton,
	})
	err = ims.RegisterMenu(bot, im)
	if err != nil {
		panic(err)
	}
}

func WarmupNotificationsMenuDataFetcher(c tele.Context) (map[string]string, error) {
	rows, err := DB.Query(context.Background(), `
				SELECT 
				    day_of_week,
    				cast(trigger_switch AS varchar(5)), 
       				to_char(trigger_time,'HH24:MI')
				FROM warmup_notifications WHERE user_id = $1`, c.Sender().ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	data := make(map[string]string)
	var dayName, daySwitch, notificationTime string
	for rows.Next() {
		err = rows.Scan(&dayName, &daySwitch, &notificationTime)
		if err != nil {
			return data, err
		}
		data[dayName+"On"] = daySwitch
		data[dayName+"Time"] = notificationTime
	}
	if err := rows.Err(); err != nil {
		return data, fmt.Errorf("WarmupNotificationsMenuDataFetcher: postgres itetator %w", err)
	}

	var globalSwitch string
	err = DB.QueryRow(context.Background(),
		`SELECT cast(global_switch AS varchar(5)) FROM warmup_notification_global WHERE user_id = $1`,
		c.Sender().ID).Scan(&globalSwitch)
	if err != nil {
		return data, err
	}
	data["globalOn"] = globalSwitch

	return data, nil
}

func NotificationButtonFabric(fsm *BotExt.FSM, ims *BotExt.InlineMenusType, dayUnique string, dayText string) (ibt [2]*BotExt.InlineButtonTemplate) {
	// switch
	ibt[0] = &BotExt.InlineButtonTemplate{
		Unique: "NotificationSwitch_" + dayUnique,
		TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
			s := dayText + ": "
			v, ok := dc[dayUnique+"On"]
			if !ok {
				return s + "???", fmt.Errorf("can't fetch %sOn", dayUnique)
			}
			if v == "true" {
				return s + "🔔", nil
			}
			return s + "🔕", nil
		},
		OnClick: func(c tele.Context) error {
			userID := c.Sender().ID
			_, err := DB.Exec(context.Background(), `
			UPDATE warmup_notifications
			SET trigger_switch = NOT trigger_switch
			WHERE (user_id = $1) AND (day_of_week = $2)`, userID, dayUnique)
			if err != nil {
				fmt.Println(fmt.Errorf("can't switch notifications for day %s: %w", dayUnique, err))
			}
			ims.Update(c, WarmupNotificationsMenu)

			ts, err := getNearestNotificationFromPg(userID)
			if err != nil {
				fmt.Println(fmt.Errorf("switch notifications for day %s: getNearestNotificationFromPg: %w", dayUnique, err))
			}
			if err = notificationService.DelUser(userID); err != nil {
				fmt.Println(fmt.Errorf("switch notifications for day %s: %w", dayUnique, err))
			}
			if err = notificationService.addUser(userID, ts); err != nil {
				fmt.Println(fmt.Errorf("switch notifications for day %s: %w", dayUnique, err))
			}
			return c.Respond()
		},
	}
	// set time
	ibt[1] = &BotExt.InlineButtonTemplate{
		Unique: "NotificationTime_" + dayUnique,
		TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
			time, ok := dc[dayUnique+"Time"]
			if !ok {
				return "HH:MM", fmt.Errorf("can't fetch %sTime", dayUnique)
			}
			return time, nil
		},
		OnClick: func(c tele.Context) error {
			BotExt.SetStateVar(c, "day", dayUnique)
			fsm.Trigger(c, NotificationSGSetTime, WarmupNotificationsMenu)
			return c.Respond()
		},
	}
	return
}
