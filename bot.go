package main

import (
	"fmt"
	"strconv"

	tele "gopkg.in/telebot.v3"
)

type UserIDType struct {
	UserID int64
}

var ProviderToken string

func (u UserIDType) Recipient() string {
	return strconv.FormatInt(u.UserID, 10)
}

func InitBot(cfg Config) *tele.Bot {
	teleCfg := tele.Settings{
		Token: cfg.Bot.Token,
	}
	ProviderToken = cfg.Bot.ProviderToken
	bot, err := tele.NewBot(teleCfg)
	if err != nil {
		panic(fmt.Errorf("InitBot: %w", err))
	}

	bot.Use(MiddlewareLogger(logger))

	bot.Handle("/start", onStart)
	bot.Handle(tele.OnText, onText)
	bot.Handle(tele.OnCallback, onCallback)
	bot.Handle(tele.OnMedia, onMedia)
	bot.Handle(tele.OnContact, onContact)
	bot.Handle(tele.OnCheckout, onCheckout)

	setupUserHandlers(bot)
	setupAdminHandlers(bot)

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
			return SendMessageToUser(bot, userID, warmupID, false)
		}
		return nil
	}

	return bot
}

func onStart(c tele.Context) error {
	ug, _ := GetUserGroup(c.Sender().ID)
	switch ug {
	case UGAdmin:
		return onAdminStart(c)
	case UGUser, UGNewUser:
		return onUserStart(c)
	}
	return nil
}

func onCallback(c tele.Context) error {
	ug, _ := GetUserGroup(c.Sender().ID)
	switch ug {
	case UGAdmin:
		return OnAdminInlineResult(c)
	case UGUser:
		return OnUserInlineResult(c)
	}
	return nil
}

func onText(c tele.Context) error {
	ug, _ := GetUserGroup(c.Sender().ID)
	switch ug {
	case UGAdmin:
		return onAdminText(c)
	case UGUser, UGNewUser:
		return onUserText(c)
	}
	return nil
}

func onMedia(c tele.Context) error {
	ug, _ := GetUserGroup(c.Sender().ID)
	switch ug {
	case UGAdmin:
		return onAdminMedia(c)
	}
	return nil
}

func onContact(c tele.Context) error {
	ug, _ := GetUserGroup(c.Sender().ID)
	switch ug {
	case UGUser:
		return onUserText(c)
	}
	return nil
}

func onCheckout(c tele.Context) error {
	ug, _ := GetUserGroup(c.Sender().ID)
	switch ug {
	case UGUser:
		return onUserCheckout(c)
	}
	return nil
}
