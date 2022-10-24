package main

import (
	"fmt"
	tele "gopkg.in/telebot.v3"
)

type StatefulBot struct {
	*tele.Bot
	states FSM
}

func InitBot(cfg Config) *StatefulBot {
	teleCfg := tele.Settings{
		Token: cfg.Bot.Token,
	}
	bot, err := tele.NewBot(teleCfg)
	if err != nil {
		panic(fmt.Errorf("InitBot: %w", err))
	}

	states := NewFSM()
	statefulBot := &StatefulBot{
		bot,
		states,
	}

	setupStates(statefulBot)
	setupHandlers(statefulBot)

	return statefulBot
}

func setupStates(bot *StatefulBot) {
	bot.states.AddState("StartSurvey",
		"Привет! Чтобы зарегистрироваться в боте надо пройти опрос. Как я могу к тебе обращаться?",
		func(c tele.Context) (nextState string, e error) {
			if c.Message().Text == "1" {
				return ResumeState, c.Reply("ENTER YOUR NAMEEE!!!")
			}
			fmt.Println(c.Message().Text)
			return ResetState, nil
		})
}

func setupHandlers(bot *StatefulBot) {
	bot.Handle("/start", func(c tele.Context) error {
		userID := c.Sender().ID
		if UserIsInDatabase(userID) {
			return c.Reply("Welcome back!")
		}
		return bot.states.TriggerState(c, "StartSurvey")
	})

	bot.Handle(tele.OnText, func(c tele.Context) error {
		return bot.states.UpdateState(c)
	})
}
