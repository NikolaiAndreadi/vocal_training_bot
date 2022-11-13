package main

import (
	"fmt"
	"net/http"
	"time"

	"vocal_training_bot/BotExt"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var notificationService *NotificationService
var logger *zap.Logger

func main() {
	var err error
	logSync := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "./log.log",
		MaxSize:    500, // mb
		MaxBackups: 3,
		MaxAge:     14, // days
		Compress:   true,
	})
	logCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		logSync,
		zap.InfoLevel,
	)
	logger = zap.New(logCore)
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

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		for {
			err := http.ListenAndServe(":2112", nil)
			if err != nil {
				logger.Error("metric server", zap.Error(err))
			}
		}
	}()

	userBot := InitBot(cfg)
	notificationService.Start()
	userBot.Start()
}
