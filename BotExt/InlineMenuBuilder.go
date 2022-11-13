package BotExt

import (
	"errors"
	"fmt"
	"strconv"

	om "github.com/wk8/go-ordered-map/v2"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

// InlineMenusType contains logic for all menus in "separate namespace"
type InlineMenusType struct {
	menus map[string]*InlineMenu
}

// NewInlineMenus - constructor for InlineMenusType
func NewInlineMenus() *InlineMenusType {
	return &InlineMenusType{
		menus: make(map[string]*InlineMenu),
	}
}

// GetInlineMenu - returns pointer to concrete InlineMenu by name
func (ims *InlineMenusType) GetInlineMenu(name string) *InlineMenu {
	if menu, ok := ims.menus[name]; ok {
		return menu
	}
	logger.Error("can't find inline menu", zap.String("menuName", name))
	return nil
}

// Update - refreshes content for concrete InlineMenu
func (ims *InlineMenusType) Update(c tele.Context, name string) {
	userID := c.Sender().ID
	menu, ok := ims.menus[name]
	if !ok {
		logger.Error("can't find inline menu", zap.Int64("userID", userID), zap.String("menuName", name))
		return
	}
	if msgID, ok := getMessageID(userID); ok {
		menu.Update(c, strconv.Itoa(msgID))
	} else {
		logger.Error("can't menu message id from db", zap.Int64("userID", userID), zap.String("menuName", name))
	}
}

// RegisterMenu adds concrete InlineMenu to InlineMenusType
func (ims *InlineMenusType) RegisterMenu(bot *tele.Bot, menu *InlineMenu) error {
	if _, ok := ims.menus[menu.Name]; ok {
		return fmt.Errorf("InlineMenusType.RegisterMenu: menu '%s' already registerd", menu.Name)
	}
	menu.construct(bot)
	ims.menus[menu.Name] = menu
	return nil
}

// Show will render user-specific menu
func (ims *InlineMenusType) Show(c tele.Context, menuName string) error {
	menu, ok := ims.menus[menuName]
	if !ok {
		return fmt.Errorf("InlineMenusType.Show: menu %s is not registered", menuName)
	}
	// current context - pressed ReplyMenu button, so next one - inline menu TODO - fix this
	setMessageID(c.Sender().ID, c.Message().ID+1)

	if m := menu.bake(c); m == nil {
		return nil
	} else {
		return c.Send(menu.header, m)
	}
}

// InlineMenu implementation

// InlineMenuTextSetter is a type for dynamic content setter
type InlineMenuTextSetter func(tele.Context, map[string]string) (string, error)

// DataFetcher is a type for extracting dynamic content from database
type DataFetcher func(c tele.Context) (map[string]string, error)

// ButtonFetcher is a type for extracting dynamic buttons (e.g. count) from database. OrderedMap -> buttons should be ordered
type ButtonFetcher func(c tele.Context) (*om.OrderedMap[string, string], error)

var NoButtons = errors.New("__NO_ROWS__")

// InlineMenu is an abstraction to construct both static and dynamic content into inline buttons.
// Name - unique id of menu, used to modify content of buttons
// header - mandatory by API message text.
// dataFetcher - function that grips all data from database into map. Can be nil.
// This map is used in InlineMenuTextSetter to insert data in specific format, in specific place
// textSetters - specific setter of dynamic content for every button. Uses InlineMenuTextSetter defined in button.
// btnTemplates - array of buttons to be rendered
type InlineMenu struct {
	Name            string
	header          string
	maxButtonsInRow int

	dataFetcher   DataFetcher
	buttonFetcher ButtonFetcher

	textSetters  map[string]InlineMenuTextSetter
	btnTemplates []*InlineButtonTemplate

	menuCarcass *tele.ReplyMarkup
}

// TODO: inlineMenu, dynamicInlineMenu => interface

// NewInlineMenu is a constructor for InlineMenu with dynamic content
func NewInlineMenu(menuName, menuHeader string, maxButtonsInRow int, fetcher DataFetcher) *InlineMenu {
	return &InlineMenu{
		Name:            menuName,
		header:          menuHeader,
		maxButtonsInRow: maxButtonsInRow,
		dataFetcher:     fetcher,
	}
}

// NewDynamicInlineMenu is a constructor for InlineMenu with dynamic button count
func NewDynamicInlineMenu(menuName, menuHeader string, maxButtonsInRow int, fetcher ButtonFetcher) *InlineMenu {
	return &InlineMenu{
		Name:            menuName,
		header:          menuHeader,
		maxButtonsInRow: maxButtonsInRow,
		buttonFetcher:   fetcher,
	}
}

// AddButtons adds concrete buttons into InlineMenu
func (im *InlineMenu) AddButtons(buttons []*InlineButtonTemplate) {
	im.PurgeButtons()
	for _, button := range buttons {
		im.AddButton(button)
	}
}

// PurgeButtons clears all possible dynamic content
func (im *InlineMenu) PurgeButtons() {
	im.btnTemplates = make([]*InlineButtonTemplate, 0)
	im.textSetters = make(map[string]InlineMenuTextSetter)
}

// AddButton adds only one button into InlineMenu
func (im *InlineMenu) AddButton(button *InlineButtonTemplate) {
	button.belongsToMenu = im

	// store dynamic content paste functions in a map, process only functions.
	// string variant will be put to the button on user-specific step later in func bake
	switch f := button.TextOnCreation.(type) { // InlineMenuTextSetter type assertion
	case func(tele.Context, map[string]string) (string, error):
		im.textSetters[button.Unique] = f
	}

	im.btnTemplates = append(im.btnTemplates, button)
}

// Update refreshes content of InlineMenu
func (im *InlineMenu) Update(c tele.Context, msgID string) {
	menu := im.bake(c)

	msg := tele.StoredMessage{
		MessageID: msgID,
		ChatID:    c.Chat().ID,
	}
	_, err := c.Bot().Edit(msg, im.header, menu)
	if (err != nil) && (err != tele.ErrSameMessageContent) {
		logger.Error("can't update inline menu", zap.Int64("userID", c.Sender().ID),
			zap.String("menuName", im.Name), zap.String("messageID", msgID), zap.Error(err))
	}
}

func (im *InlineMenu) construct(b *tele.Bot) {
	im.menuCarcass = &tele.ReplyMarkup{}

	var row []tele.Btn
	rows := make([]tele.Row, 0)
	for i, button := range im.btnTemplates {
		// fill static text in button and create handler
		bakedButton := im.manageButton(b, button)

		// button placement
		if (i%im.maxButtonsInRow == 0) || button.Unique == RowSplitterButton {
			if len(row) != 0 {
				rows = append(rows, im.menuCarcass.Row(row...))
			}
			row = make([]tele.Btn, 0)
		}
		if button.Unique != RowSplitterButton {
			row = append(row, bakedButton)
		}
	}
	// if there are some unprocessed buttons - place them into new row
	if len(row) != 0 {
		rows = append(rows, im.menuCarcass.Row(row...))
	}
	im.menuCarcass.Inline(rows...)
}

// dynamicBake uses buttonFetcher (NewDynamicInlineMenu) to extract buttons from database. key - id, value - name
func (im *InlineMenu) dynamicBake(c tele.Context) error {
	if im.buttonFetcher == nil {
		return nil
	}

	btnMap, err := im.buttonFetcher(c)
	if err != nil {
		if err == NoButtons {
			return NoButtons
		}
		return fmt.Errorf("UpdateButtons: %w", err)
	}

	im.PurgeButtons()

	if btnMap != nil {
		for pair := btnMap.Oldest(); pair != nil; pair = pair.Next() {
			im.AddButton(&InlineButtonTemplate{
				Unique:         pair.Key,
				TextOnCreation: pair.Value,
				OnClick:        pair.Value,
			})
		}
	}

	im.construct(c.Bot())

	return nil
}

func (im *InlineMenu) bake(c tele.Context) *tele.ReplyMarkup {
	if im.dataFetcher == nil {
		err := im.dynamicBake(c)
		if err != nil {
			if err == NoButtons {
				return nil
			}
			logger.Error("can't dynamicBake", zap.Int64("UserID", c.Sender().ID), zap.Error(err))
		}
		return im.menuCarcass
	}
	dynamicContentMap, err := im.dataFetcher(c)
	if err != nil {
		logger.Error("can't fetch data from db", zap.Int64("UserID", c.Sender().ID), zap.Error(err))
	}

	if dynamicContentMap == nil {
		return im.menuCarcass
	}

	for i, row := range im.menuCarcass.InlineKeyboard {
		for j, btn := range row {
			f, ok := im.textSetters[btn.Unique]
			if !ok {
				continue // static info had been placed already
			}
			content, err := f(c, dynamicContentMap)
			if err != nil {
				logger.Error("can't change val for button", zap.Int64("UserID", c.Sender().ID), zap.Error(err))
			}
			im.menuCarcass.InlineKeyboard[i][j].Text = content
		}
	}
	return im.menuCarcass
}

func (im *InlineMenu) manageButton(b *tele.Bot, button *InlineButtonTemplate) tele.Btn {
	staticText, ok := button.TextOnCreation.(string) // don't panic on failed type assertion (dynamic content is processed later)
	if !ok {
		staticText = "-" // empty string is not allowed
	}
	var bakedButton tele.Btn
	switch t := button.OnClick.(type) {
	case func(tele.Context) error:
		bakedButton = im.menuCarcass.Data(staticText, button.Unique, "\f"+button.Unique)
		b.Handle(&bakedButton, t)
	case string:
		bakedButton = im.menuCarcass.Data(t, button.Unique, im.Name)
	}
	return bakedButton
}

// InlineButtonTemplate implementation

// RowSplitterButton can be set as unique param of InlineButtonTemplate to split a table of buttons and start a new row
const RowSplitterButton = "__SPLITTER__"

// InlineButtonTemplate is an abstraction with static or dynamic content
// unique is a button name, should be unique along whole project. Used as a handler triggers
// textOnCreation is a (string) - static content, or (InlineMenuTextSetter) - dynamic content
// onClick is a (string) - FSM state trigger or (HandlerFunc) - some handler
type InlineButtonTemplate struct {
	Unique         string
	TextOnCreation interface{}
	OnClick        interface{} // string or tele.HandlerFunc
	belongsToMenu  *InlineMenu
}
