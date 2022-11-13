package BotExt

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// variables from outer scope. Use SetVars to set them up
var (
	DB     *pgxpool.Pool
	logger *zap.Logger
)

// NoState will be returned if there are no rows in state database
const NoState = ""

// SetVars sets up global vars for BotExt package
func SetVars(db *pgxpool.Pool, l *zap.Logger) {
	DB = db
	logger = l
}

// STATE RELATED FUNCTIONS

// setState defines current state for user
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
		logger.Error("pg exec error", zap.Int64("UserID", userID),
			zap.String("state", stateName), zap.Error(err))
	}
}

// setState extract current state for user from database
func getState(userID int64) (stateName string) {
	err := DB.QueryRow(context.Background(),
		"SELECT state FROM states WHERE user_id = $1", userID).Scan(&stateName)
	if err == pgx.ErrNoRows {
		stateName = NoState
		err = nil
	}
	if err != nil {
		logger.Error("pg query error", zap.Int64("UserID", userID), zap.Error(err))
		return
	}
	return
}

// HasState returns true if user have some state
func HasState(UserID int64) bool {
	var state string
	err := DB.QueryRow(context.Background(), "SELECT state from states where user_id = $1", UserID).Scan(&state)
	if err == nil {
		return state != NoState
	}
	return false
}

// ResetState empties state column in database
func ResetState(userID int64, keepVars bool) {
	setState(userID, NoState)
	if !keepVars {
		ClearStateVars(userID)
	}
}

// STATE VARIABLE RELATED FUNCTIONS

// SetStateVar sets variable related to state in jsonb format
func SetStateVar(userID int64, varName string, varValue string) {
	_, err := DB.Exec(context.Background(), `
		UPDATE states
		SET temp_vars =  temp_vars || jsonb_build_object($1::text,$2::text)
		WHERE user_id = $3
	`, varName, varValue, userID)
	if err != nil {
		logger.Error("pg exec error", zap.Int64("UserID", userID),
			zap.String("varName", varName), zap.String("varValue", varValue), zap.Error(err))
	}
}

// GetStateVar extracts variable related to state if it exists (ok return value)
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

// GetStateVars returns all state variables related to user
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
		logger.Error("pg query error", zap.Int64("UserID", userID), zap.Error(err))
		return
	}

	err = json.Unmarshal(strJSON, &values)
	if err != nil {
		logger.Error("unmarshal error", zap.Int64("UserID", userID), zap.ByteString("strJSON", strJSON), zap.Error(err))
		return
	}

	return
}

// ClearStateVars empties all state variables related to user
func ClearStateVars(userID int64) {
	_, err := DB.Exec(context.Background(), `
		UPDATE states
		SET temp_vars = '{}'::jsonb
		WHERE user_id = $1`, userID)
	if err != nil {
		logger.Error("pg exec error", zap.Int64("UserID", userID), zap.Error(err))
	}
}

// MESSAGE ID  FUNCTIONS

// setMessageID saves id to states database, used to update menu content after some action
func setMessageID(userID int64, msgID int) {
	_, err := DB.Exec(context.Background(), `
		INSERT INTO states (user_id, message_id) 
		VALUES($1, $2)
		ON CONFLICT (user_id) DO UPDATE 
			SET message_id = excluded.message_id 
		`, userID, msgID)
	if err != nil {
		logger.Error("pg exec error", zap.Int64("UserID", userID), zap.Int("msgID", msgID), zap.Error(err))
	}
}

// getMessageID extracts message id from database (if exists)
func getMessageID(userID int64) (msgID int, ok bool) {
	err := DB.QueryRow(context.Background(),
		"SELECT message_id FROM states WHERE user_id = $1", userID).Scan(&msgID)
	if err == pgx.ErrNoRows {
		err = nil
	}
	if err != nil {
		logger.Error("pg query error", zap.Int64("UserID", userID), zap.Error(err))
		return
	}
	ok = true
	return
}
