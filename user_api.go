package main

import (
	"fmt"
	"strings"

	"vocal_training_bot/BotExt"

	tele "gopkg.in/telebot.v3"
)

var (
	userInlineMenus = BotExt.NewInlineMenus()
	userFSM         = BotExt.NewFiniteStateMachine(userInlineMenus)
)

func setupUserHandlers(b *tele.Bot) {
	b.Handle("/start", onUserStart)
	b.Handle(tele.OnText, onUserText)
	b.Handle(tele.OnContact, onUserText)
	b.Handle(tele.OnCallback, OnUserInlineResult)

	SetupUserStates(userFSM)
	SetupUserMenuHandlers(b)
}

func OnUserInlineResult(c tele.Context) error {
	callback := c.Callback()
	triggeredMenu := callback.Message.Text
	triggeredData := strings.Split(callback.Data[1:len(callback.Data)], "|") // 1st - special callback symbol /f
	triggeredID := triggeredData[0]
	triggeredItem := triggeredData[1]
	fmt.Println(triggeredMenu)
	fmt.Println(triggeredID)
	fmt.Println(triggeredItem)
	return c.Respond()
}

func onUserStart(c tele.Context) error {
	userID := c.Sender().ID

	if ok := UserIsInDatabase(userID); ok {
		return c.Reply("Привет! Ты зарегистрирован в боте, тебе доступна его функциональность!", MainUserMenu)
	}

	userFSM.Trigger(c, SurveySGStartSurveyReqName)
	return nil
}

func onUserText(c tele.Context) error {
	userID := c.Sender().ID

	if ok := BotExt.HasState(userID); ok {
		userFSM.Update(c)
		return nil
	}

	if ok := UserIsInDatabase(userID); !ok {
		return c.Send("Сначала надо пройти опрос.")
	}

	switch c.Text() {
	case "Распевки":
		return userInlineMenus.Show(c, WarmupGroupsMenu)
	case "Напоминания":
		return userInlineMenus.Show(c, WarmupNotificationsMenu)
	case "Записаться на урок":
		userFSM.Trigger(c, WannabeStudentSGSendReq)
		return nil
	case "Обо мне":
		return sendAboutMe(c)
	case "Настройки аккаунта":
		return userInlineMenus.Show(c, AccountSettingsMenu)
	}
	return nil
}

func sendAboutMe(c tele.Context) error {
	_ = c.Send("Меня зовут Юля. Я учу вокалу. Этот бот поможет тебе достичь высот в этом деле")
	_ = c.Send("Мой инстаграм: [@vershkovaaa](https://www.instagram.com/vershkovaaa/)", tele.ModeMarkdownV2, tele.NoPreview)
	_ = c.Send("Мой тикток: [@vershkovaaa](https://www.tiktok.com/@vershkovaaa)", tele.ModeMarkdownV2, tele.NoPreview)
	return nil
}
