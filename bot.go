package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
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
	MainUserMenu = ReplyMenuConstructor(MainUserMenuOptions, 2, false)

	AccountSettingsMenu *InlineMenu
	//WarmupNotificationsMenu *InlineMenu
)

func InitBot(cfg Config) *tele.Bot {
	teleCfg := tele.Settings{
		Token: cfg.Bot.Token,
	}
	bot, err := tele.NewBot(teleCfg)
	if err != nil {
		panic(fmt.Errorf("InitBot: %w", err))
	}

	fsm := SetupStates(DB)
	setupInlineMenus(bot, DB, fsm)
	setupHandlers(bot, fsm)

	return bot
}

func setupInlineMenus(bot *tele.Bot, db *pgxpool.Pool, fsm *FSM) {
	AccountSettingsMenu = NewInlineMenu("Текущие настройки: нажми на пункт, чтобы изменить",
		func(c tele.Context) (map[string]string, error) {
			var name, age, city, tz, xp string
			err := db.QueryRow(context.Background(),
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

	AccountSettingsMenu.AddButtons([]*InlineButtonTemplate{
		{
			"ChangeName",
			func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["name"]
				if !ok {
					return "Имя неизвестно", fmt.Errorf("can't fetch name")
				}
				return "Имя: " + s, nil
			},
			SettingsSGSetName,
		},
		{
			"ChangeAge",
			func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["age"]
				if !ok {
					return "Возраст неизвестен", fmt.Errorf("can't fetch age")
				}
				return "Возраст: " + s, nil
			},
			SettingsSGSetAge,
		},
		{
			"ChangeCity",
			func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["city"]
				if !ok {
					return "Город неизвестен", fmt.Errorf("can't fetch city")
				}
				return "Город: " + s, nil
			},
			SettingsSGSetCity,
		},
		{
			"ChangeTimezone",
			func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["timezone"]
				if !ok {
					return "Часовой пояс неизвестен", fmt.Errorf("can't fetch timezone")
				}
				return "Часовой пояс: " + s, nil
			},
			SettingsSGSetTimezone,
		},
		{
			"ChangeExperience",
			func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["experience"]
				if !ok {
					return "Опыт вокала неизвестен", fmt.Errorf("can't fetch experience")
				}
				return "Опыт вокала: " + s, nil
			},
			SettingsSGSetExperience,
		},
		{
			"Cancel",
			"Отмена",
			func(c tele.Context) error {
				if err := fsm.ResetState(c); err != nil {
					fmt.Println(err)
				}
				if err := c.Send("OK", MainUserMenu); err != nil {
					fmt.Println(err)
				}
				return c.Respond()
			},
		},
	})
	AccountSettingsMenu.Construct(bot, fsm, 1)

	/*
		AccountSettingsMenu = InlineMenuConstructor(bot, fsm, 1, AccountSettingsButtons)

		WarmupNotificationsButtons = NewInlineMenuButtonBlock([]*InlineMenuButton{
			{
				"NotificationSwitchMon",
				func(c tele.Context) (s string, err error) {
					var v bool
					err = db.QueryRow(context.Background(),
						"SELECT mon_on FROM warmup_notifications WHERE user_id = $1", c.Sender().ID).Scan(&v)
					s = "Понедельник: "
					if v == true {
						return s + "🔔", err
					}
					return s + "🔕", err
				},
				NoState,
			},
			{
				"NotificationTimeMon",
				func(c tele.Context) (s string, err error) {
					err = db.QueryRow(context.Background(),
						"SELECT to_char(mon_time,'HH24:MI') FROM warmup_notifications WHERE user_id = $1", c.Sender().ID).Scan(&s)
					return s, err
				},
				NoState,
			},
		})
		WarmupNotificationsMenu = InlineMenuConstructor(bot, fsm, 2, WarmupNotificationsButtons)
	*/
}

func setupHandlers(bot *tele.Bot, fsm *FSM) {
	bot.Handle("/start", func(c tele.Context) error {
		userID := c.Sender().ID

		ok, err := UserIsInDatabase(userID)
		if err != nil {
			return fmt.Errorf("/start[%d]: %w", userID, err)
		}

		if ok {
			return c.Reply("Привет! Ты зарегистрирован в боте, тебе доступна его функциональность!", MainUserMenu)
		}

		return fsm.TriggerState(c, SurveySGStartSurveyReqName)
	})

	bot.Handle(tele.OnText, func(c tele.Context) error {
		userID := c.Sender().ID

		ok, err := UserIsInDatabase(userID)
		if err != nil {
			return fmt.Errorf("/OnText: %w", err)
		}
		if !ok {
			return c.Send("Сначала надо пройти опрос.")
		}

		ok, err = UserHasState(userID)
		if err != nil {
			return fmt.Errorf("/OnText: %w", err)
		}
		if ok {
			return fsm.UpdateState(c)
		}

		switch c.Text() {
		case "Распевки":
		case "Напоминания":
			//return c.Send("Напоминания:",
			//	FillInlineMenu(c, WarmupNotificationsMenu, WarmupNotificationsButtons))
		case "Записаться на урок":
		case "Обо мне":
		case "Настройки аккаунта":
			return AccountSettingsMenu.Serve(c)
			//return c.Send("Текущие настройки: нажми на пункт, чтобы изменить",
			//	FillInlineMenu(c, AccountSettingsMenu, AccountSettingsButtons))
		}
		return nil
	})
}
