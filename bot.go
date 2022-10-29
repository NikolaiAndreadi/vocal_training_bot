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

	// AccountSettingsMenu inline group
	AccountSettingsMenu    *tele.ReplyMarkup
	AccountSettingsButtons []*InlineMenuButton
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
	AccountSettingsButtons = []*InlineMenuButton{
		{
			"ChangeName",
			func(c tele.Context) (s string, err error) {
				err = db.QueryRow(context.Background(),
					"SELECT username FROM users WHERE user_id = $1", c.Sender().ID).Scan(&s)
				return "Имя: " + s, err
			},
			SettingsSGSetName,
		},
		{
			"ChangeAge",
			func(c tele.Context) (s string, err error) {
				err = db.QueryRow(context.Background(),
					"SELECT text(age) FROM users WHERE user_id = $1", c.Sender().ID).Scan(&s)
				return "Возраст: " + s, err
			},
			SettingsSGSetAge,
		},
		{
			"ChangeCity",
			func(c tele.Context) (s string, err error) {
				err = db.QueryRow(context.Background(),
					"SELECT city FROM users WHERE user_id = $1", c.Sender().ID).Scan(&s)
				return "Город: " + s, err
			},
			SettingsSGSetCity,
		},
		{
			"ChangeTimezone",
			func(c tele.Context) (s string, err error) {
				err = db.QueryRow(context.Background(),
					"SELECT timezone_txt FROM users WHERE user_id = $1", c.Sender().ID).Scan(&s)
				return "Часовой пояс: " + s, err
			},
			SettingsSGSetTimezone,
		},
		{
			"ChangeExperience",
			func(c tele.Context) (s string, err error) {
				err = db.QueryRow(context.Background(),
					"SELECT experience FROM users WHERE user_id = $1", c.Sender().ID).Scan(&s)
				return "Опыт вокала: " + s, err
			},
			SettingsSGSetExperience,
		},
		{
			"Cancel",
			func(c tele.Context) (string, error) {
				return "Отмена", nil
			},
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
	}

	AccountSettingsMenu = InlineMenuConstructor(bot, fsm, 1, AccountSettingsButtons)
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
		case "Записаться на урок":
		case "Обо мне":
		case "Настройки аккаунта":
			return c.Send("Текущие настройки: нажми на пункт, чтобы изменить",
				FillInlineMenu(c, AccountSettingsMenu, AccountSettingsButtons))
		}
		return nil
	})
}
