package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"vocal_training_bot/BotExt"

	"github.com/google/uuid"
	tele "gopkg.in/telebot.v3"
)

var (
	adminInlineMenus = BotExt.NewInlineMenus()
	adminFSM         = BotExt.NewFiniteStateMachine(adminInlineMenus)
)

func setupAdminHandlers(b *tele.Bot) {
	adminGroup := b.Group()
	adminGroup.Use(Whitelist(UGAdmin))
	adminGroup.Handle("/start", onStart)
	adminGroup.Handle(tele.OnText, onText)
	adminGroup.Handle(tele.OnMedia, onMedia)
	adminGroup.Handle(tele.OnCallback, OnAdminInlineResult)

	SetupAdminStates()
	SetupAdminMenuHandlers(b)
}

var (
	MainAdminMenuOptions = []string{
		"Отправить сообщения пользователям", BotExt.RowSplitterButton,
		"Группы распевок", "Добавить подбадривание",
		"Добавить распевку", "Изменить распевку",
		"Кто нажал на 'Стать учеником'?", "Забанить, Сделать админом",
	}
	MainAdminMenu = BotExt.ReplyMenuConstructor(MainAdminMenuOptions, 2, false)
)

func onStart(c tele.Context) error {
	return c.Send("Админ панель", MainAdminMenu)
}

func onText(c tele.Context) error {
	switch c.Text() {
	case "Отправить сообщения пользователям":
		userID := c.Sender().ID
		BotExt.SetStateVar(userID, "RecordID", uuid.New().String())
		adminFSM.Trigger(c, AdminSGRecordMessage)
		return nil

	case "Забанить, Сделать админом":
		err := sendUserList(c)
		if err != nil {
			return err
		}
		return c.Send("Выбери из списка человека, \n" +
			"нажми на него,\n" +
			"выбери ⬅Reply из меню,\n" +
			"в качестве текста напиши 'бан', 'админ' или 'юзер'\n" +
			"чтобы изменить группу пользователя`)")

	case "Группы распевок":
		err := adminInlineMenus.Show(c, warmupGroupAddGroupMenu)
		if err != nil {
			return err
		}
		_ = c.Send("Нажми на название чтобы переименовать")
		return adminInlineMenus.Show(c, warmupGroupAdminMenu)
	case "Добавить распевку":
		BotExt.SetStateVar(c.Sender().ID, "RecordID", uuid.New().String())
		adminFSM.Trigger(c, AdminSGAddWarmup)
		return nil
	case "Изменить распевку":
		return adminInlineMenus.Show(c, changeWarmupMenu)
	case "Добавить подбадривание":
		userID := c.Sender().ID
		BotExt.SetStateVar(userID, "RecordID", uuid.New().String())
		adminFSM.Trigger(c, AdminSGRecordCheerup)
		return nil
	case "Кто нажал на 'Стать учеником'?":
		return adminInlineMenus.Show(c, wannabeStudentResolutionMenu)
	}

	err, ok := handleUserGroupChange(c)
	if ok {
		return err
	}
	if err != nil {
		fmt.Println(fmt.Errorf("admin.onText: %w", err))
	}

	adminFSM.Update(c)
	return nil
}

func onMedia(c tele.Context) error {
	adminFSM.Update(c)
	return nil
}

func OnAdminInlineResult(c tele.Context) error {
	callback := c.Callback()
	triggeredData := strings.Split(callback.Data[1:len(callback.Data)], "|") // 1st - special callback symbol /f
	triggeredID := triggeredData[0]
	triggeredItem := triggeredData[1]

	switch triggeredItem {
	case wannabeStudentResolutionMenu:
		if userID, err := strconv.ParseInt(triggeredID, 10, 64); err == nil {
			return resolveWannabeStudent(c, userID)
		}
		return fmt.Errorf("OnAdminInlineResult: %s: can't parse userID", wannabeStudentResolutionMenu)
	case warmupGroupAdminMenu:
		BotExt.SetStateVar(c.Sender().ID, "selectedWarmupGroup", triggeredID)
		if adminFSM.GetCurrentState(c) == AdminSGAddWarmup {
			adminFSM.Update(c)
			return c.Respond()
		}
		adminFSM.Trigger(c, AdminSGRenameWarmupGroup)
	case changeWarmupMenu:
		BotExt.SetStateVar(c.Sender().ID, "selectedWarmup", triggeredID)
		///////////////// TODO: SHOW possiblle changes: group,name, price with inline menu. From that -> one shot FSM
	}

	return c.Respond()
}

