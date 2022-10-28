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
			func(c tele.Context) (s string) {
				userID := c.Sender().ID
				err := db.QueryRow(context.Background(), "SELECT username FROM users WHERE user_id = $1", userID).Scan(&s)
				if err != nil {
					fmt.Println(err)
				}
				return "Имя: " + s
			},
			func(c tele.Context) error {
				err := fsm.TriggerState(c, SettingsSGSetName)
				if err != nil {
					fmt.Println(err)
				}
				return c.Respond()
			},
		},
		{
			"ChangeAge",
			func(c tele.Context) (s string) {
				userID := c.Sender().ID
				err := db.QueryRow(context.Background(), "SELECT text(age) FROM users WHERE user_id = $1", userID).Scan(&s)
				if err != nil {
					fmt.Println(err)
				}
				return "Возраст: " + s
			},
			func(c tele.Context) error {
				err := fsm.TriggerState(c, SettingsSGSetAge)
				if err != nil {
					fmt.Println(err)
				}
				return c.Respond()
			},
		},
		{
			"ChangeCity",
			func(c tele.Context) (s string) {
				userID := c.Sender().ID
				err := db.QueryRow(context.Background(), "SELECT city FROM users WHERE user_id = $1", userID).Scan(&s)
				if err != nil {
					fmt.Println(err)
				}
				return "Город: " + s
			},
			func(c tele.Context) error {
				err := fsm.TriggerState(c, SettingsSGSetCity)
				if err != nil {
					fmt.Println(err)
				}
				return c.Respond()
			},
		},
		{
			"ChangeTimezone",
			func(c tele.Context) (s string) {
				userID := c.Sender().ID
				err := db.QueryRow(context.Background(), "SELECT timezone_txt FROM users WHERE user_id = $1", userID).Scan(&s)
				if err != nil {
					fmt.Println(err)
				}
				return "Часовой пояс: " + s
			},
			func(c tele.Context) error {
				err := fsm.TriggerState(c, SettingsSGSetTimezone)
				if err != nil {
					fmt.Println(err)
				}
				return c.Respond()
			},
		},
		{
			"ChangeExperience",
			func(c tele.Context) (s string) {
				userID := c.Sender().ID
				err := db.QueryRow(context.Background(), "SELECT experience FROM users WHERE user_id = $1", userID).Scan(&s)
				if err != nil {
					fmt.Println(err)
				}
				return "Опыт вокала: " + s
			},
			func(c tele.Context) error {
				err := fsm.TriggerState(c, SettingsSGSetExperience)
				if err != nil {
					fmt.Println(err)
				}
				return c.Respond()
			},
		},
		{
			"Cancel",
			func(c tele.Context) (s string) {
				return "Отмена"
			},
			func(c tele.Context) error {
				err := fsm.TriggerState(c, ResumeState)
				if err != nil {
					fmt.Println(err)
				}
				err = c.Send("OK", MainUserMenu)
				if err != nil {
					fmt.Println(err)
				}
				return c.Respond()
			},
		},
	}

	AccountSettingsMenu = InlineMenuConstructor(bot, 1, AccountSettingsButtons)
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
