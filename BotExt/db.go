package BotExt

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

const NoState = ""

///////////////////////////////////////////////// STATE FUNCTIONS

func SetDatabaseEntry(db *pgxpool.Pool) {
	DB = db
}

func setState(userID int64, stateName string) {
	_, err := DB.Exec(context.Background(), `
		INSERT INTO states (user_id, state) 
		VALUES($1, $2)
		ON CONFLICT (user_id) DO UPDATE 
			SET state = excluded.state 
		`, userID, stateName)
	if err != nil {
		if stateName == NoState {
			stateName = "NoState"
		}
		fmt.Println(fmt.Errorf("BotExt: setStateToDB[%d], stateName '%s': postgres exec error %w", userID, stateName, err))
	}
}

func getState(userID int64) (stateName string) {
	err := DB.QueryRow(context.Background(),
		"SELECT state FROM states WHERE user_id = $1", userID).Scan(&stateName)
	if err == pgx.ErrNoRows {
		stateName = NoState
		err = nil
	}
	if err != nil {
		fmt.Println(fmt.Errorf("getStateFromDB[%d]: postgres QueryRow error %w", userID, err))
		return
	}
	return
}

func HasState(UserID int64) bool {
	var state string
	err := DB.QueryRow(context.Background(), "SELECT state from states where user_id = $1", UserID).Scan(&state)
	if err == nil {
		return state != NoState
	}
	return false
}

func ResetState(userID int64) {
	setState(userID, NoState)
	clearStateVars(userID)
}

///////////////////////////////////////////////// STATE VARIABLES FUNCTIONS

func SetStateVar(userID int64, varName string, varValue string) {
	_, err := DB.Exec(context.Background(), `
		UPDATE states
		SET temp_vars =  temp_vars || jsonb_build_object($1::text,$2::text)
		WHERE user_id = $3
	`, varName, varValue, userID)
	if err != nil {
		fmt.Println(fmt.Errorf("BotExt: setStateVarToDB[%d], varName %s, varValue %s: postgres QueryRow error %w",
			userID, varName, varValue, err))
	}
}

func GetStateVar(userID int64, varName string) (value string, ok bool) {
	err := DB.QueryRow(context.Background(), `
		SELECT temp_vars->>$1 FROM states
		WHERE user_id = $2
		`, varName, userID).Scan(&value)
	if err == pgx.ErrNoRows {
		err = nil
	}
	if err != nil {
		return
	}
	return value, true
}

func GetStateVars(userID int64) (values map[string]string) {
	var strJSON []byte

	err := DB.QueryRow(context.Background(), `
		SELECT temp_vars FROM states
		WHERE user_id = $1
		`, userID).Scan(&strJSON)
	if err == pgx.ErrNoRows {
		return
	}
	if err != nil {
		fmt.Println(fmt.Errorf("GetStateVars[%d]: postgres QueryRow error %w", userID, err))
		return
	}

	err = json.Unmarshal(strJSON, &values)
	if err != nil {
		fmt.Println(fmt.Errorf("GetStateVars[%d]: json.Unmarshall error %w", userID, err))
		return
	}

	return
}

func clearStateVars(userID int64) {
	_, err := DB.Exec(context.Background(), `
		UPDATE states
		SET temp_vars = '{}'::jsonb
		WHERE user_id = $1`, userID)
	if err != nil {
		fmt.Println(fmt.Errorf("ClearStateVars[%d]: postgres exec error %w", userID, err))
	}
}

/////////////////////////////////////////////////////////////////// MessageID functions

func setMessageID(userID int64, msgID int) {
	_, err := DB.Exec(context.Background(), `
		INSERT INTO states (user_id, message_id) 
		VALUES($1, $2)
		ON CONFLICT (user_id) DO UPDATE 
			SET message_id = excluded.message_id 
		`, userID, msgID)
	if err != nil {
		fmt.Println(fmt.Errorf("BotExt: setMessageID[%d], messageID '%d': postgres exec error %w", userID, msgID, err))

	}
}

func getMessageID(userID int64) (msgID int, ok bool) {
	err := DB.QueryRow(context.Background(),
		"SELECT message_id FROM states WHERE user_id = $1", userID).Scan(&msgID)
	if err == pgx.ErrNoRows {
		err = nil
	}
	if err != nil {
		fmt.Println(fmt.Errorf("getMessageID[%d]: postgres QueryRow error %w", userID, err))
		return
	}
	ok = true
	return
}
