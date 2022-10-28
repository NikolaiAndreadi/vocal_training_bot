package main

import (
	"fmt"
	"math"

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

type InlineMenuTextSetter func(c tele.Context) string

type InlineMenuButton struct {
	Unique         string
	TextOnCreation interface{}
	OnClick        tele.HandlerFunc
}

func InlineMenuConstructor(b *tele.Bot, maxElementsInRow int, btnTemplates []*InlineMenuButton) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}

	itemCount := len(btnTemplates)
	rowCount := 1
	if itemCount > maxElementsInRow {
		rowCount = int(math.Ceil(float64(itemCount) / float64(maxElementsInRow)))
	}

	var constructedRow []tele.Btn
	rows := make([]tele.Row, rowCount)
	for i, button := range btnTemplates {
		var constructedText string
		switch val := button.TextOnCreation.(type) {
		case string:
			constructedText = val
		}
		constructedButton := menu.Data(constructedText, button.Unique, "\f"+button.Unique)
		b.Handle(&constructedButton, button.OnClick)

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

func FillInlineMenu(c tele.Context, menu *tele.ReplyMarkup, btnTemplates []*InlineMenuButton) *tele.ReplyMarkup {
	nameAssigners := make(map[string]InlineMenuTextSetter)

	for _, btn := range btnTemplates {
		switch f := btn.TextOnCreation.(type) {
		case func(c tele.Context) string: // without reflect of InlineMenuTextSetter
			nameAssigners[btn.Unique] = f
		default:
			fmt.Printf("FillInlineMenu[%d]: unknown type %T of button %s\n", c.Sender().ID, f, btn.Unique)
		}
	}

	for i, row := range menu.InlineKeyboard {
		for j, btn := range row {
			f, ok := nameAssigners[btn.Unique]
			if !ok {
				continue
			}
			menu.InlineKeyboard[i][j].Text = f(c)
		}
	}
	return menu
}

/*
InlineButtonTextUpdater Example:

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
func InlineButtonTextUpdater(c tele.Context, newText string) *tele.ReplyMarkup {
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
