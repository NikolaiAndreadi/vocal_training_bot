package main

import (
	"fmt"
	"time"
)

var notificationService *NotificationService

func main() {
	cfg := ParseConfig()

	DB = InitDbConnection(cfg)
	notificationService = NewNotificationService(cfg, 10*time.Second)
	notificationService.handler = func(userID int64) error {
		fmt.Println(userID)
		return nil
	}
	b := InitBot(cfg)
	err := notificationService.RebuildQueue()
	if err != nil {
		fmt.Println(err.Error())
	}
	notificationService.Start()

	b.Start()
}
