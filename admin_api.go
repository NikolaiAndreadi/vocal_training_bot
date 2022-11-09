package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"vocal_training_bot/BotExt"

	"github.com/google/uuid"
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
		"Отправить сообщения пользователям", BotExt.RowSplitterButton,
		"Группы распевок", "Добавить распевку",
		"Добавить подбадривание", "Кто нажал на 'Стать учеником'?",
		"Забанить, Сделать админом", BotExt.RowSplitterButton,
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
		BotExt.SetStateVar(userID, "RecordID", uuid.New().String())
		adminFSM.Trigger(c, AdminSGRecordMessage)
		return nil
	case "Забанить, Сделать админом":
		err := sendUserList(c)
		if err != nil {
			return err
		}
		return c.Send(`Выбери из списка человека, 
нажми на него, 
выбери ⬅Reply из меню, 
в качестве текста напиши 'бан', 'админ' или 'юзер'
чтобы изменить группу пользователя`)
	}
	if slices.Contains(MainAdminMenuOptions, c.Text()) {
		return nil
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

//params := map[string]string{
//"chat_id": strconv.FormatInt(c.Sender().ID, 10),
//
//"phone_number": "+79153303033",
//"first_name":   "pupok",
//}
//
//_, err := c.Bot().Raw("sendContact", params)
//return err

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
	if c.Message().ReplyTo.Sender.ID != c.Bot().Me.ID {
		return nil, false
	}
	replyText := c.Message().ReplyTo.Text
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
