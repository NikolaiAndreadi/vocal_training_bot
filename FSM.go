// Finite State Machine implementation

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	tele "gopkg.in/telebot.v3"
	"sync"
)

const (
	ResetState  = "__RESET__"
	ResumeState = ""
	NoState     = ""
)

// StateSetterType is a function that validates, checks and switches states
// if nextState equals ResetState, then state of the machine will be terminated (for concrete UserID)
// if nextState equals ResumeState, then state of the machine will not be changed
type StateSetterType func(c tele.Context) (nextState string, err error)

// State is a struct describing State logic.
// TextOnTrigger will be shown to user right after State is triggered
// Corresponding StateSetter would be triggered on FSM.UpdateState method
type State struct {
	OnTrigger      interface{}
	ExtraOnTrigger []interface{}

	StateSetter StateSetterType
}

// FSM is a finite state machine structure
// All states are stored in a concurrent-ready statePool map, where key is a unique StateName
type FSM struct {
	statePool map[string]State // key - StateName
	mu        *sync.RWMutex
}

// NewFSM is a constructor of the FSM struct
func NewFSM() FSM {
	return FSM{
		statePool: make(map[string]State),
		mu:        &sync.RWMutex{},
	}
}

// containsStateName checks if stateName in the statePool
func (s *FSM) containsStateName(stateName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.statePool[stateName]
	return ok
}

// AddState method adds a new State to the FSM with unique stateName.
// textOnTrigger will be shown to user right after State is triggered
// Corresponding stateSetter would be triggered on FSM.UpdateState method
func (s *FSM) AddState(stateName string, OnTrigger interface{}, stateSetter StateSetterType, ExtraOnTrigger ...interface{}) {
	// check uniqueness of the stateName
	if s.containsStateName(stateName) {
		panic(fmt.Errorf("AddState: State %s already exists", stateName))
	}

	state := State{
		OnTrigger:      OnTrigger,
		ExtraOnTrigger: ExtraOnTrigger,
		StateSetter:    stateSetter,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statePool[stateName] = state
}

// TriggerState is a starting point of the state
func (s *FSM) TriggerState(c tele.Context, stateName string) error {
	if stateName == ResumeState {
		return nil
	}
	if stateName == ResetState {
		return s.ResetState(c)
	}
	// validate stateName
	if !s.containsStateName(stateName) {
		return fmt.Errorf("TriggerState: Unknown state '%s'", stateName)
	}

	// Set a new state to the database
	if err2 := setStateToDB(c, stateName); err2 != nil {
		return err2
	}

	// Show send TextOnTrigger message to the user
	s.mu.RLock()
	defer s.mu.RUnlock()
	if state, ok := s.statePool[stateName]; ok {
		return c.Send(state.OnTrigger, state.ExtraOnTrigger...)
	} else {
		return fmt.Errorf("TriggerState: Unknown state '%s'", stateName)
	}
}

// ResetState clears out saved state in the database
func (s *FSM) ResetState(c tele.Context) error {
	err := ClearStateVars(c)
	if err != nil {
		fmt.Println("ResetState: %w", err)
	}

	return setStateToDB(c, "")
}

func (s *FSM) UpdateState(c tele.Context) error {
	stateName, err := getStateFromDB(c)
	if err != nil {
		return err
	}
	if stateName == NoState {
		return c.Reply("Не могу ответить на это сообщение =(")
	}

	s.mu.RLock()
	state, ok := s.statePool[stateName]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("UpdateState: Unknown state '%s'", stateName)
	}

	newState, err := state.StateSetter(c)
	if err != nil {
		return err
	}
	if newState == ResetState {
		return s.ResetState(c)
	}

	if (newState != ResumeState) || (newState != stateName) {
		err = s.TriggerState(c, newState)
		if err != nil {
			return err
		}
	}

	return nil
}

// STATE-RELATED VARIABLE METHOD

// SetStateVar saves jsonb field for user
func SetStateVar(c tele.Context, varName string, value string) error {
	userID := c.Sender().ID
	_, err := DB.Exec(context.Background(), `
		UPDATE states
		SET temp_vars =  temp_vars || jsonb_build_object($1::text,$2::text)
		WHERE user_id = $3
	`, varName, value, userID)
	return err
}

// GetStateVar extracts variable from jsonb column of table 'states'.
// if exists, ok param will be true
func GetStateVar(c tele.Context, varName string) (value string, ok bool, err error) {
	userID := c.Sender().ID
	err = DB.QueryRow(context.Background(), `
		SELECT temp_vars->>$1 FROM states
		WHERE user_id = $2
		`, varName, userID).Scan(&value)
	ok = true

	if err != nil {
		ok = false
	}
	if err == pgx.ErrNoRows {
		err = nil
	}
	return
}

func GetStateVars(c tele.Context) (values map[string]string, err error) {
	userID := c.Sender().ID

	var strJSON []byte
	err = DB.QueryRow(context.Background(), `
		SELECT temp_vars FROM states
		WHERE user_id = $1
		`, userID).Scan(&strJSON)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	err = json.Unmarshal(strJSON, &values)
	return
}

// ClearStateVars flushes whole jsonb field for user
func ClearStateVars(c tele.Context) error {
	userID := c.Sender().ID
	_, err := DB.Exec(context.Background(), `
		UPDATE states
		SET temp_vars = '{}'::jsonb
		WHERE user_id = $1
	`, userID)
	return err
}

// DATABASE functions

func setStateToDB(c tele.Context, stateName string) error {
	userID := c.Sender().ID
	_, err := DB.Exec(context.Background(), `
		INSERT INTO states (user_id, state) 
		VALUES($1, $2)
		ON CONFLICT (user_id) DO UPDATE 
			SET state = excluded.state 
		`, userID, stateName)
	return err
}

func getStateFromDB(c tele.Context) (stateName string, err error) {
	userID := c.Sender().ID
	err = DB.QueryRow(context.Background(),
		"SELECT state FROM states WHERE user_id = $1", userID).Scan(&stateName)
	if err == pgx.ErrNoRows {
		// no state -> do nothing
		stateName = NoState
		err = nil
	}
	return stateName, err
}
