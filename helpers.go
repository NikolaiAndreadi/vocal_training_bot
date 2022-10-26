package main

import (
	"math"

	tele "gopkg.in/telebot.v3"
)

func ReplyMenuConstructor(possibleSelections []string, maxElementsInRow int) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
	}

	itemCount := len(possibleSelections)

	var rowCount int
	if itemCount < maxElementsInRow {
		rowCount = 1
	} else {
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
