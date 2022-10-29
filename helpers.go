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

type InlineMenuTextSetter func(c tele.Context) (string, error)

type InlineMenuButtonBlock struct {
	Buttons     []*InlineMenuButton
	TextSetters map[string]InlineMenuTextSetter
}

func NewInlineMenuButtonBlock(bb []*InlineMenuButton) (imbb InlineMenuButtonBlock) {
	imbb.Buttons = bb
	imbb.TextSetters = make(map[string]InlineMenuTextSetter, len(bb))
	for _, btn := range bb {
		switch f := btn.TextOnCreation.(type) {
		case func(c tele.Context) (string, error): // without reflect of InlineMenuTextSetter
			imbb.TextSetters[btn.Unique] = f
		}
	}
	return
}

type InlineMenuButton struct {
	Unique         string
	TextOnCreation interface{}
	OnClick        interface{} // tele.HandlerFunc or string. String == state
}

func InlineMenuConstructor(b *tele.Bot, fsm *FSM, maxElementsInRow int, btnTemplates InlineMenuButtonBlock) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}

	itemCount := len(btnTemplates.Buttons)
	rowCount := 1
	if itemCount > maxElementsInRow {
		rowCount = int(math.Ceil(float64(itemCount) / float64(maxElementsInRow)))
	}

	var constructedRow []tele.Btn
	rows := make([]tele.Row, rowCount)
	for i, button := range btnTemplates.Buttons {
		var constructedText string
		switch val := button.TextOnCreation.(type) {
		case string:
			constructedText = val
		}
		constructedButton := menu.Data(constructedText, button.Unique, "\f"+button.Unique)
		switch v := button.OnClick.(type) {
		case string:
			b.Handle(&constructedButton, func(c tele.Context) error {
				err := fsm.TriggerState(c, v)
				if err != nil {
					fmt.Println(err)
				}
				err = fsm.SetStateVar(c, "messageID", strconv.Itoa(c.Message().ID))
				if err != nil {
					fmt.Println(err)
				}
				err = fsm.SetStateVar(c, "inlineMenuText", c.Message().Text)
				if err != nil {
					fmt.Println(err)
				}
				return c.Respond()
			})
		case func(c tele.Context) error: // tele.HandlerFunc without reflect
			b.Handle(&constructedButton, v)
		}

		if i%maxElementsInRow == 0 {
			if len(constructedRow) != 0 {
				rows = append(rows, menu.Row(constructedRow...))
			}
			constructedRow = make([]tele.Btn, 0, maxElementsInRow)
		}
		constructedRow = append(constructedRow, constructedButton)
	}
	if len(constructedRow) != 0 {
		rows = append(rows, menu.Row(constructedRow...))
	}
	menu.Inline(rows...)
	return menu
}

func FillInlineMenu(c tele.Context, menu *tele.ReplyMarkup, btnTemplates InlineMenuButtonBlock) *tele.ReplyMarkup {
	for i, row := range menu.InlineKeyboard {
		for j, btn := range row {
			f, ok := btnTemplates.TextSetters[btn.Unique]
			if !ok {
				continue
			}
			val, err := f(c)
			if err != nil {
				fmt.Println(fmt.Errorf("can't change value for button %s, %w", menu.InlineKeyboard[i][j].Text, err))
				continue
			}
			menu.InlineKeyboard[i][j].Text = val
		}
	}
	return menu
}

func EditInlineMenu(c tele.Context, fsm *FSM, menu *tele.ReplyMarkup, btnTemplates InlineMenuButtonBlock) error {
	vars, err := fsm.GetStateVars(c)
	if err != nil {
		return fmt.Errorf("EditInlineMenu: can't get vars: %w", err)
	}
	mid, ok := vars["messageID"]
	if !ok {
		return fmt.Errorf("EditInlineMenu: can't get messageID: %w", err)
	}
	mtxt, ok := vars["inlineMenuText"]
	if !ok {
		return fmt.Errorf("EditInlineMenu: can't get inlineMenuText: %w", err)
	}

	for i, row := range menu.InlineKeyboard {
		for j, btn := range row {
			f, ok := btnTemplates.TextSetters[btn.Unique]
			if !ok {
				continue
			}
			val, err := f(c)
			if err != nil {
				fmt.Println(fmt.Errorf("can't change value for button %s, %w", menu.InlineKeyboard[i][j].Text, err))
				continue
			}
			menu.InlineKeyboard[i][j].Text = val
		}
	}

	m := tele.StoredMessage{
		MessageID: mid,
		ChatID:    c.Chat().ID,
	}

	_, err = c.Bot().Edit(m, mtxt, menu)

	return err
}

/*
InlineButtonTextInstantUpdater Example:

	bot.Handle(&btn, func(c tele.Context) error {
		if btn.Text == "+" {
			btn.Text = "-"
		} else {
			btn.Text = "+"
		}
		menu := InlineButtonUpdater(c, btn.Text)
		err := c.Respond(&tele.CallbackResponse{Text: "hi", ShowAlert: true})
		fmt.Println(err)
		return c.Edit(c.Callback().Message.Text, menu)
	})
*/
func InlineButtonTextInstantUpdater(c tele.Context, newText string) *tele.ReplyMarkup {
	cb := c.Callback()

	thisBtn := "\f" + cb.Unique
	if cb.Data != "" {
		thisBtn = thisBtn + "|" + cb.Data
	}

	menu := cb.Message.ReplyMarkup

loop:
	for i, row := range menu.InlineKeyboard {
		for j, btn := range row {
			if btn.Data == thisBtn {
				menu.InlineKeyboard[i][j].Text = newText
				break loop
			}
		}
	}
	return menu
}
