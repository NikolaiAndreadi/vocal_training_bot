package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/exp/slices"
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
	setupInlineMenus(bot, DB)
	setupHandlers(bot, fsm)

	return bot
}

func setupInlineMenus(bot *tele.Bot, db *pgxpool.Pool) {
	//"ChangeName"
	//"ChangeAge"
	//"ChangeCity"
	//"ChangeTimezone"
	//"ChangeExperience"
	//"AccountSettingsMenuCancel"
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
				fmt.Println("ChangeName")
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
				fmt.Println("ChangeAge")
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
			return fmt.Errorf("/start[%d]: %w", userID, err)
		}

		if !ok {
			return fsm.UpdateState(c)
		}

		if ok := slices.Contains(MainUserMenuOptions, c.Text()); !ok {
			return c.Send("Не могу распознать ответ. Выбери вариант из меню")
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
