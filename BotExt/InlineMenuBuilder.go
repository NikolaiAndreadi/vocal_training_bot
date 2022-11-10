package BotExt

import (
	"fmt"
	"strconv"

	om "github.com/wk8/go-ordered-map/v2"
	tele "gopkg.in/telebot.v3"
)

type InlineMenusType struct {
	menus map[string]*InlineMenu
}

func NewInlineMenus() *InlineMenusType {
	return &InlineMenusType{
		menus: make(map[string]*InlineMenu),
	}
}

func (ims *InlineMenusType) GetInlineMenu(name string) *InlineMenu {
	if menu, ok := ims.menus[name]; ok {
		return menu
	}
	fmt.Println(fmt.Errorf("InlineMenusType.Update: can't find menu '%s'", name))
	return nil
}

func (ims *InlineMenusType) Update(c tele.Context, name string) {
	userID := c.Sender().ID
	if menu, ok := ims.menus[name]; ok {
		if msgID, ok := getMessageID(userID); ok {
			menu.Update(c, strconv.Itoa(msgID))
		} else {
			fmt.Println(fmt.Errorf("ims.Update[%d]: can't fetch menuMessageID from db for menu %s", userID, name))
		}
	} else {
		fmt.Println(fmt.Errorf("InlineMenusType.Update: can't find menu '%s'", name))
	}
}

func (ims *InlineMenusType) RegisterMenu(bot *tele.Bot, menu *InlineMenu) error {
	if _, ok := ims.menus[menu.Name]; ok {
		return fmt.Errorf("InlineMenusType.RegisterMenu: menu '%s' already registerd", menu.Name)
	}
	menu.construct(bot)
	ims.menus[menu.Name] = menu
	return nil
}

func (ims *InlineMenusType) Show(c tele.Context, menuName string) error {
	menu, ok := ims.menus[menuName]
	if !ok {
		return fmt.Errorf("InlineMenusType.Show: menu %s is not registered", menuName)
	}
	setMessageID(c.Sender().ID, c.Message().ID+1) // current context - pressed ReplyMenu button, so next one - inline menu
	err := c.Send(menu.header, menu.bake(c))
	return err
}

///////////////////////////////////////////////////////////// CONCRETE InlineMenu implementation

// InlineMenuTextSetter is a type for dynamic content setter
type InlineMenuTextSetter func(tele.Context, map[string]string) (string, error)

// DataFetcher is a type for extracting dynamic content from database
type DataFetcher func(c tele.Context) (map[string]string, error)
type ButtonFetcher func(c tele.Context) (*om.OrderedMap[string, string], error)

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

// NewInlineMenu is a constructor for
func NewInlineMenu(menuName, menuHeader string, maxButtonsInRow int, fetcher DataFetcher) *InlineMenu {
	return &InlineMenu{
		Name:            menuName,
		header:          menuHeader,
		maxButtonsInRow: maxButtonsInRow,
		dataFetcher:     fetcher,
	}
}

func NewDynamicInlineMenu(menuName, menuHeader string, maxButtonsInRow int, fetcher ButtonFetcher) *InlineMenu {
	return &InlineMenu{
		Name:            menuName,
		header:          menuHeader,
		maxButtonsInRow: maxButtonsInRow,
		buttonFetcher:   fetcher,
	}
}

// UpdateButtons uses buttonFetcher to extract buttons from database. key - id, value - name
func (im *InlineMenu) UpdateButtons(c tele.Context) error {
	if im.buttonFetcher == nil {
		return nil
	}

	btnMap, err := im.buttonFetcher(c)
	if err != nil {
		return fmt.Errorf("UpdateButtons: %w", err)
	}

	im.PurgeButtons()
	for pair := btnMap.Oldest(); pair != nil; pair = pair.Next() {
		im.AddButton(&InlineButtonTemplate{
			Unique:         pair.Key,
			TextOnCreation: pair.Value,
			OnClick:        pair.Value,
		})
	}

	im.construct(c.Bot())

	return nil
}

func (im *InlineMenu) AddButtons(buttons []*InlineButtonTemplate) {
	im.PurgeButtons()
	for _, button := range buttons {
		im.AddButton(button)
	}
}

func (im *InlineMenu) PurgeButtons() {
	im.btnTemplates = make([]*InlineButtonTemplate, 0)
	im.textSetters = make(map[string]InlineMenuTextSetter)
}

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

func (im *InlineMenu) Update(c tele.Context, msgID string) {
	menu := im.bake(c)

	msg := tele.StoredMessage{
		MessageID: msgID,
		ChatID:    c.Chat().ID,
	}
	_, err := c.Bot().Edit(msg, im.header, menu)
	if err != nil {
		fmt.Println(fmt.Errorf("InlineMenu.Update[%d]: can't update menu '%s' to messageID %s: %w",
			c.Sender().ID, im.Name, msgID, err))
	}
}

func (im *InlineMenu) bake(c tele.Context) *tele.ReplyMarkup {
	if im.dataFetcher == nil {
		return im.menuCarcass
	}
	dynamicContentMap, err := im.dataFetcher(c)
	if err != nil {
		fmt.Println(fmt.Errorf("InlineMenu.bake[%d]:can't fetch data from database: %w", c.Sender().ID, err))
	}

	for i, row := range im.menuCarcass.InlineKeyboard {
		for j, btn := range row {
			f, ok := im.textSetters[btn.Unique]
			if !ok {
				continue // static info had been placed already
			}
			content, err := f(c, dynamicContentMap)
			if err != nil {
				fmt.Println(fmt.Errorf("InlineMenu.bake[%d]: can't change value for button: %w", c.Sender().ID, err))
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
		bakedButton = im.menuCarcass.Data(staticText, t, t)
	}
	return bakedButton
}

///////////////////////////////////////////////////////////// InlineButtonTemplate implementation

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
