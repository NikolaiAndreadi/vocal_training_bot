package main

import (
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
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

		cheerupRecordID, err := getRandomCheerup()
		if (err != nil) && (err == pgx.ErrNoRows) {
			return fmt.Errorf("notificationService.handler: %w", err)
		}

		if cheerupRecordID != "" {
			return SendMessageToUser(bot, userID, cheerupRecordID, false)
		}
		return nil
	}

	return bot
}

func onStart(c tele.Context) error {
	ug, _ := GetUserGroup(c.Sender().ID)
	c.Set("route", "onStart")
	c.Set("userGroup", string(ug))
	switch ug {
	case UGAdmin:
		return onAdminStart(c)
	case UGUser:
		return onUserStart(c)
	case UGNewUser:
		return onUserStart(c)
	}
	return nil
}

func onCallback(c tele.Context) error {
	ug, _ := GetUserGroup(c.Sender().ID)
	c.Set("route", "onCallback")
	c.Set("userGroup", string(ug))

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
	c.Set("route", "onText")
	c.Set("userGroup", string(ug))

	switch ug {
	case UGAdmin:
		return onAdminText(c)
	case UGUser:
		return onUserText(c)
	case UGNewUser:
		return onUserText(c)
	}
	return nil
}

func onMedia(c tele.Context) error {
	ug, _ := GetUserGroup(c.Sender().ID)
	c.Set("route", "onMedia")
	c.Set("userGroup", string(ug))

	switch ug {
	case UGAdmin:
		return onAdminMedia(c)
	}
	return nil
}

func onContact(c tele.Context) error {
	ug, _ := GetUserGroup(c.Sender().ID)
	c.Set("route", "onContact")
	c.Set("userGroup", string(ug))

	switch ug {
	case UGUser:
		return onUserText(c)
	}
	return nil
}

func onCheckout(c tele.Context) error {
	ug, _ := GetUserGroup(c.Sender().ID)
	c.Set("route", "onCheckout")
	c.Set("userGroup", string(ug))

	switch ug {
	case UGUser:
		return onUserCheckout(c)
	}
	return nil
}
