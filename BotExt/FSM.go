package BotExt

import (
	"errors"
	"fmt"
	"strconv"

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

var ContinueState = errors.New("__CONTINUE__")

type FSM struct {
	stateMap map[string]*State
	menus    *InlineMenusType
}

func NewFiniteStateMachine(ims *InlineMenusType) *FSM {
	return &FSM{
		stateMap: make(map[string]*State),
		menus:    ims,
	}
}

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
		s.menus = f.menus
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

func (f *FSM) RegisterOneShotState(s *State) error {
	if _, ok := f.stateMap[s.Name]; ok {
		return fmt.Errorf("RegisterOneShotState: state %s already registered", s.Name)
	}
	s.fsm = f
	s.menus = f.menus
	f.stateMap[s.Name] = s
	return nil
}

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

func (f *FSM) Update(c tele.Context) {
	stateName := getState(c.Sender().ID)
	if stateName == "" {
		return
	}
	state, ok := f.stateMap[stateName]
	if !ok {
		logger.Error("state from db is corrupted", zap.Int64("UserID", c.Sender().ID), zap.String("stateName", stateName))
	}
	state.Update(c)
}

func (f *FSM) GetCurrentState(c tele.Context) string {
	return getState(c.Sender().ID)
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// State is a chain element in Finite State Machine implementation
//   - Name required for logging
//   - validator is a function that validates user input
//   - manipulator is a function that changes data in database
//   - onTrigger - string or telebot.Sendable, should be telebot.Context.Send to user on state trigger
//   - onTriggerExtra - variadic argument of telebot.Context.Send. Can be *telebot.SendOptions, *telebot.ReplyMarkup,
//     telebot.Option, telebot.ParseMode, telebot.Entities
//   - next is a next state in chain. If empty - end of the chain
type State struct {
	Name string

	Validator   func(tele.Context) string
	Manipulator func(tele.Context) error

	OnTrigger      interface{}
	OnTriggerExtra []interface{}

	OnSuccess   interface{}
	OnQuitExtra []interface{}

	fsm         *FSM
	menus       *InlineMenusType
	next        string
	menuTrigger string // "" if no menu

}

// Trigger is a method to start a State for specific user.
func (s *State) Trigger(c tele.Context) {
	setState(c.Sender().ID, s.Name)
	var err error
	if s.OnTriggerExtra == nil {
		err = c.Send(s.OnTrigger)
	} else {
		if len(s.OnTriggerExtra) == 1 {
			switch ote := s.OnTriggerExtra[0].(type) {
			case string:
				_ = c.Send(s.OnTrigger)
				err = s.menus.Show(c, ote)
			default:
				err = c.Send(s.OnTrigger, ote)
			}
		} else {
			err = c.Send(s.OnTrigger, s.OnTriggerExtra...)
		}
	}
	if err != nil {
		logger.Error("can't send a message", zap.Int64("UserID", c.Sender().ID), zap.Error(err))
	}
}

// Update is a function to process current state
func (s *State) Update(c tele.Context) {
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
			ResetState(c.Sender().ID)
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
		ResetState(c.Sender().ID)
	} else {
		s.fsm.Trigger(c, s.next)
	}
}
