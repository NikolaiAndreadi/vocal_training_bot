package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"vocal_training_bot/BotExt"

	tele "gopkg.in/telebot.v3"
)

var (
	userInlineMenus = BotExt.NewInlineMenus()
	userFSM         = BotExt.NewFiniteStateMachine(userInlineMenus)
)

const (
	WarmupPayloadChecker = "BuyWarmup"
	PayloadSplit         = "|"
	PaymentErrorText     = "Произошла ошибка при проведении платежа!"
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

func onUserCheckout(c tele.Context) error {
	checkout := c.PreCheckoutQuery()

	userID := c.Sender().ID
	checkoutID := checkout.ID

	payloadData := strings.Split(checkout.Payload, PayloadSplit)
	if len(payloadData) != 2 {
		fmt.Println(fmt.Errorf("onUserCheckout[%d->%s]: can't extract payloadData '%s'", userID, checkoutID, payloadData))
		return c.Bot().Accept(checkout, PaymentErrorText)
	}
	if payloadData[0] != WarmupPayloadChecker {
		fmt.Println(fmt.Errorf("onUserCheckout[%d->%s]: unknown payload checker '%s'", userID, checkoutID, payloadData))
		return c.Bot().Accept(checkout, PaymentErrorText)
	}
	warmupID := payloadData[1]
	priceWhenAcquired := strconv.Itoa(checkout.Total) + checkout.Currency

	var warmupName string
	var dbPrice int
	err := DB.QueryRow(context.Background(), `
		SELECT warmup_name, price*100 FROM warmups
		WHERE warmup_id = $1`, warmupID).Scan(&warmupName, &dbPrice)
	if err != nil {
		fmt.Println(fmt.Errorf("onUserCheckout[%d->%s warmup %s]: can't find warmup in db: %w", userID, checkoutID, warmupID, err))
		return c.Bot().Accept(checkout, PaymentErrorText)
	}

	if dbPrice != checkout.Total {
		fmt.Println(fmt.Errorf("onUserCheckout[%d->%s warmup %s]: price in database (%d) not equals checkout price (%d)",
			userID, checkoutID, warmupID, dbPrice, checkout.Total))
		return c.Bot().Accept(checkout, PaymentErrorText)
	}

	_, err = DB.Exec(context.Background(), `
		INSERT INTO acquired_warmups(user_id, warmup_id, checkout_id, price_when_acquired)
		VALUES ($1, $2, $3, $4)`, userID, warmupID, checkoutID, priceWhenAcquired)
	if err != nil {
		fmt.Println(fmt.Errorf("onUserCheckout[%d->%s warmup %s]: exec db error: %w", userID, checkoutID, warmupID, err))
		return c.Bot().Accept(checkout, PaymentErrorText)
	}

	_ = c.Send("Распевка '" + warmupName + "' преобретена! Теперь она доступна для просмотра в меню Распевки")
	fmt.Println(fmt.Printf("onUserCheckout[%d]: successful payment of warmupID %s for %s", userID, warmupID, priceWhenAcquired))
	return c.Bot().Accept(checkout)
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
	err := c.Send("Меня зовут [Юля](https://t.me/vershkovaaa) Я учу вокалу\\. Этот бот поможет тебе достичь высот в этом деле", tele.ModeMarkdownV2)
	if err != nil {
		return err
	}
	err = c.Send("Мой инстаграм: [@vershkovaaa](https://www.instagram.com/vershkovaaa)", tele.ModeMarkdownV2, tele.NoPreview)
	if err != nil {
		return err
	}
	err = c.Send("Мой тикток: [@vershkovaaa](https://www.tiktok.com/@vershkovaaa)", tele.ModeMarkdownV2, tele.NoPreview)
	if err != nil {
		return err
	}
	err = c.Send("Бот сделал: [@NikolaiAndreadi](https://t.me/NikolaiAndreadi)", tele.ModeMarkdownV2, tele.NoPreview)
	if err != nil {
		return err
	}
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
		Description: "Распевка '" + warmupName + "'",
		Payload:     WarmupPayloadChecker + PayloadSplit + warmupID,
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
