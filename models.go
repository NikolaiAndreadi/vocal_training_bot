package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
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

// CreateSchema creates db schemas
func CreateSchema(conn *pgxpool.Pool) {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		user_id			int8		NOT NULL, -- 64 bit integer for chat_id / user_id
		username		varchar(50), -- user name TODO extract from user data
		age				int2  		CHECK (age > 0),
		city			varchar(50), -- city name
		timezone_raw	int4  		CHECK (timezone_raw BETWEEN -43200 AND 50400), -- shift from UTC in seconds
		timezone_txt	text		NOT NULL, -- text representation of timezone, from google maps API
		user_class		VARCHAR(7)  NOT NULL DEFAULT 'USER' CHECK (user_class IN ('USER', 'ADMIN', 'BANNED')), -- group for user
		join_dt			timestamp	NOT NULL, -- UTC timestamp of connection to the bot
		
		PRIMARY KEY (user_id)
	);

	CREATE TABLE IF NOT EXISTS warmup_global_notifications (
	    user_id	int8	REFERENCES users(user_id),
	    online	bool	NOT NULL
	);
	CREATE INDEX idx_warmup_global_notifications__user_id ON warmup_global_notifications(user_id);

	CREATE TABLE IF NOT EXISTS warmup_notification_timings (
		user_id		int8		REFERENCES users(user_id),
		online		bool		NOT NULL,
		timing_day	char(3)		CHECK (timing_day IN ('MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT', 'SUN')),
		timing_dt	timestamp	-- NO timezone, - calculate by using users.timezone_raw
	);
	CREATE INDEX idx_warmup_notification_timings__user_id ON warmup_notification_timings(user_id);


	CREATE TABLE IF NOT EXISTS warmup_cheerups (
		cheerup_id	serial	NOT NULL,
		cheerup_txt	text	NOT NULL,
		online		bool,
		
		PRIMARY KEY (cheerup_id)
	);

	CREATE TABLE IF NOT EXISTS warmup_log (
	    user_id		int8		REFERENCES users(user_id),
	    exec_dt		timestamp,	-- timezone from users.timezone_raw
	    duration	interval,	-- None -> not committed
	    cheerup_id	int4 		REFERENCES warmup_cheerups(cheerup_id)
	);
	CREATE INDEX idx_warmup_log__user_id ON warmup_log(user_id);


	CREATE TABLE IF NOT EXISTS become_student_requests (
		user_id		int8		REFERENCES users(user_id),
	    datetime    timestamp,
	    resolved    bool
	);
	CREATE INDEX idx_become_student_requests__resolved ON become_student_requests(resolved);

	CREATE TABLE IF NOT EXISTS blog_messages (
	    message_id	serial,
	    datetime    timestamp	NOT NULL,
	    user_class	VARCHAR(7)  NOT NULL DEFAULT 'ALL' CHECK (user_class IN ('ALL', 'USER', 'ADMIN', 'BANNED')), -- group for user
		posted		bool,
	    
	    PRIMARY KEY (message_id)
	); -- TODO views count!!!
	CREATE INDEX idx_blog_messages__posted ON blog_messages(posted);
	
	CREATE TABLE IF NOT EXISTS texts (
	    description text NOT NULL,
	    content     text NOT NULL
	);
	`

	if _, err := conn.Exec(context.Background(), schema); err != nil {
		panic(err)
	}
}

type Users struct {
	UserID      int64     `db:"user_id"`
	Username    string    `db:"username"`
	Age         int16     `db:"age"`
	City        string    `db:"city"`
	TimezoneRaw int32     `db:"timezone_raw"`
	TimezoneTxt string    `db:"timezone_txt"`
	UserClass   string    `db:"user_class"`
	JoinDt      time.Time `db:"join_dt"`
}

type WarmupGlobalNotifications struct {
	UserID int64 `db:"user_id"`
	Online bool  `db:"online"`
}

type WarmupNotificationTimings struct {
	UserID    int64     `db:"user_id"`
	Online    bool      `db:"online"`
	TimingDay string    `db:"timing_day"`
	TimingDt  time.Time `db:"timing_dt"`
}

type WarmupCheerups struct {
	CheerupID  int64  `db:"cheerup_id"`
	CheerupTxt string `db:"cheerup_txt"`
	Online     bool   `db:"online"`
}

type WarmupLog struct {
	UserID    int64         `db:"user_id"`
	ExecDt    time.Time     `db:"exec_dt"`
	Duration  time.Duration `db:"duration"`
	CheerupID int64         `db:"cheerup_id"`
}

type BecomeStudentRequests struct {
	UserID   int64     `db:"user_id"`
	Datetime time.Time `db:"datetime"`
	Resolved bool      `db:"resolved"`
}

type BlogMessages struct {
	MessageID int32     `db:"message_id"`
	Datetime  time.Time `db:"datetime"`
	UserClass string    `db:"user_class"`
	Posted    bool      `db:"posted"`
}

type Texts struct {
	Description string `db:"description"`
	Content     string `db:"content"`
}
