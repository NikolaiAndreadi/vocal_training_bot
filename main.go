package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
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
	logCore := buildLogger()

	logger = zap.New(logCore, zap.AddStacktrace(zap.WarnLevel))
	var err error
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
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(writer, "OK")
	})
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

func buildLogger() zapcore.Core {
	logSync := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "./log.log",
		MaxSize:    500, // mb
		MaxBackups: 3,
		MaxAge:     14, // days
		Compress:   true,
	})

	encoder := zap.NewProductionEncoderConfig()
	encoder.EncodeTime = zapcore.ISO8601TimeEncoder

	fileEncoder := zapcore.NewJSONEncoder(encoder)
	consoleEncoder := zapcore.NewConsoleEncoder(encoder)

	logCore := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, logSync, zap.InfoLevel),
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zap.DebugLevel),
	)

	return logCore
}
