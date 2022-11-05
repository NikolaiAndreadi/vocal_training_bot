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

	b := InitBot(cfg)
	err := notificationService.RebuildQueue()
	if err != nil {
		fmt.Println(err.Error())
	}

	notificationService.Start()
	b.Start()
}
