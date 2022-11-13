package main

import (
	"context"
	"fmt"

	"vocal_training_bot/BotExt"

	om "github.com/wk8/go-ordered-map/v2"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

var (
	MainUserMenuOptions = []string{
		"Распевки",
		"Напоминания",
		"Записаться на урок",
		"Обо мне",
		"Настройки аккаунта",
	}
	MainUserMenu = BotExt.ReplyMenuConstructor(MainUserMenuOptions, 2, false)
)

const (
	AccountSettingsMenu     = "AccountSettingsMenu"
	WarmupNotificationsMenu = "WarmupNotificationsMenu"
	WarmupGroupsMenu        = "WarmupGroupsMenu"
	WarmupsMenu             = "WarmupsMenu"
)

var (
	experienceAllowedAnswers = []string{"без опыта", "менее 1 года", "1-2 года", "2-3 года", "3-5 лет", "более 5 лет"}
	experienceReplyMenu      = BotExt.ReplyMenuConstructor(experienceAllowedAnswers, 2, true)

	wannabeStudentMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
)

func SetupUserMenuHandlers(bot *tele.Bot) {
	wannabeStudentMenu.Reply(
		wannabeStudentMenu.Row(wannabeStudentMenu.Text("Написать в личку в телеграм")),
		wannabeStudentMenu.Row(wannabeStudentMenu.Contact("Позвонить")),
		wannabeStudentMenu.Row(wannabeStudentMenu.Text("Отмена")),
	)

	cancelButton := &BotExt.InlineButtonTemplate{
		Unique:         "Cancel",
		TextOnCreation: "Отмена",
		OnClick: func(c tele.Context) error {
			BotExt.ResetState(c.Sender().ID, false)
			if err := c.Send("OK", MainUserMenu); err != nil {
				logger.Error("can't send OK button", zap.Int64("userID", c.Sender().ID), zap.Error(err))
			}
			return c.Respond()
		},
	}

	AccountSettingsIM := BotExt.NewInlineMenu(
		AccountSettingsMenu,
		"Текущие настройки: нажми на пункт, чтобы изменить",
		1,
		func(c tele.Context) (map[string]string, error) {
			var name, age, city, tz, xp string
			err := DB.QueryRow(context.Background(),
				"SELECT username, text(age), city, timezone_txt, experience FROM users WHERE user_id = $1",
				c.Sender().ID).Scan(&name, &age, &city, &tz, &xp)
			if err != nil {
				return nil, err
			}
			data := map[string]string{
				"name":       name,
				"age":        age,
				"city":       city,
				"timezone":   tz,
				"experience": xp,
			}
			return data, nil
		},
	)
	AccountSettingsIM.AddButtons([]*BotExt.InlineButtonTemplate{
		{
			Unique: "ChangeName",
			TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["name"]
				if !ok {
					return "Имя неизвестно", fmt.Errorf("can't fetch name")
				}
				return "Имя: " + s, nil
			},
			OnClick: func(c tele.Context) error {
				userFSM.Trigger(c, SettingsSGSetName, AccountSettingsMenu)
				return c.Respond()
			},
		},
		{
			Unique: "ChangeAge",
			TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["age"]
				if !ok {
					return "Возраст неизвестен", fmt.Errorf("can't fetch age")
				}
				return "Возраст: " + s, nil
			},
			OnClick: func(c tele.Context) error {
				userFSM.Trigger(c, SettingsSGSetAge, AccountSettingsMenu)
				return c.Respond()
			},
		},
		{
			Unique: "ChangeCity",
			TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["city"]
				if !ok {
					return "Город неизвестен", fmt.Errorf("can't fetch city")
				}
				return "Город: " + s, nil
			},
			OnClick: func(c tele.Context) error {
				userFSM.Trigger(c, SettingsSGSetCity, AccountSettingsMenu)
				return c.Respond()
			},
		},
		{
			Unique: "ChangeTimezone",
			TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["timezone"]
				if !ok {
					return "Часовой пояс неизвестен", fmt.Errorf("can't fetch timezone")
				}
				return "Часовой пояс: " + s, nil
			},
			OnClick: func(c tele.Context) error {
				userFSM.Trigger(c, SettingsSGSetTimezone, AccountSettingsMenu)
				return c.Respond()
			},
		},
		{
			Unique: "ChangeExperience",
			TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["experience"]
				if !ok {
					return "Опыт вокала неизвестен", fmt.Errorf("can't fetch experience")
				}
				return "Опыт вокала: " + s, nil
			},
			OnClick: func(c tele.Context) error {
				userFSM.Trigger(c, SettingsSGSetExperience, AccountSettingsMenu)
				return c.Respond()
			},
		},
		cancelButton,
	})
	err := userInlineMenus.RegisterMenu(bot, AccountSettingsIM)
	if err != nil {
		panic(err)
	}

	warmupNotificationIM := BotExt.NewInlineMenu(
		WarmupNotificationsMenu,
		"Настройки напоминаний о распевках:",
		2,
		WarmupNotificationsMenuDataFetcher,
	)
	mon := NotificationButtonFabric(userFSM, userInlineMenus, "mon", "Понедельник")
	tue := NotificationButtonFabric(userFSM, userInlineMenus, "tue", "Вторник")
	wed := NotificationButtonFabric(userFSM, userInlineMenus, "wed", "Среда")
	thu := NotificationButtonFabric(userFSM, userInlineMenus, "thu", "Четверг")
	fri := NotificationButtonFabric(userFSM, userInlineMenus, "fri", "Пятница")
	sat := NotificationButtonFabric(userFSM, userInlineMenus, "sat", "Суббота")
	sun := NotificationButtonFabric(userFSM, userInlineMenus, "sun", "Воскресенье")
	warmupNotificationIM.AddButtons([]*BotExt.InlineButtonTemplate{
		mon[0], mon[1],
		tue[0], tue[1],
		wed[0], wed[1],
		thu[0], thu[1],
		fri[0], fri[1],
		sat[0], sat[1],
		sun[0], sun[1],
		{Unique: BotExt.RowSplitterButton},
		{
			Unique: "GlobalSwitch",
			TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
				s := "Глобальный выключатель: "
				v, ok := dc["globalOn"]
				if !ok {
					return s + "???", fmt.Errorf("can't fetch globalOn")
				}
				if v == "true" {
					return s + "🔔", nil
				}
				return s + "🔕", nil
			},
			OnClick: func(c tele.Context) error {
				var res bool
				userID := c.Sender().ID
				err := DB.QueryRow(context.Background(), `
					UPDATE warmup_notification_global
					SET global_switch = NOT global_switch
					WHERE user_id = $1
					RETURNING global_switch`, userID).Scan(&res)
				if err != nil {
					logger.Error("can't switch global notifications",
						zap.Int64("userID", userID), zap.Error(err))
				}
				userInlineMenus.Update(c, WarmupNotificationsMenu)
				if res {
					err = notificationService.AddUser(userID)
				} else {
					err = notificationService.DelUser(userID)
				}
				if err != nil {
					logger.Error("can't switch global notifications",
						zap.Int64("userID", userID), zap.Error(err))
				}
				return c.Respond()
			}},
		cancelButton,
	})
	err = userInlineMenus.RegisterMenu(bot, warmupNotificationIM)
	if err != nil {
		panic(err)
	}

	warmupGroupsIM := BotExt.NewDynamicInlineMenu(
		WarmupGroupsMenu,
		"Категории распевок",
		1,
		warmupGroupsFetcher,
	)
	err = userInlineMenus.RegisterMenu(bot, warmupGroupsIM)
	if err != nil {
		panic(err)
	}

	warmupsIM := BotExt.NewDynamicInlineMenu(
		WarmupsMenu,
		"Распевки:",
		1,
		warmupsFetcher)
	err = userInlineMenus.RegisterMenu(bot, warmupsIM)
	if err != nil {
		panic(err)
	}
}

