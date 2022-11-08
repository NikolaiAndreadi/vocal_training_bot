package main

import (
	"strconv"
	"time"

	"vocal_training_bot/BotExt"

	"golang.org/x/exp/slices"
	tele "gopkg.in/telebot.v3"
)

func AdminFilterMiddleware(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		if ug, _ := GetUserGroup(c.Sender().ID); ug == UGAdmin {
			return next(c)
		}
		return nil
	}
}

var (
	adminInlineMenus = BotExt.NewInlineMenus()
	adminFSM         = BotExt.NewFiniteStateMachine(adminInlineMenus)
)

func setupAdminHandlers(b *tele.Bot) {
	adminGroup := b.Group()
	adminGroup.Use(AdminFilterMiddleware)
	adminGroup.Handle("/start", onStart)
	adminGroup.Handle(tele.OnText, onText)
	adminGroup.Handle(tele.OnMedia, onMedia)

	SetupAdminStates()
	SetupAdminMenuHandlers(b)
}

var (
	MainAdminMenuOptions = []string{
		"Разослать сообщения пользователям", BotExt.RowSplitterButton,
		"Добавить подбадривание для распевок", BotExt.RowSplitterButton,
		"Меню распевок", "Кто нажал на 'Стать учеником'?",
		"БАН-лист", "АДМИН-лист",
	}
	MainAdminMenu = BotExt.ReplyMenuConstructor(MainAdminMenuOptions, 2, false)
)

func onStart(c tele.Context) error {
	return c.Send("Админ панель", MainAdminMenu)
}

func onText(c tele.Context) error {
	switch c.Text() {
	case "Разослать сообщения пользователям":
		userID := c.Sender().ID
		recordID := strconv.FormatInt(userID, 10) +
			strconv.FormatInt(time.Now().UTC().Unix(), 10)
		BotExt.SetStateVar(userID, "RecordID", recordID)
		adminFSM.Trigger(c, AdminSGRecordMessage)
		return nil
	}
	if slices.Contains(MainAdminMenuOptions, c.Text()) {
		return nil
	}
	adminFSM.Update(c)
	return nil
}

func onMedia(c tele.Context) error {
	adminFSM.Update(c)
	return nil
}

//params := map[string]string{
//"chat_id": strconv.FormatInt(c.Sender().ID, 10),
//
//"phone_number": "+79153303033",
//"first_name":   "pupok",
//}
//
//_, err := c.Bot().Raw("sendContact", params)
//return err
