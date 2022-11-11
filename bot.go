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

func InitAdminBot(cfg Config) *tele.Bot {
	teleCfg := tele.Settings{
		Token: cfg.AdminBot.Token,
	}
	bot, err := tele.NewBot(teleCfg)
	if err != nil {
		panic(fmt.Errorf("InitAdminBot: %w", err))
	}
	bot.Use(Whitelist(UGAdmin))
	setupAdminHandlers(bot)
	return bot
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

	notificationService.handler = func(userID int64) error {
		_, err = bot.Send(UserIDType{userID}, "❗ НАПОМИНАНИЕ ❗ Пришло время делать распевку")
		if err != nil {
			return err
		}

		warmupID, err := getRandomCheerup()
		if err != nil {
			return fmt.Errorf("notificationService.handler: %w", err)
		}

		if warmupID != "" {
			return SendMessageToUser(bot, userID, warmupID)
		}
		return nil
	}

	return bot
}
