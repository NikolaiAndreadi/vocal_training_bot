package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
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
	MainUserMenu = ReplyMenuConstructor(MainUserMenuOptions, 2, false)

	AccountSettingsMenu     *InlineMenu
	WarmupNotificationsMenu *InlineMenu
)

func InitBot(cfg Config) *tele.Bot {
	teleCfg := tele.Settings{
		Token: cfg.Bot.Token,
	}
	bot, err := tele.NewBot(teleCfg)
	if err != nil {
		panic(fmt.Errorf("InitBot: %w", err))
	}

	fsm := SetupStates(DB)
	setupInlineMenus(bot, DB, fsm)
	setupHandlers(bot, fsm)

	return bot
}

func setupInlineMenus(bot *tele.Bot, db *pgxpool.Pool, fsm *FSM) {
	AccountSettingsMenu = NewInlineMenu("Текущие настройки: нажми на пункт, чтобы изменить",
		func(c tele.Context) (map[string]string, error) {
			var name, age, city, tz, xp string
			err := db.QueryRow(context.Background(),
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
		})

	cancelButton := &InlineButtonTemplate{
		"Cancel",
		"Отмена",
		func(c tele.Context) error {
			if err := fsm.ResetState(c); err != nil {
				fmt.Println(err)
			}
			if err := c.Send("OK", MainUserMenu); err != nil {
				fmt.Println(err)
			}
			return c.Respond()
		},
	}

	AccountSettingsMenu.AddButtons([]*InlineButtonTemplate{
		{
			"ChangeName",
			func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["name"]
				if !ok {
					return "Имя неизвестно", fmt.Errorf("can't fetch name")
				}
				return "Имя: " + s, nil
			},
			SettingsSGSetName,
		},
		{
			"ChangeAge",
			func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["age"]
				if !ok {
					return "Возраст неизвестен", fmt.Errorf("can't fetch age")
				}
				return "Возраст: " + s, nil
			},
			SettingsSGSetAge,
		},
		{
			"ChangeCity",
			func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["city"]
				if !ok {
					return "Город неизвестен", fmt.Errorf("can't fetch city")
				}
				return "Город: " + s, nil
			},
			SettingsSGSetCity,
		},
		{
			"ChangeTimezone",
			func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["timezone"]
				if !ok {
					return "Часовой пояс неизвестен", fmt.Errorf("can't fetch timezone")
				}
				return "Часовой пояс: " + s, nil
			},
			SettingsSGSetTimezone,
		},
		{
			"ChangeExperience",
			func(c tele.Context, dc map[string]string) (string, error) {
				s, ok := dc["experience"]
				if !ok {
					return "Опыт вокала неизвестен", fmt.Errorf("can't fetch experience")
				}
				return "Опыт вокала: " + s, nil
			},
			SettingsSGSetExperience,
		},
		cancelButton,
	})
	AccountSettingsMenu.Construct(bot, fsm, 1)

	WarmupNotificationsMenu = NewInlineMenu("Настройки напоминаний о распевках:",
		func(c tele.Context) (map[string]string, error) {
			var globalOn, monOn, tueOn, wedOn, thuOn, friOn, satOn, sunOn,
				monTime, tueTime, wedTime, thuTime, friTime, satTime, sunTime string
			err := db.QueryRow(context.Background(),
				`SELECT 
    					cast(global_on AS varchar(5)), 
    					cast(mon_on AS varchar(5)), 
    					cast(tue_on AS varchar(5)), 
    					cast(wed_on AS varchar(5)), 
    					cast(thu_on AS varchar(5)), 
    					cast(fri_on AS varchar(5)), 
    					cast(sat_on AS varchar(5)), 
    					cast(sun_on AS varchar(5)), 
       					to_char(mon_time,'HH24:MI'), 
       					to_char(tue_time,'HH24:MI'), 
       					to_char(wed_time,'HH24:MI'), 
       					to_char(thu_time,'HH24:MI'), 
       					to_char(fri_time,'HH24:MI'), 
       					to_char(sat_time,'HH24:MI'),
       					to_char(sun_time,'HH24:MI')
				 		FROM warmup_notifications WHERE user_id = $1`,
				c.Sender().ID).Scan(&globalOn, &monOn, &tueOn, &wedOn, &thuOn, &friOn, &satOn, &sunOn,
				&monTime, &tueTime, &wedTime, &thuTime, &friTime, &satTime, &sunTime)
			if err != nil {
				return nil, err
			}
			data := map[string]string{
				"globalOn": globalOn,

				"monOn": monOn,
				"tueOn": tueOn,
				"wedOn": wedOn,
				"thuOn": thuOn,
				"friOn": friOn,
				"satOn": satOn,
				"sunOn": sunOn,

				"monTime": monTime,
				"tueTime": tueTime,
				"wedTime": wedTime,
				"thuTime": thuTime,
				"friTime": friTime,
				"satTime": satTime,
				"sunTime": sunTime,
			}
			return data, nil
		})
	WarmupNotificationsMenu.AddButtons([]*InlineButtonTemplate{
		{
			"NotificationSwitchMon",
			func(c tele.Context, dc map[string]string) (string, error) {
				s := "Понедельник: "
				v, ok := dc["monOn"]
				if !ok {
					return s + "???", fmt.Errorf("can't fetch monOn")
				}
				if v == "true" {
					return s + "🔔", nil
				}
				return s + "🔕", nil
			},
			NoState,
		},
		{
			"NotificationTimeMon",
			func(c tele.Context, dc map[string]string) (string, error) {
				v, ok := dc["monTime"]
				if !ok {
					return "HH:MM", fmt.Errorf("can't fetch monTime")
				}
				return v, nil
			},
			NoState,
		},
		{
			"NotificationSwitchTue",
			func(c tele.Context, dc map[string]string) (string, error) {
				s := "Вторник: "
				v, ok := dc["tueOn"]
				if !ok {
					return s + "???", fmt.Errorf("can't fetch tueOn")
				}
				if v == "true" {
					return s + "🔔", nil
				}
				return s + "🔕", nil
			},
			NoState,
		},
		{
			"NotificationTimeTue",
			func(c tele.Context, dc map[string]string) (string, error) {
				v, ok := dc["tueTime"]
				if !ok {
					return "HH:MM", fmt.Errorf("can't fetch tueTime")
				}
				return v, nil
			},
			NoState,
		},
		{
			"NotificationSwitchWed",
			func(c tele.Context, dc map[string]string) (string, error) {
				s := "Среда: "
				v, ok := dc["wedOn"]
				if !ok {
					return s + "???", fmt.Errorf("can't fetch wedOn")
				}
				if v == "true" {
					return s + "🔔", nil
				}
				return s + "🔕", nil
			},
			NoState,
		},
		{
			"NotificationTimeWed",
			func(c tele.Context, dc map[string]string) (string, error) {
				v, ok := dc["wedTime"]
				if !ok {
					return "HH:MM", fmt.Errorf("can't fetch wedTime")
				}
				return v, nil
			},
			NoState,
		},
		{
			"NotificationSwitchThu",
			func(c tele.Context, dc map[string]string) (string, error) {
				s := "Четверг: "
				v, ok := dc["thuOn"]
				if !ok {
					return s + "???", fmt.Errorf("can't fetch thuOn")
				}
				if v == "true" {
					return s + "🔔", nil
				}
				return s + "🔕", nil
			},
			NoState,
		},
		{
			"NotificationTimeThu",
			func(c tele.Context, dc map[string]string) (string, error) {
				v, ok := dc["thuTime"]
				if !ok {
					return "HH:MM", fmt.Errorf("can't fetch thuTime")
				}
				return v, nil
			},
			NoState,
		},
		{
			"NotificationSwitchFri",
			func(c tele.Context, dc map[string]string) (string, error) {
				s := "Пятница: "
				v, ok := dc["friOn"]
				if !ok {
					return s + "???", fmt.Errorf("can't fetch friOn")
				}
				if v == "true" {
					return s + "🔔", nil
				}
				return s + "🔕", nil
			},
			NoState,
		},
		{
			"NotificationTimeFri",
			func(c tele.Context, dc map[string]string) (string, error) {
				v, ok := dc["friTime"]
				if !ok {
					return "HH:MM", fmt.Errorf("can't fetch friTime")
				}
				return v, nil
			},
			NoState,
		},
		{
			"NotificationSwitchSat",
			func(c tele.Context, dc map[string]string) (string, error) {
				s := "Суббота: "
				v, ok := dc["satOn"]
				if !ok {
					return s + "???", fmt.Errorf("can't fetch satOn")
				}
				if v == "true" {
					return s + "🔔", nil
				}
				return s + "🔕", nil
			},
			NoState,
		},
		{
			"NotificationTimeSat",
			func(c tele.Context, dc map[string]string) (string, error) {
				v, ok := dc["satTime"]
				if !ok {
					return "HH:MM", fmt.Errorf("can't fetch satTime")
				}
				return v, nil
			},
			NoState,
		},
		{
			"NotificationSwitchSun",
			func(c tele.Context, dc map[string]string) (string, error) {
				s := "Воскресенье: "
				v, ok := dc["sunOn"]
				if !ok {
					return s + "???", fmt.Errorf("can't fetch sunOn")
				}
				if v == "true" {
					return s + "🔔", nil
				}
				return s + "🔕", nil
			},
			NoState,
		},
		{
			"NotificationTimeSun",
			func(c tele.Context, dc map[string]string) (string, error) {
				v, ok := dc["sunTime"]
				if !ok {
					return "HH:MM", fmt.Errorf("can't fetch sunTime")
				}
				return v, nil
			},
			NoState,
		},
		{
			"GlobalNotificationSwitch",
			func(c tele.Context, dc map[string]string) (string, error) {
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
			NoState,
		},
		{RowSplitterButton, nil, nil},
		cancelButton,
	})
	WarmupNotificationsMenu.Construct(bot, fsm, 2)
}

