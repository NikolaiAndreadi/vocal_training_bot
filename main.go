package main

import (
	"fmt"
	"time"

	"vocal_training_bot/BotExt"
)

var notificationService *NotificationService

func main() {
	cfg := ParseConfig()

	DB = InitDbConnection(cfg)
	BotExt.SetDatabaseEntry(DB)
	RD = InitCacheConnection(cfg)
	notificationService = NewNotificationService(RD, 10*time.Second)

	userBot := InitBot(cfg)

	err := notificationService.RebuildQueue()
	if err != nil {
		fmt.Println(err.Error())
	}

	notificationService.Start()

	userBot.Start()
}
