package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/jackc/pgx/v5/pgconn"
)

const delayedNotificationList = "delayedNotificationList"

type NotificationService struct {
	rd        *redis.Client
	frequency time.Duration
	handler   func(int64) error // job function
	quit      chan struct{}
}

func NewNotificationService(rd *redis.Client, frequency time.Duration) *NotificationService {
	ns := &NotificationService{}

	ns.rd = rd
	ns.frequency = frequency
	return ns
}

func (ns *NotificationService) Start() {
	if ns.handler == nil {
		panic(fmt.Errorf("NotificationService.Start: handler is not set"))
	}
	ticker := time.NewTicker(ns.frequency)
	ns.quit = make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				err := ns.processUsers()
				if err != nil {
					fmt.Println(fmt.Errorf("NotificationService.Run: %w", err))
				}
			case <-ns.quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func (ns *NotificationService) Stop() {
	close(ns.quit)
}

func (ns *NotificationService) AddUser(userID int64) error {
	ts, err := getNearestNotificationFromPg(userID)
	if err != nil {
		return fmt.Errorf("NotificationService.AddUser: %w", err)
	}
	err = ns.addUser(userID, ts)
	if err != nil {
		return fmt.Errorf("NotificationService.AddUser: %w", err)
	}
	return nil
}

// add user only if it does not exist or if new score is less than existing one
func (ns *NotificationService) addUser(userID int64, timestamp int64) error {
	req := ns.rd.ZScore(delayedNotificationList, strconv.FormatInt(userID, 10))
	currentTimestamp := int64(req.Val())
	if timestamp == 0 {
		return nil
	}
	if (timestamp > currentTimestamp) && currentTimestamp != 0 {
		return nil
	}
	_, err := ns.rd.ZAdd(delayedNotificationList, redis.Z{Member: userID, Score: float64(timestamp)}).Result()
	if err != nil {
		return fmt.Errorf("NotificationService.addUserToQueue[%d]: %w", userID, err)
	}
	return nil
}

func (ns *NotificationService) DelUser(userID int64) error {
	err := ns.rd.ZRem(delayedNotificationList, strconv.FormatInt(userID, 10)).Err()
	if err != nil {
		return fmt.Errorf("NotificationService.delUser[%d]: %w", userID, err)
	}
	return nil
}

func (ns *NotificationService) processUsers() error {
	now := time.Now().Unix()
	users, err := ns.rd.ZRevRangeByScore(delayedNotificationList,
		redis.ZRangeBy{
			Min: "1",
			Max: strconv.FormatInt(now, 10),
		}).Result()
	if err != nil {
		return fmt.Errorf("NotificationService.processUsers: %w", err)
	}
	if len(users) == 0 {
		return nil
	}

	var updateList []int64
	for _, user := range users {
		userID, err := strconv.ParseInt(user, 10, 64)
		if err != nil {
			fmt.Println(fmt.Errorf("NotificationService.processUsers ParseInt: %s lead to error %w", user, err))
			continue
		}
		err = ns.handler(userID)
		if err != nil {
			fmt.Println(fmt.Errorf("NotificationService.processUsers handler[%d]: %w", userID, err))
			continue
		}
		err = ns.DelUser(userID)
		if err != nil {
			fmt.Println(fmt.Errorf("NotificationService.processUsers delUser[%d]: %w", userID, err))
			continue
		}
		updateList = append(updateList, userID)
	}

	currentNotifications, err := getNearestNotificationsFromPg()
	if err != nil {
		fmt.Println(fmt.Errorf("NotificationService.processUsers get currentNotifications: %w", err))
	}
	for _, updateUserID := range updateList {
		timestamp, ok := currentNotifications[updateUserID]
		if !ok {
			fmt.Println(fmt.Errorf("NotificationService.processUsers no UserID %d in map", updateUserID))
			continue
		}
		err = ns.addUser(updateUserID, timestamp)
		if err != nil {
			fmt.Println(fmt.Errorf("NotificationService.processUsers addUser[%d]: %w", updateUserID, err))
		}
	}

	return nil
}

func (ns *NotificationService) RebuildQueue() error {
	err := ns.purge()
	if err != nil {
		return fmt.Errorf("NotificationService.RebuildQueue: %w", err)
	}
	currentNotifications, err := getNearestNotificationsFromPg()
	if err != nil {
		return fmt.Errorf("NotificationService.RebuildQueue get currentNotifications: %w", err)
	}

	for key, val := range currentNotifications {
		err = ns.addUser(key, val)
		if err != nil {
			fmt.Println(fmt.Errorf("NotificationService.RebuildQueue: %w", err))
		}
	}
	return nil
}

func (ns *NotificationService) purge() error {
	err := ns.rd.Del(delayedNotificationList).Err()
	if err != nil {
		return fmt.Errorf("NotificationService.purge: %w", err)
	}
	return nil
}

func getNearestNotificationFromPg(userID int64) (timestamp int64, err error) {
	err = DB.QueryRow(context.Background(), `
		SELECT
			MIN(EXTRACT(EPOCH FROM nearest_notification) :: INT) AS nearest_notification_ts
		FROM
			(SELECT user_dt :: DATE + trigger_time + INTERVAL '1 day' * (
				CASE
					WHEN (user_dow_int = extract(dow FROM user_dt)) AND (user_dt :: TIME < trigger_time) THEN 0
					ELSE
						CASE
							WHEN user_dow_int > extract(dow FROM user_dt) THEN user_dow_int - extract(dow FROM user_dt)
							ELSE 7 - extract(dow FROM user_dt) + user_dow_int
						END
				END
			) - timezone_raw * INTERVAL '1 minute' AS nearest_notification -- convert to UTC Time

			FROM (
				SELECT
					user_id,
					trigger_time,
					user_dt,
					timezone_raw,
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
					FROM warmup_notification_global
					WHERE global_switch = TRUE
				) notifications USING (user_id)
				INNER JOIN(
					SELECT
						user_id,
						timezone_raw,
						now() AT TIME ZONE 'UTC' + (timezone_raw * INTERVAL '1 minute') AS user_dt
					FROM users
				) user_time USING (user_id)
				WHERE trigger_switch = TRUE
			) warmup_table_query
			WHERE user_id = $1
		) query`, userID).Scan(&timestamp)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return timestamp, err
	}
	return timestamp, nil
}

