package BotExt

import (
	"errors"
	"fmt"
	"strconv"

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

// ContinueState used in State.Manipulator to not end current state and wait for further messages from user
var ContinueState = errors.New("__CONTINUE__")

// FSM is a base structure to manage all existing states. Used in conjunction with InlineMenus, as their content
// can be updated by the state
type FSM struct {
	stateMap map[string]*State
	menus    *InlineMenusType
}

// NewFiniteStateMachine is a constructor for FSM. One can create separate FSMs for different user groups (admins, users...)
func NewFiniteStateMachine(ims *InlineMenusType) *FSM {
	return &FSM{
		stateMap: make(map[string]*State),
		menus:    ims,
	}
}

// RegisterStateChain creates linked list of states, that executes one after another
func (f *FSM) RegisterStateChain(states []*State) error {
	if len(states) == 0 {
		return fmt.Errorf("RegisterStates: no states specified")
	}
	// validate that every stateName is implemented
	for i, s := range states {
		if _, ok := f.stateMap[s.Name]; ok {
			return fmt.Errorf("RegisterStateChain: state %s already registered", s.Name)
		}
		s.fsm = f
		f.stateMap[s.Name] = s
		// fill next
		if i != 0 {
			prevName := states[i-1].Name
			prev := f.stateMap[prevName]
			prev.next = s.Name
		}
	}
	return nil
}

// RegisterOneShotState - creates single state
func (f *FSM) RegisterOneShotState(s *State) error {
	if _, ok := f.stateMap[s.Name]; ok {
		return fmt.Errorf("RegisterOneShotState: state %s already registered", s.Name)
	}
	s.fsm = f
	f.stateMap[s.Name] = s
	return nil
}

// Trigger - starting point of State. byMenu is used if States starting in menu with content that can be changed
func (f *FSM) Trigger(c tele.Context, stateName string, byMenu ...string) {
	state, ok := f.stateMap[stateName]
	if !ok {
		logger.Error("no such state", zap.Int64("UserID", c.Sender().ID), zap.String("stateName", stateName))
		return
	}
	if len(byMenu) != 0 {
		state.menuTrigger = byMenu[0]
	}
	state.Trigger(c)
}

// Update starts cycle of Validation and data Manipulation for the State
func (f *FSM) Update(c tele.Context) {
	stateName := f.GetCurrentState(c)
	if stateName == "" {
		return
	}
	state, ok := f.stateMap[stateName]
	if !ok {
		logger.Error("state from db is corrupted", zap.Int64("UserID", c.Sender().ID), zap.String("stateName", stateName))
	}
	state.Update(c)
}

// GetCurrentState extracts current state from the database
func (f *FSM) GetCurrentState(c tele.Context) string {
	return getState(c.Sender().ID)
}

// State definition

// State is a chain element in FSM implementation
//   - Name required for logging
//   - Validator is a function that validates user input
//     validator returns "" if validation is successful, otherwise - it is an error message for user
//   - Manipulator is a function that changes data in database
//   - OnTrigger - string or telebot.Sendable, will be telebot.Context.Send to user
//   - OnTriggerExtra - variadic argument of telebot.Context.Send. Can be *telebot.SendOptions, *telebot.ReplyMarkup...
//     if OnTriggerExtra is string -> it is trigger for menu rendering
//   - OnSuccess -  string or telebot.Sendable, will be telebot.Context.Send to user after State completion
//   - OnQuitExtra - variadic argument of telebot.Context.Send. Can be *telebot.SendOptions, *telebot.ReplyMarkup...
//     this would be executed even on not successful run
//   - KeepVarsOnQuit - should StateVars be cleared after completion?
type State struct {
	Name string

	Validator   func(tele.Context) string
	Manipulator func(tele.Context) error

	OnTrigger      interface{}
	OnTriggerExtra []interface{}

	OnSuccess   interface{}
	OnQuitExtra []interface{}

	KeepVarsOnQuit bool

	fsm         *FSM
	next        string
	menuTrigger string // "" if no menu

}

// Trigger is a method to start a State for specific user.
func (s *State) Trigger(c tele.Context) {
	userID := c.Sender().ID
	setState(userID, s.Name)
	var err error
	if s.OnTriggerExtra != nil {
		if len(s.OnTriggerExtra) == 1 {
			// if OnTriggerExtra is string -> it is trigger for menu rendering
			switch ote := s.OnTriggerExtra[0].(type) {
			case string:
				_ = c.Send(s.OnTrigger)
				oldMsgID, _ := getMessageID(userID)
				err = s.fsm.menus.Show(c, ote)
				setMessageID(userID, oldMsgID)
			default:
				err = c.Send(s.OnTrigger, ote)
			}
		} else {
			err = c.Send(s.OnTrigger, s.OnTriggerExtra...)
		}
	} else {
		err = c.Send(s.OnTrigger)
	}
	if err != nil {
		logger.Error("can't send a message", zap.Int64("UserID", userID), zap.Error(err))
	}

	if s.Validator == nil {
		if s.Manipulator == nil {
			s.Update(c)
		}
	}
}

// Update is a function to process current state
func (s *State) Update(c tele.Context) {
	c.Set("state", s.Name)
	if s.Validator != nil {
		errString := s.Validator(c)
		if errString != "" {
			err := c.Send(errString)
			if err != nil {
				logger.Error("can't send validation", zap.Int64("UserID", c.Sender().ID), zap.Error(err))
			}
			return
		}
	}

	if s.Manipulator != nil {
		err := s.Manipulator(c)
		if err != nil {
			if err == ContinueState {
				return
			}
			text := "Что-то пошло не так... Мы будем разбираться, в чем была проблема. Попробуй повторить это действие позже!"
			var err2 error
			if s.OnQuitExtra != nil {
				err2 = c.Send(text, s.OnQuitExtra)
			} else {
				err2 = c.Send(text)
			}
			if err2 != nil {
				logger.Error("can't send manipulator2", zap.Int64("UserID", c.Sender().ID), zap.Error(err2))
			}
			logger.Error("can't send manipulator", zap.Int64("UserID", c.Sender().ID), zap.Error(err))
			ResetState(c.Sender().ID, s.KeepVarsOnQuit)
			return
		}
	}

	if s.menuTrigger != "" {
		menu := s.fsm.menus.GetInlineMenu(s.menuTrigger)
		if msgID, ok := getMessageID(c.Sender().ID); ok {
			menu.Update(c, strconv.Itoa(msgID))
		} else {
			logger.Error("can't fetch menuMessageID from db", zap.Int64("UserID", c.Sender().ID), zap.String("state", s.Name))
		}
	}

	if s.OnSuccess != nil {
		var err error
		if s.OnQuitExtra == nil {
			err = c.Send(s.OnSuccess)
		} else {
			err = c.Send(s.OnSuccess, s.OnQuitExtra...)
		}
		if err != nil {
			logger.Error("can't send success msg", zap.Int64("UserID", c.Sender().ID),
				zap.String("state", s.Name), zap.Error(err))
		}
	}

	if s.next == "" {
		ResetState(c.Sender().ID, s.KeepVarsOnQuit)
	} else {
		s.fsm.Trigger(c, s.next)
	}
}
