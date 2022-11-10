package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool
var RD *redis.Client

// InitDbConnection creates a Pool of connections to Postgres database. Panics on fail: without database
// there's nothing to do.
// TODO: add MaxConnections and other parameters here and to config structure
func InitDbConnection(cfg Config) *pgxpool.Pool {
	DSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s",
		cfg.Pg.Host, cfg.Pg.Port, cfg.Pg.User, cfg.Pg.Pass, cfg.Pg.DBName)

	pgCfg, err := pgxpool.ParseConfig(DSN)
	if err != nil {
		panic(fmt.Errorf("InitDbConnection: ParseConfig: %w", err))
	}

	db, err := pgxpool.NewWithConfig(context.Background(), pgCfg)
	if err != nil {
		panic(fmt.Errorf("InitDbConnection: NewWithConfig: %w", err))
	}

	createSchema(db)

	return db
}

// CreateSchema creates db schemas
func createSchema(conn *pgxpool.Pool) {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		user_id			int8		NOT NULL, -- 64 bit integer for chat_id / user_id
		username		varchar(50), -- user name
		age				int2  		CHECK (age > 0),
		city			varchar(50), -- city name
		timezone_raw	int4  		CHECK (timezone_raw BETWEEN -720 AND 840), -- shift from UTC in minutes
		timezone_txt	text		NOT NULL, -- text representation of timezone, from google maps API
		experience      VARCHAR(20) NOT NULL, -- experience of vocal training
		user_class		VARCHAR(7)  NOT NULL DEFAULT 'USER' CHECK (user_class IN ('USER', 'ADMIN', 'BANNED')), -- group for user
		join_dt			timestamp	NOT NULL, -- UTC timestamp of connection to the bot
		
		PRIMARY KEY (user_id)
	);

	CREATE TABLE IF NOT EXISTS states (
		user_id			int8		NOT NULL, -- 64 bit integer for chat_id / user_id
		state			text,
		message_id      int4,
		temp_vars		jsonb		NOT NULL DEFAULT '{}'::jsonb,
		
		PRIMARY KEY (user_id)
	);

	CREATE TABLE IF NOT EXISTS warmup_notifications (
		user_id		int8		REFERENCES users(user_id),
		
		day_of_week 	varchar(3)  NOT NULL CHECK (day_of_week IN ('sun','mon','tue','wed','thu','fri','sat')),
		trigger_switch	bool        NOT NULL DEFAULT true,
		trigger_time 	time(0) 	NOT NULL DEFAULT '18:00:00'
	);
	CREATE INDEX IF NOT EXISTS idx_warmup_notification_timings__user_id ON warmup_notifications(user_id);
	CREATE INDEX IF NOT EXISTS idx_warmup_notification_timings__switch ON warmup_notifications(trigger_switch);

	CREATE TABLE IF NOT EXISTS warmup_notification_global (
		user_id			int8	REFERENCES users(user_id),
		global_switch	bool	NOT NULL DEFAULT false
	);
	CREATE INDEX IF NOT EXISTS idx_warmup_notification_global__user_id ON warmup_notification_global(user_id);
	CREATE INDEX IF NOT EXISTS idx_warmup_notification_global__global_switch ON warmup_notification_global(global_switch);

	CREATE TABLE IF NOT EXISTS messages (
	    record_id uuid, 
	    
	    message_id text NOT NULL,
	    chat_id int8 NOT NULL,
	    album_id text,
	    
	    message_type varchar(10) NOT NULL,
	    message_text text,
	    entity_json text
	);
	CREATE INDEX IF NOT EXISTS idx_messages__record_id ON messages(record_id);

	CREATE TABLE IF NOT EXISTS warmup_cheerups (
		cheerup_id	serial	PRIMARY KEY,
		record_id	uuid	-- REFERENCES messages(record_id) MATCH SIMPLE
	);

	CREATE TABLE IF NOT EXISTS warmup_groups (
	    warmup_group	serial	PRIMARY KEY, 
	    group_name		text
	);
	CREATE TABLE IF NOT EXISTS warmups (
	    warmup_id		serial	PRIMARY KEY,
	    warmup_group	int		REFERENCES warmup_groups(warmup_group),
	    warmup_name		text,
	    price			int2	CHECK (price >= 0) DEFAULT 0,
	    record_id		uuid    -- REFERENCES messages(record_id) MATCH SIMPLE 
	);
	`

	if _, err := conn.Exec(context.Background(), schema); err != nil {
		panic(fmt.Errorf("createSchema: %w", err))

	}
}

func UserIsInDatabase(UserID int64) bool {
	var userID int64
	err := DB.QueryRow(context.Background(), "SELECT user_id from users where user_id = $1", UserID).Scan(&userID)
	if err == nil {
		return userID != 0
	}
	if err == pgx.ErrNoRows {
		return false
	}
	if err != nil {
		fmt.Println(fmt.Errorf("UserIsInDatabase[%d]: %w", UserID, err))
	}

	return false
}

func initUserDBs(userID int64) error {
	_, err := DB.Exec(context.Background(), `
	INSERT INTO states(user_id)
	VALUES ($1)
	ON CONFLICT DO NOTHING`, userID)
	if err != nil {
		return fmt.Errorf("initUserDBs: %w", err)
	}

	_, err = DB.Exec(context.Background(), `
	INSERT INTO warmup_notifications(user_id, day_of_week)
	VALUES 
	    ($1, 'sun'),
	    ($1, 'mon'),
	    ($1, 'tue'),
	    ($1, 'wed'),
	    ($1, 'thu'),
	    ($1, 'fri'),
	    ($1, 'sat')
	ON CONFLICT DO NOTHING`, userID)
	if err != nil {
		return fmt.Errorf("initUserDBs: %w", err)
	}

	_, err = DB.Exec(context.Background(), `
	INSERT INTO warmup_notification_global(user_id)
	VALUES ($1)
	ON CONFLICT DO NOTHING`, userID)
	if err != nil {
		return fmt.Errorf("initUserDBs: %w", err)
	}

	return nil
}

func getRandomCheerup() (recordID string, err error) {
	// TODO prevent repetition (with warmup_notification_global.last_cheerup_id)
	err = DB.QueryRow(context.Background(), `
		SELECT record_id from warmup_cheerups
		ORDER BY RANDOM()
		LIMIT 1`).Scan(&recordID)
	if err != nil {
		err = fmt.Errorf("selectRandomCheerup: %w", err)
	}
	return
}

func InitCacheConnection(cfg Config) *redis.Client {
	rd := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Host + ":" + cfg.Redis.Port,
		Password: cfg.Redis.Pass,
	})
	if err := rd.Ping().Err(); err != nil {
		panic(err)
	}
	return rd
}

type UserGroup string

const (
	UGUser    UserGroup = "USER"
	UGAdmin             = "ADMIN"
	UGBanned            = "BANNED"
	UGNewUser           = ""
)

func GetUserGroup(userID int64) (UserGroup, error) {
	strUserID := strconv.FormatInt(userID, 10)
	if ug, err := RD.Get(strUserID).Result(); err == nil {
		return UserGroup(ug), nil
	}
	// no data in cache - get from postgres
	var ug string
	err := DB.QueryRow(context.Background(), `
		SELECT user_class FROM users
		WHERE user_id = $1`, userID).Scan(&ug)
	if err == nil {
		if rdErr := RD.Set(strUserID, ug, 1*time.Hour).Err(); rdErr != nil {
			fmt.Println(fmt.Errorf("CheckUserGroup[%d]: can't cache user group", userID))
		}
		return UserGroup(ug), nil
	}
	if err == pgx.ErrNoRows {
		if rdErr := RD.Set(strUserID, UGNewUser, 1*time.Hour).Err(); rdErr != nil {
			fmt.Println(fmt.Errorf("CheckUserGroup[%d]: can't cache user group: %w", userID, rdErr))
		}
		return UGNewUser, nil
	}
	return UGNewUser, fmt.Errorf("CheckUserGroup[%d] can't fetch UserGroup from pg: %w", userID, err)
}

func SetUserGroup(userID int64, ug UserGroup) error {
	_, err := DB.Exec(context.Background(), `
				UPDATE users
				SET user_class= $1
				WHERE user_id = $2
				`, string(ug), userID)
	if err != nil {
		return fmt.Errorf("SetUserGroup[%d]: can't change UserGroup: %w", userID, err)
	}
	// update cache
	if rdErr := RD.Del(strconv.FormatInt(userID, 10)).Err(); rdErr != nil {
		fmt.Println(fmt.Errorf("SetUserGroup[%d]: can't invalidaate cache: %w", userID, rdErr))
	}
	return nil
}
