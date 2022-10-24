package main

import (
	"fmt"
	tele "gopkg.in/telebot.v3"
)

type StatefulBot struct {
	*tele.Bot
	fsm FSM
}

func InitBot(cfg Config) *StatefulBot {
	teleCfg := tele.Settings{
		Token: cfg.Bot.Token,
	}
	bot, err := tele.NewBot(teleCfg)
	if err != nil {
		panic(fmt.Errorf("InitBot: %w", err))
	}

	fsm := NewFSM()
	statefulBot := &StatefulBot{
		bot,
		fsm,
	}

	setupStates(statefulBot)
	setupHandlers(statefulBot)

	return statefulBot
}

func setupStates(bot *StatefulBot) {
	bot.fsm.AddState("StartSurvey",
		"Привет! Чтобы зарегистрироваться в боте надо пройти опрос. Как я могу к тебе обращаться?",
		func(c tele.Context) (nextState string, err error) {
			err = SetStateVar(c, "UserName", c.Message().Text)
			return "SetAge", err
		})

	bot.fsm.AddState("SetAge",
		"Введите возраст",
		func(c tele.Context) (nextState string, err error) {
			err = SetStateVar(c, "Age", c.Message().Text)
			val, ok, err := GetStateVar(c, "UserName")
			fmt.Println(val)
			fmt.Println(ok)
			val, ok, err = GetStateVar(c, "UserNameLox")
			fmt.Println(val)
			fmt.Println(ok)
			err = ClearStateVars(c)
			return ResetState, err
		})
}

func setupHandlers(bot *StatefulBot) {
	bot.Handle("/start", func(c tele.Context) error {
		userID := c.Sender().ID
		if UserIsInDatabase(userID) {
			return c.Reply("Welcome back!")
		}
		return bot.fsm.TriggerState(c, "StartSurvey")
	})

	bot.Handle(tele.OnText, func(c tele.Context) error {
		return bot.fsm.UpdateState(c)
	})
}
