package main

import (
	"fmt"
	"math"
	"strconv"

	tele "gopkg.in/telebot.v3"
)

func ReplyMenuConstructor(possibleSelections []string, maxElementsInRow int, once bool) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{
		ResizeKeyboard:  true,
		OneTimeKeyboard: once,
		RemoveKeyboard:  once,
	}

	itemCount := len(possibleSelections)
	rowCount := 1
	if itemCount > maxElementsInRow {
		rowCount = int(math.Ceil(float64(itemCount) / float64(maxElementsInRow)))
	}

	var buttons []tele.Btn
	rows := make([]tele.Row, rowCount)
	for i, possibleSelection := range possibleSelections {
		if i%maxElementsInRow == 0 {
			if len(buttons) != 0 {
				rows = append(rows, menu.Row(buttons...))
			}
			buttons = make([]tele.Btn, 0, maxElementsInRow)
		}
		buttons = append(buttons, menu.Text(possibleSelection))
	}
	if len(buttons) != 0 {
		rows = append(rows, menu.Row(buttons...))
	}
	menu.Reply(rows...)
	return menu
}

// INLINE BUTTON HELPERS

// InlineButtonTemplate is an abstraction with static or dynamic content
// unique is a button name, should be unique along whole project. Used as a handler triggers
// textOnCreation is a (string) - static content, or (InlineMenuTextSetter) - dynamic content
// onClick is a (string) - FSM state trigger or (HandlerFunc) - some handler
type InlineButtonTemplate struct {
	unique         string
	textOnCreation interface{}
	onClick        interface{} // tele.HandlerFunc or string. String == state
}

/////////////////////////////////////////////////////////////

// InlineMenuTextSetter is a type for dynamic content setter
type InlineMenuTextSetter func(tele.Context, map[string]string) (string, error)

// DataFetcher is a type for extracting dynamic content from database
type DataFetcher func(c tele.Context) (map[string]string, error)

// InlineMenu is an abstraction to construct both static and dynamic content into inline buttons.
// menuHeader - mandatory by API message text.
// dataFetcher - function that grips all data from database into map storage. Can be nil.
// textSetters - specific setter of dynamic content for every button. Uses InlineMenuTextSetter defined in button.
// btnTemplates - array of buttons to be rendered
type InlineMenu struct {
	menuHeader string

	dataFetcher DataFetcher

	textSetters  map[string]InlineMenuTextSetter
	btnTemplates []*InlineButtonTemplate

	menuCarcass *tele.ReplyMarkup
}

// NewInlineMenu is a constructor for
func NewInlineMenu(menuHeader string, dataFetcher DataFetcher) *InlineMenu {
	return &InlineMenu{
		menuHeader:  menuHeader,
		dataFetcher: dataFetcher,
	}
}

func (im *InlineMenu) AddButtons(buttons []*InlineButtonTemplate) {
	im.btnTemplates = buttons
	// store dynamic content paste functions in a map, process only functions.
	// string variant will be put to the button on user-specific step later in func bake
	im.textSetters = make(map[string]InlineMenuTextSetter)
	for _, button := range buttons {
		switch f := button.textOnCreation.(type) { // InlineMenuTextSetter type assertion
		case func(tele.Context, map[string]string) (string, error):
			im.textSetters[button.unique] = f
		}
	}
}

func (im *InlineMenu) Construct(b *tele.Bot, fsm *FSM, maxElementsInRow int) {
	im.menuCarcass = &tele.ReplyMarkup{}

	var row []tele.Btn
	rows := make([]tele.Row, 0)
	for i, button := range im.btnTemplates {
		// fill static text in button and create handler
		bakedButton := im.manageButton(b, fsm, button)

		// button placement
		if i%maxElementsInRow == 0 {
			if len(row) != 0 {
				rows = append(rows, im.menuCarcass.Row(row...))
			}
			row = make([]tele.Btn, 0)
		}
		row = append(row, bakedButton)
	}
	// if there are some unprocessed buttons - place them into new row
	if len(row) != 0 {
		rows = append(rows, im.menuCarcass.Row(row...))
	}
	im.menuCarcass.Inline(rows...)
}

func (im *InlineMenu) Serve(c tele.Context) error {
	return c.Send(im.menuHeader, im.bake(c))
}

func (im *InlineMenu) Update(c tele.Context, fsm *FSM) error {
	vars, err := fsm.GetStateVars(c)
	if err != nil {
		return fmt.Errorf("EditInlineMenu: can't get vars: %w", err)
	}
	menuID, ok := vars["messageID"]
	if !ok {
		return fmt.Errorf("EditInlineMenu: can't get messageID: %w", err)
	}

	menu := im.bake(c)

	msg := tele.StoredMessage{
		MessageID: menuID,
		ChatID:    c.Chat().ID,
	}

	_, err = c.Bot().Edit(msg, im.menuHeader, menu)

	return err
}

func (im *InlineMenu) bake(c tele.Context) *tele.ReplyMarkup {
	dynamicContentMap, err := im.dataFetcher(c)
	if err != nil {
		fmt.Println(fmt.Errorf("can't fetch data from database: %w", err))
	}

	for i, row := range im.menuCarcass.InlineKeyboard {
		for j, btn := range row {
			f, ok := im.textSetters[btn.Unique]
			if !ok {
				continue // static info had been placed already
			}
			content, err := f(c, dynamicContentMap)
			if err != nil {
				fmt.Println(fmt.Errorf("can't change value for button %s, %w", im.menuCarcass.InlineKeyboard[i][j].Text, err))
				continue
			}
			im.menuCarcass.InlineKeyboard[i][j].Text = content
		}
	}
	return im.menuCarcass
}

func (im *InlineMenu) manageButton(b *tele.Bot, fsm *FSM, button *InlineButtonTemplate) tele.Btn {
	staticText, ok := button.textOnCreation.(string) // don't panic on failed type assertion (dynamic content is processed later)
	if !ok {
		staticText = "." // empty string is not allowed
	}
	bakedButton := im.menuCarcass.Data(staticText, button.unique, "\f"+button.unique)

	switch v := button.onClick.(type) {
	case string: // if handlerFunc - FSM state => prepare state variables for further updating of button dynamic content
		stateHandler := createHandlerFSM(fsm, button)
		b.Handle(&bakedButton, stateHandler)
	case func(tele.Context) error: // tele.HandlerFunc
		b.Handle(&bakedButton, v)
	}

	return bakedButton
}

func createHandlerFSM(fsm *FSM, button *InlineButtonTemplate) tele.HandlerFunc {
	return func(c tele.Context) error {
		state, ok := button.onClick.(string)
		if !ok {
			fmt.Println(fmt.Errorf("can't make type assertion to string"))
		}
		err := fsm.TriggerState(c, state)
		if err != nil {
			fmt.Println(err)
		}
		err = fsm.SetStateVar(c, "messageID", strconv.Itoa(c.Message().ID))
		if err != nil {
			fmt.Println(err)
		}
		return c.Respond()
	}
}
