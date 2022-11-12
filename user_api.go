package main

import (
	"context"
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
	SetupUserStates(userFSM)
	SetupUserMenuHandlers(b)
}

func OnUserInlineResult(c tele.Context) error {
	callback := c.Callback()
	triggeredData := strings.Split(callback.Data[1:len(callback.Data)], "|") // 1st - special callback symbol /f
	triggeredID := triggeredData[0]
	triggeredItem := triggeredData[1]

	switch triggeredItem {
	case WarmupGroupsMenu:
		BotExt.SetStateVar(c.Sender().ID, "selectedWarmupGroup", triggeredID)
		err := userInlineMenus.Show(c, WarmupsMenu)
		if err != nil {
			fmt.Println(fmt.Errorf("OnUserInlineResult: WarmupGroupsMenu: %w", err))
		}
		return c.Respond()
	case WarmupsMenu:
		err := processWarmups(c, triggeredID)
		if err != nil {
			return fmt.Errorf("OnUserInlineResult: WarmupsMenu: %w", err)
		}
		return c.Respond()
	}

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

func processWarmups(c tele.Context, warmupID string) error {
	var (
		acquired   bool
		price      int
		recordID   string
		warmupName string
	)
	err := DB.QueryRow(context.Background(), `
	SELECT COALESCE(acquired, false), price, record_id, warmup_name FROM warmups
		LEFT JOIN (
			SELECT warmup_id, true AS acquired 
			FROM acquired_warmups 
			WHERE user_id = $1) AS acquired_warmups ON warmups.warmup_id = acquired_warmups.warmup_id
	WHERE warmups.warmup_id = $2`, c.Sender().ID, warmupID).Scan(&acquired, &price, &recordID, &warmupName)
	if err != nil {
		return fmt.Errorf("processWarmups: can't select row: %w", err)
	}

	if (price == 0) || acquired {
		err = SendMessageToUser(c.Bot(), c.Sender().ID, recordID, true)
		if err != nil {
			return fmt.Errorf("processWarmups: SendMessageToUser: %w", err)
		}
		return nil
	}

	invoice := &tele.Invoice{
		Title:       "Покупка распевки",
		Description: "Распевка " + warmupName,
		Payload:     warmupID,
		Currency:    "RUB",
		Prices: []tele.Price{
			{
				Label:  "RUB",
				Amount: price * 100,
			},
		},
		Token: ProviderToken,
	}
	return c.Send(invoice)
}
