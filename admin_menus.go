package main

import (
	"context"
	"fmt"

	"vocal_training_bot/BotExt"

	om "github.com/wk8/go-ordered-map/v2"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

const (
	wannabeStudentResolutionMenu = "wannabeStudentResolutionMenu"
	warmupGroupAdminMenu         = "warmupGroupAdminMenu"
	changeWarmupMenu             = "changeWarmupMenu"
	changeWarmupParamsMenu       = "changeWarmupParamsMenu"
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

	changeWarmupIM := BotExt.NewDynamicInlineMenu(
		changeWarmupMenu,
		"Список существующих распевок:",
		1,
		warmupListFetcher,
	)
	err = adminInlineMenus.RegisterMenu(b, changeWarmupIM)
	if err != nil {
		panic(err)
	}

	changeWarmupParamsIM := BotExt.NewInlineMenu(
		changeWarmupParamsMenu,
		"Параметры для изменения",
		1,
		warmupParamsFetcher,
	)
	changeWarmupParamsIM.AddButtons([]*BotExt.InlineButtonTemplate{
		{
			Unique: "ChangeWarmupGroup",
			TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["warmupGroup"]
				if !ok {
					return "Группа неизвестна", fmt.Errorf("can't fetch warmupGroup")
				}
				return "Группа: " + s, nil
			},
			OnClick: func(c tele.Context) error {
				adminFSM.Trigger(c, ChangeWarmupSetGroup, changeWarmupParamsMenu)
				return c.Respond()
			},
		},
		{
			Unique: "ChangeWarmupName",
			TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["warmupName"]
				if !ok {
					return "Название неизвестно", fmt.Errorf("can't fetch warmupName")
				}
				return "Название: " + s, nil
			},
			OnClick: func(c tele.Context) error {
				adminFSM.Trigger(c, ChangeWarmupSetName, changeWarmupParamsMenu)
				return c.Respond()
			},
		},
	})
	err = adminInlineMenus.RegisterMenu(b, changeWarmupParamsIM)
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
		err := c.Send("Активных заявок нет")
		if err != nil {
			logger.Error("can't send message", zap.Error(err))
		}
		return nil, BotExt.NoButtons
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
		err = c.Send("Категорий распевок пока нет")
		if err != nil {
			logger.Error("can't send message", zap.Error(err))
		}
		return nil, BotExt.NoButtons
	}

	return omap, nil
}

func warmupListFetcher(c tele.Context) (*om.OrderedMap[string, string], error) {
	rows, err := DB.Query(context.Background(), `
	SELECT warmup_id, warmup_name FROM warmups
	ORDER BY warmup_group`)
	defer rows.Close()
	if err != nil {
		return nil, fmt.Errorf("warmupListFetcher: can't fetch database: %w", err)
	}
	omap := om.New[string, string]()

	var warmupID, warmupName string

	for rows.Next() {
		err = rows.Scan(&warmupID, &warmupName)
		if err != nil {
			return omap, fmt.Errorf("warmupListFetcher: can't fetch row: %w", err)
		}
		omap.Set(warmupID, warmupName)
	}

	if omap.Len() == 0 {
		err = c.Send("Пока распевок нет")
		if err != nil {
			logger.Error("can't send message", zap.Error(err))
		}
		return nil, BotExt.NoButtons
	}

	return omap, nil
}

func warmupParamsFetcher(c tele.Context) (map[string]string, error) {
	userID := c.Sender().ID
	warmupID, ok := BotExt.GetStateVar(userID, "selectedWarmup")
	if !ok {
		return nil, fmt.Errorf("warmupParamsFetcher: can't fetch selectedWarmup")
	}

	var warmupGroup, warmupName, warmupPrice string
	err := DB.QueryRow(context.Background(), `
	SELECT group_name, warmup_name, price::text FROM warmups
	INNER JOIN warmup_groups ON warmups.warmup_group = warmup_groups.warmup_group_id                                                    
	WHERE warmup_id = $1`, warmupID).Scan(&warmupGroup, &warmupName, &warmupPrice)
	if err != nil {
		return nil, fmt.Errorf("warmupParamsFetcher: can't fetch %s warmup data: %w", warmupID, err)
	}

	out := make(map[string]string)
	out["warmupGroup"] = warmupGroup
	out["warmupName"] = warmupName
	out["warmupPrice"] = warmupPrice

	return out, nil
}
