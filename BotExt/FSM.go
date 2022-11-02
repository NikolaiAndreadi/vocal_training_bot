package BotExt

import (
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	tele "gopkg.in/telebot.v3"
)

type FSM struct {
	stateMap map[string]*State
	menus    *InlineMenusType
}

func NewFiniteStateMachine(db *pgxpool.Pool, ims *InlineMenusType) *FSM {
	DB = db
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
	f.stateMap[s.Name] = s
	return nil
}

func (f *FSM) Trigger(c tele.Context, stateName string, byMenu ...string) {
	state, ok := f.stateMap[stateName]
	if !ok {
		fmt.Println(fmt.Errorf("BotExt.Trigger[%d]: no such state '%s'", c.Sender().ID, stateName))
		return
	}
	if len(byMenu) != 0 {
		state.menuTrigger = byMenu[0]
	}
	state.Trigger(c)
}

func (f *FSM) Update(c tele.Context) {
	stateName := getState(c)
	if stateName == "" {
		return
	}
	state, ok := f.stateMap[stateName]
	if !ok {
		fmt.Println(fmt.Errorf("BotExt.Update[%d]: stateName '%s' from database is corrupted", c.Sender().ID, stateName))
	}
	state.Update(c)
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

	OnSuccess      interface{}
	OnSuccessExtra []interface{}

	fsm         *FSM
	menus       *InlineMenusType
	next        string
	menuTrigger string // "" if no menu

}

// Trigger is a method to start a State for specific user.
func (s *State) Trigger(c tele.Context) {
	setState(c, s.Name)
	var err error
	if s.OnTriggerExtra == nil {
		err = c.Send(s.OnTrigger)
	} else {
		err = c.Send(s.OnTrigger, s.OnTriggerExtra...)
	}
	if err != nil {
		fmt.Println(fmt.Errorf("state %s.Trigger[%d]: can't send a message: %w", s.Name, c.Sender().ID, err))
	}
}

// Update is a function to process current state
func (s *State) Update(c tele.Context) {
	errString := s.Validator(c)
	if errString != "" {
		err := c.Send(errString)
		if err != nil {
			fmt.Println(fmt.Errorf("state %s.Update[%d]: can't send validation error message: %w", s.Name, c.Sender().ID, err))
		}
		return
	}

	if s.Manipulator != nil {
		err := s.Manipulator(c)
		if err != nil {
			err2 := c.Send("Что-то пошло не так... Мы будем разбираться, в чем была проблема. Попробуй повторить это действие позже!")
			if err2 != nil {
				fmt.Println(fmt.Errorf("state %s.Update[%d]: can't send a manipulator message: %w", s.Name, c.Sender().ID, err))
			}
			fmt.Println(fmt.Errorf("state %s.Update[%d]: manipulator: %w", s.Name, c.Sender().ID, err))
			ResetState(c)
			return
		}
	}

	if s.menuTrigger != "" {
		menu := s.fsm.menus.GetInlineMenu(s.menuTrigger)
		if msgID, ok := getMessageID(c); ok {
			menu.Update(c, strconv.Itoa(msgID))
		} else {
			fmt.Println(fmt.Errorf("state %s.Update[%d]: can't fetch menuMessageID from db", s.Name, c.Sender().ID))
		}
	}

	if s.OnSuccess != nil {
		var err error
		if s.OnSuccessExtra == nil {
			err = c.Send(s.OnSuccess)
		} else {
			err = c.Send(s.OnSuccess, s.OnSuccessExtra...)
		}
		if err != nil {
			fmt.Println(fmt.Errorf("state %s.Update[%d]: can't send success message: %w", s.Name, c.Sender().ID, err))
		}
	}

	if s.next == "" {
		ResetState(c)
	} else {
		s.fsm.Trigger(c, s.next)
	}
}