func WarmupNotificationsMenuDataFetcher(c tele.Context) (map[string]string, error) {
	rows, err := DB.Query(context.Background(), `
				SELECT 
				    day_of_week,
    				cast(trigger_switch AS varchar(5)), 
       				to_char(trigger_time,'HH24:MI')
				FROM warmup_notifications WHERE user_id = $1`, c.Sender().ID)
	defer rows.Close()
	if err != nil {
		return nil, err
	}
	data := make(map[string]string)
	var dayName, daySwitch, notificationTime string
	for rows.Next() {
		err = rows.Scan(&dayName, &daySwitch, &notificationTime)
		if err != nil {
			return data, err
		}
		data[dayName+"On"] = daySwitch
		data[dayName+"Time"] = notificationTime
	}
	if err := rows.Err(); err != nil {
		return data, fmt.Errorf("WarmupNotificationsMenuDataFetcher: postgres itetator %w", err)
	}

	var globalSwitch string
	err = DB.QueryRow(context.Background(),
		`SELECT cast(global_switch AS varchar(5)) FROM warmup_notification_global WHERE user_id = $1`,
		c.Sender().ID).Scan(&globalSwitch)
	if err != nil {
		return data, err
	}
	data["globalOn"] = globalSwitch

	return data, nil
}

func NotificationButtonFabric(fsm *BotExt.FSM, ims *BotExt.InlineMenusType, dayUnique string, dayText string) (ibt [2]*BotExt.InlineButtonTemplate) {
	// switch
	ibt[0] = &BotExt.InlineButtonTemplate{
		Unique: "NotificationSwitch_" + dayUnique,
		TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
			s := dayText + ": "
			v, ok := dc[dayUnique+"On"]
			if !ok {
				return s + "???", fmt.Errorf("can't fetch %sOn", dayUnique)
			}
			if v == "true" {
				return s + "🔔", nil
			}
			return s + "🔕", nil
		},
		OnClick: func(c tele.Context) error {
			userID := c.Sender().ID
			_, err := DB.Exec(context.Background(), `
			UPDATE warmup_notifications
			SET trigger_switch = NOT trigger_switch
			WHERE (user_id = $1) AND (day_of_week = $2)`, userID, dayUnique)
			if err != nil {
				logger.Error("can't switch notifications for day",
					zap.Int64("userID", userID), zap.Error(err))
			}
			ims.Update(c, WarmupNotificationsMenu)

			ts, err := getNearestNotificationFromPg(userID)
			if err != nil {
				logger.Error("can't switch notifications for day",
					zap.Int64("userID", userID), zap.String("dayUnique", dayUnique), zap.Error(err))
			}
			if err = notificationService.DelUser(userID); err != nil {
				logger.Error("can't switch notifications for day",
					zap.Int64("userID", userID), zap.String("dayUnique", dayUnique), zap.Error(err))
			}
			if err = notificationService.addUser(userID, ts); err != nil {
				logger.Error("can't switch notifications for day",
					zap.Int64("userID", userID), zap.String("dayUnique", dayUnique), zap.Error(err))
			}
			return c.Respond()
		},
	}
	// set time
	ibt[1] = &BotExt.InlineButtonTemplate{
		Unique: "NotificationTime_" + dayUnique,
		TextOnCreation: func(c tele.Context, dc map[string]string) (string, error) {
			time, ok := dc[dayUnique+"Time"]
			if !ok {
				return "HH:MM", fmt.Errorf("can't fetch %sTime", dayUnique)
			}
			return time, nil
		},
		OnClick: func(c tele.Context) error {
			BotExt.SetStateVar(c.Sender().ID, "day", dayUnique)
			fsm.Trigger(c, NotificationSGSetTime, WarmupNotificationsMenu)
			return c.Respond()
		},
	}
	return
}

