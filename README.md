# vocal_training_bot
telegram bot for @vershkovaaa

bot API:  

    /start - starts survey  DONE
    ask for: Name, Age, City, Timezone, Experience (Newbie, < 1 year, 1-2 years, 2-3 years, 3-5 years, > 5 years)
    then: add to **users** table
    ! No functions are available until survey completion

    -------
    USER API
    / account_settings
        / change_name
        / change_age
        / change_city
        / change_timezone
        / change_experience
        / show current

    / warmup_notification_settings    
        / notification_switch (on/off)
        / notification_days (7 digit mask)
        / change_time_for_day(DAY) in HH:MM format
        / show_schedule
        
        / start_warmup
        / warmup_in_progress
        / end_warmup
        / reject_warmup

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
        name Varchar(100)
        age int8 > 0
        city Varchar(50)
        timezone_raw int32  BETWEEN (-43200, 50400) -- shift in seconds
        timezone text + google.maps.api
        user_class CHAR(7) IS IN ("USER", "ADMIN", "STUDENT", "BANNED")
        join_date  Timestamptz

    -- timezone generates from view "select name from pg_timezone_names where name not like 'posix%' and name not ilike 'system%' order by name;"
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
    - If missed warmup previously then send  MissedWarmupText
    - On correct cheerup don't change message to drastically, as it helps to understand which is efficient and which is not
    - what is warmup?
    - add healhchecks, grafana, prometheus etc...