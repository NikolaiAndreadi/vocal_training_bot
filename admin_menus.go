package main

import (
	"context"
	"fmt"

	"vocal_training_bot/BotExt"

	om "github.com/wk8/go-ordered-map/v2"
	tele "gopkg.in/telebot.v3"
)

const (
	wannabeStudentResolutionMenu = "wannabeStudentResolutionMenu"
)

func SetupAdminMenuHandlers(b *tele.Bot) {
	wannabeStudentResolutionIM := BotExt.NewDynamicInlineMenu(
		wannabeStudentResolutionMenu,
		"Существующие заявки",
		1,
		wannabeStudentResolutionFetcher,
	)
	err := adminInlineMenus.RegisterMenu(b, wannabeStudentResolutionIM)
	if err != nil {
		panic(err)
	}
}

func wannabeStudentResolutionFetcher(c tele.Context) (*om.OrderedMap[string, string], error) {
	rows, err := DB.Query(context.Background(), `
	SELECT user_id::text, user_name FROM wannabe_student
	WHERE resolved = false
	ORDER BY created DESC`)
	defer rows.Close()
	if err != nil {
		return nil, fmt.Errorf("wannabeStudentResolutionFetcher: can't fetch database: %w", err)
	}
	omap := om.New[string, string]()

	var userID, userName string

	for rows.Next() {
		err = rows.Scan(&userID, &userName)
		if err != nil {
			return omap, fmt.Errorf("wannabeStudentResolutionFetcher: can't fetch row: %w", err)
		}
		text := "Пользователь @" + userName
		omap.Set(userID, text)
	}

	if omap.Len() == 0 {
		return nil, c.Send("Активных заявок нет")
	}

	return omap, nil
}