func warmupGroupsFetcher(c tele.Context) (*om.OrderedMap[string, string], error) {
	rows, err := DB.Query(context.Background(), `
		SELECT warmup_group_id, group_name FROM warmup_groups
		INNER JOIN (SELECT DISTINCT warmup_group FROM warmups) AS not_empty_groups 
		ON warmup_groups.warmup_group_id = not_empty_groups.warmup_group`)
	defer rows.Close()
	if err != nil {
		return nil, fmt.Errorf("warmupGroupsFetcher: can't fetch database: %w", err)
	}
	omap := om.New[string, string]()

	var unique, text string
	for rows.Next() {
		err = rows.Scan(&unique, &text)
		if err != nil {
			return omap, fmt.Errorf("warmupGroupsFetcher: can't fetch row: %w", err)
		}
		omap.Set(unique, text)
	}

	if omap.Len() == 0 {
		return nil, c.Send("Пока в этом разделе пусто... Скоро тут будет много интересного!")
	}

	return omap, nil
}

func warmupsFetcher(c tele.Context) (*om.OrderedMap[string, string], error) {
	groupID, ok := BotExt.GetStateVar(c.Sender().ID, "selectedWarmupGroup")
	if !ok {
		return nil, fmt.Errorf("warmupsFetcher: can't get var selectedWarmupGroup")
	}

	rows, err := DB.Query(context.Background(), `
		SELECT warmups.warmup_id::text, warmup_name, price::text, COALESCE(acquired, false) FROM warmups
		LEFT JOIN (
			SELECT warmup_id, true AS acquired 
			FROM acquired_warmups 
			WHERE user_id = $1) AS acquired_warmups ON warmups.warmup_id = acquired_warmups.warmup_id
		WHERE warmup_group = $2
		ORDER BY price DESC`, c.Sender().ID, groupID)
	defer rows.Close()
	if err != nil {
		return nil, fmt.Errorf("warmupsFetcher: can't fetch database: %w", err)
	}
	omap := om.New[string, string]()

	var warmupID, warmupName, warmupPrice string
	var acquired bool
	for rows.Next() {
		err = rows.Scan(&warmupID, &warmupName, &warmupPrice, &acquired)
		if err != nil {
			return omap, fmt.Errorf("warmupsFetcher: can't fetch row: %w", err)
		}
		var priceText string
		if (warmupPrice == "0") && !acquired {
			priceText = "🎁 бесплатно"
		} else {
			if acquired {
				priceText = "🤑 куплено"
			} else {
				priceText = "💳 " + warmupPrice + " рублей"
			}
		}
		text := fmt.Sprintf("%s [%s]", warmupName, priceText)
		omap.Set(warmupID, text)
	}

	if omap.Len() == 0 {
		return nil, c.Send("Пока в этом разделе пусто... Скоро тут будет много интересного!")
	}

	return omap, nil
}
