package BotExt

import (
	"math"

	tele "gopkg.in/telebot.v3"
)

// ReplyMenuConstructor creates reply menu (under user input field)
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
		if (i%maxElementsInRow == 0) || possibleSelection == RowSplitterButton {
			if len(buttons) != 0 {
				rows = append(rows, menu.Row(buttons...))
			}
			buttons = make([]tele.Btn, 0, maxElementsInRow)
		}
		if possibleSelection != RowSplitterButton {
			buttons = append(buttons, menu.Text(possibleSelection))
		}
	}
	if len(buttons) != 0 {
		rows = append(rows, menu.Row(buttons...))
	}
	menu.Reply(rows...)
	return menu
}