type notificationQuery map[int64]int64 // key - userID, value - timestamp

func getNearestNotificationsFromPg() (results notificationQuery, err error) {
	results = make(notificationQuery)
	rows, err := DB.Query(context.Background(), `
	SELECT user_id, nearest_notification_ts FROM
		(SELECT 
			user_id,
			EXTRACT(EPOCH FROM nearest_notification) AS nearest_notification_ts,
			row_number() OVER (PARTITION BY user_id ORDER BY nearest_notification) priority
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
				) - timezone_raw * INTERVAL '1 minute' AS nearest_notification -- convert to UTC Time
			FROM
				(SELECT 
					user_id,
					trigger_time,
					user_dt,
					timezone_raw,
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
					FROM warmup_notification_global
					WHERE global_switch = TRUE
				) notifications USING (user_id)
				INNER JOIN(
					SELECT 
						user_id,
						timezone_raw,
						now() AT TIME ZONE 'UTC' + (timezone_raw * INTERVAL '1 minute') AS user_dt
					FROM users
				) user_time USING (user_id)
				WHERE trigger_switch = TRUE 
				) warmup_table_query
			) time_convert_query
		) window_query
	WHERE priority = 1`)
	defer rows.Close()
	if err != nil {
		return results, fmt.Errorf("getNearestNotificationsFromPg: %w", err)
	}

	var userID, timestamp int64
	for rows.Next() {
		err := rows.Scan(&userID, &timestamp)
		if err != nil {
			return results, fmt.Errorf("getNearestNotificationsFromPg: scan row: %w", err)
		}
		results[userID] = timestamp
	}
	if err := rows.Err(); err != nil {
		return results, fmt.Errorf("getNearestNotificationsFromPg: postgres itetator %w", err)
	}
	return
}
