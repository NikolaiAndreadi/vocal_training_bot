// FSM - Finite State Machine implementation
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	tele "gopkg.in/telebot.v3"
)

const (
	ResetState  = "__RESET__"
	ResumeState = ""
	NoState     = ""
)

// StateSetterType is a function that validates, checks and switches states
// if nextState equals ResetState, then state of the machine will be terminated (for concrete UserID)
// if nextState equals ResumeState or NoState, then state of the machine will not be changed
type StateSetterType func(c tele.Context) (nextState string, err error)

// State is a struct describing concrete FSM element.
// OnTrigger contains the action that will be executed on stage trigger. Can be string or telebot.Sendable
// ExtraOnTrigger contains additional events like menu with buttons. Can be string or telebot.Sendable
// StateSetter will be triggered on FSM.UpdateState and will decide if state changes.
type State struct {
	OnTrigger      interface{}
	ExtraOnTrigger []interface{}

	StateSetter StateSetterType
}

// FSM is a finite state machine structure
// All states are stored in the statePool map, where key is a unique StateName
type FSM struct {
	db        *pgxpool.Pool
	statePool map[string]State // key - StateName
	mu        *sync.RWMutex
}

// NewFSM is a constructor of the FSM struct
func NewFSM(db *pgxpool.Pool) FSM {
	return FSM{
		db:        db,
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
func (s *FSM) AddState(stateName string, onTrigger interface{}, stateSetter StateSetterType, extraOnTrigger ...interface{}) {
	// check uniqueness of the stateName
	if s.containsStateName(stateName) {
		panic(fmt.Errorf("AddState: State %s already exists", stateName))
	}

	state := State{
		OnTrigger:      onTrigger,
		ExtraOnTrigger: extraOnTrigger,
		StateSetter:    stateSetter,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statePool[stateName] = state
}

// ResetState clears out saved state in the database
func (s *FSM) ResetState(c tele.Context) error {
	err := s.ClearStateVars(c)
	if err != nil {
		return fmt.Errorf("ResetState: %w", err)
	}

	err = s.setStateToDB(c, "")
	if err != nil {
		return fmt.Errorf("ResetState: %w", err)
	}

	return nil
}

// TriggerState is a starting point of the state
func (s *FSM) TriggerState(c tele.Context, stateName string) error {
	if stateName == ResetState {
		err := s.ResetState(c)
		if err != nil {
			return fmt.Errorf("TriggerState: %w", err)
		}
	}

	// validate stateName
	if !s.containsStateName(stateName) {
		return fmt.Errorf("TriggerState[%d]: unknown state '%s'", c.Sender().ID, stateName)
	}

	// Set a new state to the database
	if err := s.setStateToDB(c, stateName); err != nil {
		return fmt.Errorf("TriggerState: %w", err)
	}

	// exec OnTrigger for the user
	s.mu.RLock()
	defer s.mu.RUnlock()
	if state, ok := s.statePool[stateName]; ok {
		return c.Send(state.OnTrigger, state.ExtraOnTrigger...)
	} else {
		return fmt.Errorf("TriggerState: Unknown state '%s'", stateName)
	}
}

// UpdateState is a method that should be executed at OnText, OnImage etc.. handlers
func (s *FSM) UpdateState(c tele.Context) error {
	stateName, err := s.getStateFromDB(c)
	if err != nil {
		return fmt.Errorf("UpdateState: %w", err)
	}
	if stateName == NoState {
		return c.Reply("Не могу ответить на это сообщение =(")
	}

	s.mu.RLock()
	state, ok := s.statePool[stateName]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("UpdateState[%d]: Unknown state '%s'", c.Sender().ID, stateName)
	}

	newState, err := state.StateSetter(c)
	if err != nil {
		return fmt.Errorf("UpdateState: %w", err)
	}

	if (newState != ResumeState) || (newState != stateName) {
		err = s.TriggerState(c, newState)
		if err != nil {
			return fmt.Errorf("UpdateState: %w", err)
		}
	}

	return nil
}

// STATE-VARIABLE-RELATED METHODS

// SetStateVar saves jsonb field for user
func (s *FSM) SetStateVar(c tele.Context, varName string, value string) error {
	userID := c.Sender().ID

	_, err := s.db.Exec(context.Background(), `
		UPDATE states
		SET temp_vars =  temp_vars || jsonb_build_object($1::text,$2::text)
		WHERE user_id = $3
	`, varName, value, userID)

	if err != nil {
		return fmt.Errorf("SetStateVar[%d], varName %s, value %s: postgres QueryRow error %w",
			userID, varName, value, err)
	}

	return nil
}

// GetStateVar extracts variable from jsonb column of table 'states'.
// if exists, ok return value will be true
func (s *FSM) GetStateVar(c tele.Context, varName string) (value string, ok bool, err error) {
	userID := c.Sender().ID

	err = s.db.QueryRow(context.Background(), `
		SELECT temp_vars->>$1 FROM states
		WHERE user_id = $2
		`, varName, userID).Scan(&value)

	ok = err == nil

	if err == pgx.ErrNoRows {
		err = nil
	}
	if err != nil {
		return value, ok, fmt.Errorf("GetStateVar[%d], varName %s: postgres QueryRow error %w", userID, varName, err)
	}

	return
}

// GetStateVars extracts whole jsonb column to map
func (s *FSM) GetStateVars(c tele.Context) (values map[string]string, err error) {
	var strJSON []byte
	userID := c.Sender().ID

	err = s.db.QueryRow(context.Background(), `
		SELECT temp_vars FROM states
		WHERE user_id = $1
		`, userID).Scan(&strJSON)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetStateVars[%d]: postgres QueryRow error %w", userID, err)
	}

	err = json.Unmarshal(strJSON, &values)
	if err != nil {
		return nil, fmt.Errorf("GetStateVars[%d]: json.Unmarshal error %w", userID, err)
	}

	return
}

// ClearStateVars flushes whole jsonb field for user
func (s *FSM) ClearStateVars(c tele.Context) error {
	userID := c.Sender().ID

	_, err := s.db.Exec(context.Background(), `
		UPDATE states
		SET temp_vars = '{}'::jsonb
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("ClearStateVars[%d]: postgres exec error %w", userID, err)
	}

	return nil
}

// DATABASE METHODS

func (s *FSM) setStateToDB(c tele.Context, stateName string) error {
	userID := c.Sender().ID

	_, err := s.db.Exec(context.Background(), `
		INSERT INTO states (user_id, state) 
		VALUES($1, $2)
		ON CONFLICT (user_id) DO UPDATE 
			SET state = excluded.state 
		`, userID, stateName)
	if err != nil {
		return fmt.Errorf("setStateToDB[%d], stateName %s: postgres exec error %w", userID, stateName, err)
	}

	return nil
}

func (s *FSM) getStateFromDB(c tele.Context) (stateName string, err error) {
	userID := c.Sender().ID

	err = s.db.QueryRow(context.Background(),
		"SELECT state FROM states WHERE user_id = $1", userID).Scan(&stateName)
	if err == pgx.ErrNoRows {
		// no state -> do nothing
		stateName = NoState
		err = nil
	}
	if err != nil {
		return stateName, fmt.Errorf("getStateFromDB[%d]: postgres QueryRow error %w", userID, err)
	}

	return stateName, err
}
