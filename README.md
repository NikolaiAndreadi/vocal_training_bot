# vocal_training_bot
telegram bot for @vershkovaaa

bot API:  

    /start - starts survey +  
    ask for: Name, Age, City, Timezone, Experience (Newbie, < 1 year, 1-2 years, 2-3 years, 3-5 years, > 5 years)
    then: add to **users** table
    ! No functions are available until survey completion

    -------
    USER API
    / account_settings + 
        / change_name +
        / change_age +
        / change_city +
        / change_timezone +
        / change_experience +
        / show current +

    / warmup_notification_settings +    
        / notification_switch (on/off) +
        / notification_days (7 digit mask) +
        / change_time_for_day(DAY) in HH:MM format +
        / show_schedule +

    / warmups
        / fetch_free_warmup
        / warmup_catalog
        / buy_warmup
        / show_my_warmups

    / about_me       -> about_me text
    / become_student -> sends message "write to DM in instagram"


    ADMIN API
    / send fanout_message (ALL, USERS, STUDENTS)
    
    / set_about_me_text
    / show_about_me_text
    
    / set_become_student_text
    / show_become_student_text

    / add_warmup_cheerup 
    / switch_cheerup
    / correct_cheerup

    / set_user_group

    / add_warmup
    / update_warmup

Database Tables:

    +Users:
        id (chat_id?) Int64
        Name Varchar(100)
        age int8 > 0
        city Varchar(50)
        timezone_raw int32  BETWEEN (-43200, 50400) -- shift in seconds
        timezone text + google.maps.api
        user_class CHAR(7) IS IN ("USER", "ADMIN", "STUDENT", "BANNED")
        join_date  Timestamptz

    -- timezone generates from view "select Name from pg_timezone_names where Name not like 'posix%' and Name not ilike 'system%' order by Name;"
    -- https://stackoverflow.com/questions/55901/web-service-current-time-zone-for-a-city

    +WarmupGlobalNotifications:
        user_id REFERENCES Users(id)
        online  Bool (True/False)

    +WarmupNotificationsTimings:
        user_id REFERENCES Users(id)
        online  Bool (True/False)
        timing_day CHAR(3) IN (MON, TUE, WED, THU, FRI, SAT, SUN)
        timing_ts  timestamp(NO timezone) - calculate on Users.timezone param

    +WarmupLog:
        user_id REFERENCES Users(id)
        datetime timestamp (timezone from users.timezone)
        duration interval -- None -> not commited
        cheerup_id REFERENCES WarmupCheerups(cheerup_id)

    +BecomeStudentRequests:
        user_id REFERENCES Users(id)
        datetime timestamp # when pushed

    +WarmupCheerups:
        cheerup_id 
        text Text
        enabled Bool
    

    +BlogMessages:
        message_id
        datetime
        user_class
        views_count
        posted bool

    +Texts:
        descr
        content
    (EXAMPLES)
    AboutMeText:
        text Text

    MissedWarmupText:
        text Text

SCRATCHPAD:
- DEFINITELY dump database weekly
- LOGIC TO NOT UTILIZE changes of notification to go to the top of the leaderboard
- If missed warmup previously then send MissedWarmupText
- On correct cheerup don't change message to drastically, as it helps to understand which is efficient and which is not
- what is warmup?
- add healhchecks, grafana, prometheus etc...

TODO:
REFACTOR error handling in states:
extract error handling. if error - write SOMETHING WENT WRONG and reset, log error text
auto update menu that triggered event

    BUG: something with FSM vars implementation, resets when not necessary

    FEATURE: after menu is built, but not constructed, we should raise an error to prevent corrupted menu rendering



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
	);
	CREATE INDEX IF NOT EXISTS idx_blog_messages__posted ON blog_messages(posted);
	
	DROP TABLE IF EXISTS texts CASCADE;
	CREATE TABLE IF NOT EXISTS texts (
	    name		text NOT NULL,
	    description text NOT NULL,
	    content     text NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_texts__name ON texts(name);
