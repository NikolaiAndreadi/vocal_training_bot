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
	DSN := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
		cfg.Pg.Host, cfg.Pg.Port, cfg.Pg.User, cfg.Pg.Pass, cfg.Pg.DBName)

	pgCfg, err := pgxpool.ParseConfig(DSN)
	if err != nil {
		panic(err)
	}

	db, err := pgxpool.NewWithConfig(context.Background(), pgCfg)
	if err != nil {
		panic(err)
	}

	createSchema(db)

	return db
}

// CreateSchema creates db schemas
func createSchema(conn *pgxpool.Pool) {
	schema := `
	DROP TABLE IF EXISTS users CASCADE;
	CREATE TABLE IF NOT EXISTS users (
		user_id			int8		NOT NULL, -- 64 bit integer for chat_id / user_id
		username		varchar(50), -- user name TODO extract from user data
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
		temp_vars		jsonb		NOT NULL DEFAULT '{}'::jsonb,
		
		PRIMARY KEY (user_id)
	);

	DROP TABLE IF EXISTS warmup_global_notifications CASCADE;
	CREATE TABLE IF NOT EXISTS warmup_global_notifications (
	    user_id	int8	REFERENCES users(user_id),
	    online	bool	NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_warmup_global_notifications__user_id ON warmup_global_notifications(user_id);

	DROP TABLE IF EXISTS warmup_notification_timings CASCADE;
	CREATE TABLE IF NOT EXISTS warmup_notification_timings (
		user_id		int8		REFERENCES users(user_id),
		online		bool		NOT NULL,
		timing_day	char(3)		CHECK (timing_day IN ('MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT', 'SUN')),
		timing_dt	timestamp	-- NO timezone, - calculate by using users.timezone_raw
	);
	CREATE INDEX IF NOT EXISTS idx_warmup_notification_timings__user_id ON warmup_notification_timings(user_id);

	DROP TABLE IF EXISTS warmup_cheerups CASCADE;
	CREATE TABLE IF NOT EXISTS warmup_cheerups (
		cheerup_id	serial	NOT NULL,
		cheerup_txt	text	NOT NULL,
		online		bool,
		
		PRIMARY KEY (cheerup_id)
	);

	DROP TABLE IF EXISTS warmup_log CASCADE;
	CREATE TABLE IF NOT EXISTS warmup_log (
	    user_id		int8		REFERENCES users(user_id),
	    exec_dt		timestamp,	-- timezone from users.timezone_raw
	    duration	interval,	-- None -> not committed
	    cheerup_id	int4 		REFERENCES warmup_cheerups(cheerup_id)
	);
	CREATE INDEX IF NOT EXISTS idx_warmup_log__user_id ON warmup_log(user_id);

	DROP TABLE IF EXISTS become_student_requests CASCADE;
	CREATE TABLE IF NOT EXISTS become_student_requests (
		user_id		int8		REFERENCES users(user_id),
	    datetime    timestamp,
	    resolved    bool
	);
	CREATE INDEX IF NOT EXISTS idx_become_student_requests__resolved ON become_student_requests(resolved);

	DROP TABLE IF EXISTS blog_messages CASCADE;
	CREATE TABLE IF NOT EXISTS blog_messages (
	    message_id	serial,
	    datetime    timestamp	NOT NULL,
	    user_class	VARCHAR(7)  NOT NULL DEFAULT 'ALL' CHECK (user_class IN ('ALL', 'USER', 'ADMIN', 'BANNED')), -- group for user
		posted		bool,
	    
	    PRIMARY KEY (message_id)
	); -- TODO views count!!!
	CREATE INDEX IF NOT EXISTS idx_blog_messages__posted ON blog_messages(posted);
	
	DROP TABLE IF EXISTS texts CASCADE;
	CREATE TABLE IF NOT EXISTS texts (
	    name		text NOT NULL,
	    description text NOT NULL,
	    content     text NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_texts__name ON texts(name);
	`

	if _, err := conn.Exec(context.Background(), schema); err != nil {
		panic(err)
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
	panic(fmt.Errorf("UserIsInDatabase: %w", err))
}
