package main

import (
	"context"

	"github.com/go-redis/redis"
)

type NotificationService struct {
	rd *redis.Client
}

func NewNotificationService(cfg Config) *NotificationService {
	ns := &NotificationService{}
	rd := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Host + ":" + cfg.Redis.Port,
		Password: cfg.Redis.Pass,
	})
	if err := rd.Ping(); err != nil {
		panic(err)
	}

	ns.rd = rd
	return ns
}

func GetNearestWarmupNotifications() error {
	_, err := DB.Query(context.Background(), `
	SELECT user_id, notification_time FROM
		(SELECT 
			user_id,
			notification_time,
			row_number() OVER (PARTITION BY user_id ORDER BY notification_time) priority
		FROM
			(SELECT 
				user_id,
				user_dt :: DATE + trigger_time + INTERVAL '1 day' * (
				CASE
					WHEN (user_dow_int = extract(dow FROM user_dt)) AND (user_dt :: TIME < trigger_time) THEN 0
					ELSE 
						CASE
							WHEN user_dow_int > extract(dow FROM user_dt) THEN user_dow_int - extract(dow FROM user_dt)
							ELSE 7 - extract(dow FROM user_dt) + user_dow_int
						END
				END
				) AS notification_time
			FROM
				(SELECT 
					user_id,
					trigger_time,
					user_dt,
					CASE day_of_week
						WHEN 'sun' THEN 0
						WHEN 'mon' THEN 1
						WHEN 'tue' THEN 2
						WHEN 'wed' THEN 3
						WHEN 'thu' THEN 4
						WHEN 'fri' THEN 5
						WHEN 'sat' THEN 6
					END AS user_dow_int
				FROM warmup_notifications
				INNER JOIN(
					SELECT user_id
					FROM warmup_global_switch
					WHERE global_switch = TRUE
				) notifications USING (user_id)
				INNER JOIN(
					SELECT 
						user_id,
						now() AT TIME ZONE 'UTC' + (timezone_raw * INTERVAL '1 minute') AS user_dt
					FROM users
				) user_time USING (user_id)
				WHERE trigger_switch = TRUE 
				) warmup_table_query
			) time_convert_query
		) window_query
	WHERE priority = 1`)

	return err
}
