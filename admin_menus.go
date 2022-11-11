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
	warmupGroupAdminMenu         = "warmupGroupAdminMenu"
	warmupGroupAddGroupMenu      = "warmupGroupAddGroupMenu"
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

	warmupGroupAdminIM := BotExt.NewDynamicInlineMenu(
		warmupGroupAdminMenu,
		"Существующие группы распевок:",
		1,
		warmupGroupAdminFetcher,
	)
	err = adminInlineMenus.RegisterMenu(b, warmupGroupAdminIM)
	if err != nil {
		panic(err)
	}

	warmupGroupAddGroupIM := BotExt.NewInlineMenu(
		warmupGroupAddGroupMenu,
		"Добавить новую группу. Внимание! Удалить ее будет уже нельзя.",
		1,
		nil,
	)
	warmupGroupAddGroupIM.AddButton(&BotExt.InlineButtonTemplate{
		Unique:         "AddWarmupGroup",
		TextOnCreation: "➕",
		OnClick: func(c tele.Context) error {
			adminFSM.Trigger(c, AdminSGAddGroupMenu, warmupGroupAddGroupMenu)
			return c.Respond()
		},
	})
	err = adminInlineMenus.RegisterMenu(b, warmupGroupAddGroupIM)
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

func warmupGroupAdminFetcher(c tele.Context) (*om.OrderedMap[string, string], error) {
	rows, err := DB.Query(context.Background(), `
	SELECT warmup_group_id::text, group_name FROM warmup_groups
	ORDER BY warmup_group_id`)
	defer rows.Close()
	if err != nil {
		return nil, fmt.Errorf("warmupGroupAdminFetcher: can't fetch database: %w", err)
	}
	omap := om.New[string, string]()

	var groupID, groupName string

	for rows.Next() {
		err = rows.Scan(&groupID, &groupName)
		if err != nil {
			return omap, fmt.Errorf("warmupGroupAdminFetcher: can't fetch row: %w", err)
		}
		omap.Set(groupID, groupName)
	}

	if omap.Len() == 0 {
		return nil, c.Send("Категорий распевок пока нет")
	}

	return omap, nil
}
