package main

import (
	"vocal_training_bot/BotExt"

	tele "gopkg.in/telebot.v3"
)

var (
	userInlineMenus = BotExt.NewInlineMenus()
	userFSM         = BotExt.NewFiniteStateMachine(userInlineMenus)
)

func UserFilterMiddleware(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		if ug, _ := GetUserGroup(c.Sender().ID); ug == UGUser {
			return next(c)
		}
		return nil
	}
}

func setupUserHandlers(b *tele.Bot) {
	userGroup := b.Group()
	userGroup.Use(UserFilterMiddleware)
	userGroup.Handle("/start", onUserStart)
	userGroup.Handle(tele.OnText, onUserText)

	SetupUserStates(userFSM)
	SetupUserMenuHandlers(b)
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
	case "Напоминания":
		return userInlineMenus.Show(c, WarmupNotificationsMenu)
	case "Записаться на урок":
	case "Обо мне":
	case "Настройки аккаунта":
		return userInlineMenus.Show(c, AccountSettingsMenu)
	}
	return nil
}
