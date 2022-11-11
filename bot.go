package main

import (
	"fmt"
	"strconv"

	tele "gopkg.in/telebot.v3"
)

type UserIDType struct {
	UserID int64
}

func (u UserIDType) Recipient() string {
	return strconv.FormatInt(u.UserID, 10)
}

func InitBot(cfg Config) *tele.Bot {
	teleCfg := tele.Settings{
		Token: cfg.Bot.Token,
	}
	bot, err := tele.NewBot(teleCfg)
	if err != nil {
		panic(fmt.Errorf("InitBot: %w", err))
	}
	bot.Use(Blacklist(UGBanned))

	setupUserHandlers(bot)
	setupAdminHandlers(bot)

	notificationService.handler = func(userID int64) error {
		warmupText, err := getRandomCheerup()
		if err != nil {
			return fmt.Errorf("notificationService.handler: %w", err)
		}
		_, sendErr := bot.Send(UserIDType{userID}, warmupText)
		return sendErr
	}

	return bot
}
