// Finite State Machine implementation

package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	tele "gopkg.in/telebot.v3"
	"sync"
)

const ResetState = "__RESET__"

type StateSetterType func(c tele.Context) (nextState string, e error)

type State struct {
	TextOnTrigger string
	StateSetter   StateSetterType
}

type States struct {
	statePool map[string]State // key - StateName
	mu        *sync.RWMutex
}

func CreateStates() States {
	return States{
		statePool: make(map[string]State),
		mu:        &sync.RWMutex{},
	}
}

func (s *States) AddState(stateName string, textOnTrigger string, stateSetter StateSetterType) {
	state := State{
		TextOnTrigger: textOnTrigger,
		StateSetter:   stateSetter,
	}
	s.mu.Lock()
	s.statePool[stateName] = state
	s.mu.Unlock()
}

func (s *States) TriggerState(c tele.Context, stateName string) error {
	userID := c.Sender().ID
	_, err := DB.Exec(context.Background(), `
			INSERT INTO states (user_id, state) 
			VALUES($1, $2)
			ON CONFLICT (user_id) DO UPDATE 
				SET state = excluded.state 
			`, userID, stateName)
	if err != nil {
		return err
	}
	s.mu.RLock()
	state := s.statePool[stateName]
	s.mu.RUnlock()
	return c.Reply(state.TextOnTrigger)
}

func (s *States) ResetState(c tele.Context) error {
	userID := c.Sender().ID
	_, err := DB.Exec(context.Background(), `
			INSERT INTO states (user_id, state) 
			VALUES($1, '')
			ON CONFLICT (user_id) DO UPDATE 
				SET state = excluded.state 
			`, userID)
	return err
}

func (s *States) UpdateState(c tele.Context) error {
	var stateName string
	userID := c.Sender().ID
	err := DB.QueryRow(context.Background(), "SELECT state FROM states WHERE user_id = $1", userID).Scan(&stateName)

	if err == pgx.ErrNoRows {
		// no state -> do nothing
		return c.Reply("I can't respond for this message")
	}
	if err != nil {
		return err
	}

	s.mu.RLock()
	state := s.statePool[stateName]
	s.mu.RUnlock()

	newState, err := state.StateSetter(c)

	if newState == ResetState {
		return s.ResetState(c)
	}

	if newState != "" {
		err2 := s.TriggerState(c, newState)
		if err != nil || err2 != nil {
			err2 = fmt.Errorf("UpdateState with new state: %v; %v", err, err2)
			return err2
		}
	}

	return err
}

// TODO read from map with err statement
