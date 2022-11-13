package main

import (
	"fmt"
	"time"

	"vocal_training_bot/BotExt"

	"go.uber.org/zap"
)

var notificationService *NotificationService
var logger *zap.Logger

func main() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(fmt.Errorf("main(): %w", err))
	}
	defer func() {
		err = logger.Sync()
		if err != nil {
			fmt.Printf("main(): can't sync logger: %s", err.Error())
		}
	}()

	cfg := ParseConfig()

	DB = InitDbConnection(cfg)
	BotExt.SetVars(DB, logger)
	RD = InitCacheConnection(cfg)
	notificationService = NewNotificationService(RD, 10*time.Second)

	userBot := InitBot(cfg)

	notificationService.Start()

	userBot.Start()
}