func resolveWannabeStudent(c tele.Context, userID int64) error {
	var userName, userPhone, createdDate string
	err := DB.QueryRow(context.Background(), `
	SELECT user_name, phone_num, created::date::text FROM wannabe_student
	WHERE user_id = $1`, userID).Scan(&userName, &userPhone, &createdDate)
	if err != nil {
		return fmt.Errorf("resolveWannabeStudent: can't query row: %w", err)
	}

	text := fmt.Sprintf("@%s оставил(a) заявку %s", userName, createdDate)
	err = c.Send(text)
	if err != nil {
		return err
	}
	if userPhone != "" {
		params := map[string]string{
			"chat_id":      strconv.FormatInt(c.Sender().ID, 10),
			"phone_number": userPhone,
			"first_name":   userName,
		}
		_, err = c.Bot().Raw("sendContact", params)
		if err != nil {
			return err
		}
		_ = c.Send("Пользователь просил позвонить по телефону")
	}

	_, err = DB.Exec(context.Background(), `
	UPDATE wannabe_student
	SET resolved = true
	WHERE user_id = $1`, userID)

	return err
}

func sendUserList(c tele.Context) error {
	rows, err := DB.Query(context.Background(), `
		SELECT user_id, username, user_class from users
		ORDER BY user_class`)
	defer rows.Close()
	if err != nil {
		return fmt.Errorf("sendUserList: pg query error %w", err)
	}

	var userID int64
	var userName, userClass string
	for rows.Next() {
		err := rows.Scan(&userID, &userName, &userClass)
		if err != nil {
			fmt.Println(fmt.Errorf("sendUserList: users row scan error %w", err))
		}
		userLine := fmt.Sprintf("%d|%s|%s", userID, userName, userClass)
		err = c.Send(userLine)
		if err != nil {
			fmt.Println(fmt.Errorf("sendUserList: can't send message %w", err))
		}
	}
	return nil
}

func handleUserGroupChange(c tele.Context) (error, bool) {
	replyTo := c.Message().ReplyTo
	if replyTo == nil {
		return nil, false
	}
	if replyTo.Sender.ID != c.Bot().Me.ID {
		return nil, false
	}
	replyText := replyTo.Text
	if replyText == "" {
		return nil, false
	}
	replySplit := strings.Split(replyText, "|")
	if len(replySplit) != 3 {
		return nil, false
	}
	userID, err := strconv.ParseInt(replySplit[0], 10, 64)
	if err != nil {
		return err, false
	}
	switch strings.ToLower(c.Text()) {
	case "бан":
		err := c.Send(fmt.Sprintf("Пользователь %s[ID%s] теперь забанен", replySplit[1], replySplit[0]))
		if err != nil {
			fmt.Println(fmt.Errorf("handleTextWithReply: can't send message"))
		}
		return SetUserGroup(userID, UGBanned), true
	case "админ":
		err := c.Send(fmt.Sprintf("Пользователь %s[ID%s] теперь админ", replySplit[1], replySplit[0]))
		if err != nil {
			fmt.Println(fmt.Errorf("handleTextWithReply: can't send message"))
		}
		return SetUserGroup(userID, UGAdmin), true
	case "юзер":
		err := c.Send(fmt.Sprintf("Пользователь %s[ID%s] теперь обычный пользователь", replySplit[1], replySplit[0]))
		if err != nil {
			fmt.Println(fmt.Errorf("handleTextWithReply: can't send message"))
		}
		return SetUserGroup(userID, UGUser), true
	}
	return nil, false
}