func setupHandlers(bot *tele.Bot, fsm *FSM) {
	bot.Handle("/start", func(c tele.Context) error {
		userID := c.Sender().ID

		ok, err := UserIsInDatabase(userID)
		if err != nil {
			return fmt.Errorf("/start[%d]: %w", userID, err)
		}

		if ok {
			return c.Reply("Привет! Ты зарегистрирован в боте, тебе доступна его функциональность!", MainUserMenu)
		}

		return fsm.TriggerState(c, SurveySGStartSurveyReqName)
	})

	bot.Handle(tele.OnText, func(c tele.Context) error {
		userID := c.Sender().ID

		ok, err := UserIsInDatabase(userID)
		if err != nil {
			return fmt.Errorf("/OnText: %w", err)
		}
		if !ok {
			return c.Send("Сначала надо пройти опрос.")
		}

		ok, err = UserHasState(userID)
		if err != nil {
			return fmt.Errorf("/OnText: %w", err)
		}
		if ok {
			return fsm.UpdateState(c)
		}

		switch c.Text() {
		case "Распевки":
		case "Напоминания":
			return WarmupNotificationsMenu.Serve(c)
		case "Записаться на урок":
		case "Обо мне":
		case "Настройки аккаунта":
			return AccountSettingsMenu.Serve(c)
		}
		return nil
	})
}
