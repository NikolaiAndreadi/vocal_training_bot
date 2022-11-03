package main

import (
	"github.com/go-redis/redis"
)

type notificationService struct {
	rd *redis.Client
}

func NewNotificationService(cfg Config) *notificationService {
	ns := &notificationService{}
	ns.rd = redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Host + ":" + cfg.Redis.Port,
		Password: cfg.Redis.Pass,
	})
	return ns
}
