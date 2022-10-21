package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

	return db
}

func CreateSchema(conn *pgxpool.Pool) {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id				int8		NOT NULL, -- 64 bit integer for chat_id / user_id
		username		varchar(50), -- user name TODO extract from user data
		age				int2  		CHECK (age > 0),
		city			varchar(50), -- city name
		timezone_raw	int4  		CHECK (timezone_raw BETWEEN -43200 AND 50400), -- shift from UTC in seconds
		timezone_txt	text		NOT NULL, -- text representation of timezone, from google maps API
		user_class		VARCHAR(7)  NOT NULL DEFAULT 'USER' CHECK (user_class IN ('USER', 'ADMIN', 'BANNED')), -- group for user
		join_dt			timestamp	NOT NULL, -- UTC timestamp of connection to the bot
		
		PRIMARY KEY (id)
	);

	CREATE TABLE IF NOT EXISTS warmup_global_notifications (
	    id		int8	REFERENCES users(id),
	    online	bool	NOT NULL
	);

	CREATE TABLE IF NOT EXISTS warmup_notification_timings (
		id			int8		REFERENCES users(id),
		online		bool		NOT NULL,
		timing_day	char(3)		CHECK (timing_day IN ('MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT', 'SUN')),
		timing_dt	timestamp	-- NO timezone, - calculate by using users.timezone_raw
	);

	CREATE TABLE IF NOT EXISTS warmup_cheerups (
		cheerup_id	serial	NOT NULL,
		cheerup_txt	text	NOT NULL,
		online		bool,
		
		PRIMARY KEY (cheerup_id)
	);

	CREATE TABLE IF NOT EXISTS warmup_log (
	    id			int8		REFERENCES users(id),
	    exec_dt		timestamp,	-- timezone from users.timezone_raw
	    duration	interval,	-- None -> not committed
	    cheerup_id	int4 		REFERENCES warmup_cheerups(cheerup_id)
	);

	CREATE TABLE IF NOT EXISTS become_student_requests (
		id			int8		REFERENCES users(id),
	    datetime    timestamp
	);

	CREATE TABLE IF NOT EXISTS blog_messages (
	    message_id	serial,
	    datetime    timestamp	NOT NULL,
	    user_class	VARCHAR(7)  NOT NULL DEFAULT 'ALL' CHECK (user_class IN ('ALL', 'USER', 'ADMIN', 'BANNED')), -- group for user
		posted		bool,
	    
	    PRIMARY KEY (message_id)
	); -- TODO views count!!!
	
	CREATE TABLE IF NOT EXISTS texts (
	    description text NOT NULL,
	    content     text NOT NULL
	);
	`

	if _, err := conn.Exec(context.Background(), schema); err != nil {
		panic(err)
	}
}
