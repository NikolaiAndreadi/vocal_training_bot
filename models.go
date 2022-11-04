package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

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

	DROP TABLE IF EXISTS states CASCADE;
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
	
	CREATE TABLE IF NOT EXISTS warmup_global_switch (
		user_id			int8	REFERENCES users(user_id),
		global_switch	bool	NOT NULL DEFAULT false
	);
	CREATE INDEX IF NOT EXISTS idx_warmup_global_switch__user_id ON warmup_global_switch(user_id);
	CREATE INDEX IF NOT EXISTS idx_warmup_global_switch__global_switch ON warmup_global_switch(global_switch);

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
	INSERT INTO warmup_global_switch(user_id)
	VALUES ($1)
	ON CONFLICT DO NOTHING`, userID)
	if err != nil {
		return fmt.Errorf("initUserDBs: %w", err)
	}

	return nil
}
