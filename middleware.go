package main

import (
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

type RestrictConfig struct {
	UserType UserGroup
	In       tele.HandlerFunc
	Out      tele.HandlerFunc
}

// Restrict returns a middleware that handles a list of provided
// chats with the logic defined by In and Out functions.
// If the chat is found in the Chats field, In function will be called,
// otherwise Out function will be called.
func Restrict(v RestrictConfig) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		if v.In == nil {
			v.In = next
		}
		if v.Out == nil {
			v.Out = next
		}
		return func(c tele.Context) error {
			if ug, _ := GetUserGroup(c.Sender().ID); ug == v.UserType {
				return v.In(c)
			}
			return v.Out(c)
		}
	}
}

// Blacklist returns a middleware that skips the update for users
// specified in the chats field.
func Blacklist(userType UserGroup) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return Restrict(RestrictConfig{
			UserType: userType,
			Out:      next,
			In:       func(c tele.Context) error { return nil },
		})(next)
	}
}

// Whitelist returns a middleware that skips the update for users
// NOT specified in the chats field.
func Whitelist(userType UserGroup) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return Restrict(RestrictConfig{
			UserType: userType,
			In:       next,
			Out:      func(c tele.Context) error { return nil },
		})(next)
	}
}

func MiddlewareLogger(logger *zap.Logger) tele.MiddlewareFunc {
	if logger == nil {
		panic("MiddlewareLogger: logger is nil")
	}

	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			var msgType string
			if c.Message() != nil {
				m := c.Message().Media()
				if m != nil {
					msgType = m.MediaType()
				}
			}
			logger.Info(c.Text(),
				zap.Int64("user", c.Sender().ID),
				zap.String("data", c.Data()),
				zap.String("type", msgType),
			)

			err := next(c)

			if err != nil {
				logger.Error("",
					zap.Int64("user", c.Sender().ID),
					zap.Error(err),
				)
			}
			return err
		}
	}
}

func MiddlewareMetrics() tele.MiddlewareFunc {
	handlerLatency := prom.NewSummaryVec(prom.SummaryOpts{
		Name:       "handler_latency",
		Help:       "latency of handler for route, state, menu and for success/fail",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"route", "state", "menu", "status"})
	prom.MustRegister(handlerLatency)

	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			start := time.Now()
			err := next(c)
			duration := time.Now().Sub(start).Seconds()

			route, ok := c.Get("route").(string)
			if !ok {
				route = "NoRoute"
			}
			state, ok := c.Get("state").(string)
			if !ok {
				state = "NoState"
			}
			menu, ok := c.Get("menu").(string)
			if !ok {
				menu = "NoMenu"
			}
			status := "ok"
			if err != nil {
				status = "fail"
			}
			handlerLatency.WithLabelValues(route, state, menu, status).Observe(duration)

			return err
		}
	}
}
