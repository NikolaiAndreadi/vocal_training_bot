package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"vocal_training_bot/BotExt"

	tele "gopkg.in/telebot.v3"
)

const (
	AdminSGRecordMessage     = "AdminSG_RecordMessage"
	AdminSGRecordCheerup     = "AdminSG_RecordCheerup"
	AdminSGAddGroupMenu      = "AdminSG_AddGroupMenu"
	AdminSGRenameWarmupGroup = "AdminSG_RenameWarmupGroup"

	AdminSGAddWarmup        = "AdminSG_AddWarmup"
	AdminSGWarmupSetName    = "AdminSG_WarmupSetName"
	AdminSGWarmupSetPrice   = "AdminSG_WarmupSetPrice"
	AdminSGWarmupSetContent = "AdminSG_WarmupSetContent"
)

func SetupAdminStates() {
	err := adminFSM.RegisterOneShotState(&BotExt.State{
		Name: AdminSGRecordMessage,
		OnTrigger: `Начни писать одно или несколько сообщений. Когда закончишь - просто напиши слово 'СТОП' - и сообщение отправится всем пользователям.
Если надо отменить запись сообщений напиши 'ОТМЕНА'`,
		Manipulator: RecordOneTimeMessage,
		OnSuccess:   "DONE!",
	})
	if err != nil {
		panic(err)
	}

	err = adminFSM.RegisterOneShotState(&BotExt.State{
		Name: AdminSGRecordCheerup,
		OnTrigger: `Начни писать одно или несколько сообщений. Когда закончишь - просто напиши слово 'СТОП', подбадривание будет сохранено.
Если надо отменить запись сообщений - напиши 'ОТМЕНА'`,
		Manipulator: RecordCheerup,
		OnSuccess:   "DONE!",
	})
	if err != nil {
		panic(err)
	}

	err = adminFSM.RegisterOneShotState(&BotExt.State{
		Name:        AdminSGAddGroupMenu,
		OnTrigger:   `Введи название группы, макс 50 символов. Для отмены напиши 'ОТМЕНА'`,
		Validator:   nameMax50Validator,
		Manipulator: AddWarmupGroup,
		OnSuccess:   "DONE!",
	})
	if err != nil {
		panic(err)
	}

	err = adminFSM.RegisterOneShotState(&BotExt.State{
		Name:      AdminSGRenameWarmupGroup,
		OnTrigger: `Введи новое название группы, макс 50 символов. Для отмены напиши 'ОТМЕНА'`,
		Validator: func(c tele.Context) string {
			if len(c.Text()) >= 50 {
				return "Название группы слишком длинное!"
			}
			return ""
		},
		Manipulator: RenameWarmupGroup,
		OnSuccess:   "DONE!",
	})
	if err != nil {
		panic(err)
	}

	err = adminFSM.RegisterStateChain([]*BotExt.State{
		{
			Name:           AdminSGAddWarmup,
			OnTrigger:      "В какую группу поместить распевку?",
			OnTriggerExtra: []interface{}{warmupGroupAdminMenu},
			Validator: func(c tele.Context) string {
				_, ok := BotExt.GetStateVar(c.Sender().ID, "selectedWarmupGroup")
				if !ok {
					return "Выбери группу из списка!"
				}
				return ""
			},
		},
		{
			Name:      AdminSGWarmupSetName,
			OnTrigger: "Как будет называться распевка? Макс 50 символов",
			Validator: nameMax50Validator,
			Manipulator: func(c tele.Context) error {
				BotExt.SetStateVar(c.Sender().ID, "WarmupName", c.Text())
				return nil
			},
		},
		{
			Name:      AdminSGWarmupSetPrice,
			OnTrigger: "Сколько будет стоить распевка? Цена в рублях, без копеек!",
			Validator: priceValidator,
			Manipulator: func(c tele.Context) error {
				BotExt.SetStateVar(c.Sender().ID, "WarmupPrice", c.Text())
				return nil
			},
		},
		{
			Name:        AdminSGWarmupSetContent,
			OnTrigger:   "Напиши содержание распевки. Как закончишь - напиши СТОП",
			Manipulator: RecordWarmup,
			OnSuccess:   "Успех! Распевка сохранена!",
		},
	})
	if err != nil {
		panic(err)
	}
}

func nameMax50Validator(c tele.Context) string {
	if len(c.Text()) >= 50 {
		return "Название группы слишком длинное!"
	}
	return ""
}

func priceValidator(c tele.Context) string {
	price, err := strconv.Atoi(c.Text())
	if err != nil {
		return "Тут должно быть неотрицательное число!"
	}
	if price < 0 {
		return "Тут должно быть неотрицательное число!"
	}
	return ""
}

func AddWarmupGroup(c tele.Context) error {
	if strings.ToLower(c.Text()) == "отмена" {
		return nil
	}

	_, err := DB.Exec(context.Background(), `
	INSERT INTO warmup_groups (group_name)
	VALUES ($1)`, c.Text())
	if err != nil {
		return fmt.Errorf("AddWarmupGroup: %w", err)
	}
	return nil
}

func RenameWarmupGroup(c tele.Context) error {
	if strings.ToLower(c.Text()) == "отмена" {
		return nil
	}

	groupID, ok := BotExt.GetStateVar(c.Sender().ID, "selectedWarmupGroup")
	if !ok {
		return fmt.Errorf("RenameWarmupGroup: can't find state var selectedWarmupGroup")
	}
	_, err := DB.Exec(context.Background(), `
	UPDATE warmup_groups
	SET group_name = $1
	WHERE warmup_group_id = $2
	`, c.Text(), groupID)
	if err != nil {
		return fmt.Errorf("AddWarmupGroup: %w", err)
	}
	return nil
}

func RecordCheerup(c tele.Context) error {
	userID := c.Sender().ID
	recordID, ok := BotExt.GetStateVar(userID, "RecordID")
	if !ok {
		return fmt.Errorf("RecordCheerup[%d]: no RecordID in database", userID)
	}

	if c.Text() == "СТОП" {
		_, err := DB.Exec(context.Background(), `
		INSERT INTO warmup_cheerups (record_id)
		VALUES ($1 :: uuid)`, recordID)
		if err != nil {
			return fmt.Errorf("RecordCheerup[%d]: cannot update database, %w", userID, err)
		}
		return nil
	}

	if c.Text() == "ОТМЕНА" {
		_, err := DB.Exec(context.Background(), `
		DELETE FROM messages 
		WHERE record_id = $1`, recordID)
		if err != nil {
			return fmt.Errorf("RecordCheerup[%d]: cannot delete record, %w", userID, err)
		}
		return nil
	}

	err := saveMessageToDBandDisk(c, userID, recordID)
	if err != nil {
		return fmt.Errorf("RecordCheerup: %w", err)
	}

	return BotExt.ContinueState
}

func RecordWarmup(c tele.Context) error {
	userID := c.Sender().ID
	recordID, ok := BotExt.GetStateVar(userID, "RecordID")
	if !ok {
		return fmt.Errorf("RecordWarmup[%d]: no RecordID in database", userID)
	}

	if c.Text() == "СТОП" {
		values := BotExt.GetStateVars(userID)
		warmupGroup, ok := values["selectedWarmupGroup"]
		if !ok {
			return fmt.Errorf("RecordWarmup[%d]: can't fetch selectedWarmupGroup", userID)
		}
		warmupName, ok := values["WarmupName"]
		if !ok {
			return fmt.Errorf("RecordWarmup[%d]: can't fetch WarmupName", userID)
		}
		warmupPrice, ok := values["WarmupPrice"]
		if !ok {
			return fmt.Errorf("RecordWarmup[%d]: can't fetch WarmupPrice", userID)
		}
		_, err := DB.Exec(context.Background(), `
		INSERT INTO warmups (warmup_group, warmup_name, price, record_id)
		VALUES ($1::int, $2, $3::int2, $4::uuid)`, warmupGroup, warmupName, warmupPrice, recordID)
		if err != nil {
			return fmt.Errorf("RecordWarmup[%d]: cannot update database, %w", userID, err)
		}
		return nil
	}

	if c.Text() == "ОТМЕНА" {
		_, err := DB.Exec(context.Background(), `
		DELETE FROM messages 
		WHERE record_id = $1`, recordID)
		if err != nil {
			return fmt.Errorf("RecordWarmup[%d]: cannot delete record, %w", userID, err)
		}
		return nil
	}

	err := saveMessageToDBandDisk(c, userID, recordID)
	if err != nil {
		return fmt.Errorf("RecordWarmup: %w", err)
	}

	return BotExt.ContinueState
}

func RecordOneTimeMessage(c tele.Context) error {
	userID := c.Sender().ID
	recordID, ok := BotExt.GetStateVar(userID, "RecordID")
	if !ok {
		return fmt.Errorf("RecordOneTimeMessage[%d]: no RecordID in database", userID)
	}

	if c.Text() == "СТОП" {
		if err := c.Send("Отправка сообщений..."); err != nil {
			fmt.Println(fmt.Errorf("RecordOneTimeMessage[%d]: can't send message: %w", userID, err))
		}
		return SendMessages(c.Bot(), recordID)
	}

	if c.Text() == "ОТМЕНА" {
		_, err := DB.Exec(context.Background(), `
		DELETE FROM messages 
		WHERE record_id = $1`, recordID)
		if err != nil {
			return fmt.Errorf("RecordOneTimeMessage[%d]: cannot delete record, %w", userID, err)
		}
		return nil
	}

	err := saveMessageToDBandDisk(c, userID, recordID)
	if err != nil {
		return fmt.Errorf("RecordOneTimeMessage: %w", err)
	}
	return BotExt.ContinueState
}

func saveMessageToDBandDisk(c tele.Context, userID int64, recordID string) error {
	msg := c.Message()
	messageID, chatID, albumID := strconv.Itoa(msg.ID), msg.Chat.ID, msg.AlbumID

	var messageText string
	switch {
	case msg.Text != "":
		messageText = msg.Text
	case msg.Caption != "":
		messageText = msg.Caption
	}

	media := msg.Media()
	mediaType := "text"
	var mediaJSON []byte
	if media != nil {
		mediaType = media.MediaType()

		mediaFile := media.MediaFile()
		mediaJSONTmp, err := json.Marshal(mediaFile)
		mediaJSON = mediaJSONTmp
		if err != nil {
			fmt.Println(fmt.Errorf("saveMessageToDBandDisk[%d]: cannot unmarshal json, %w", userID, err))
		}

		fileName := "./message_storage/" + mediaFile.UniqueID
		checkOrCreateStorageFolder()
		if _, err := os.Stat(fileName); errors.Is(err, os.ErrNotExist) {
			err = c.Bot().Download(mediaFile, fileName)
			if err != nil {
				fmt.Println(fmt.Errorf("saveMessageToDBandDisk[%d]: cannot save message, %w", userID, err))
			}
		} else {
			fmt.Println("exists")
		}
	}

	_, err := DB.Exec(context.Background(), `
		INSERT INTO messages (record_id, message_id, chat_id, album_id, message_type, message_text, entity_json)
		VALUES ($1 :: uuid, $2, $3, $4, $5, $6, $7)`,
		recordID, messageID, chatID, albumID, mediaType, messageText, mediaJSON)
	if err != nil {
		return fmt.Errorf("saveMessageToDBandDisk[%d]: cannot update database, %w", userID, err)
	}
	return nil
}

func checkOrCreateStorageFolder() {
	path := "./message_storage"
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			fmt.Println(fmt.Errorf("checkOrCreateStorageFolder: %w", err))
		}
	}
}

type message struct {
	messageID string
	chatID    int64
	albumID   string
}

func (m message) MessageSig() (messageID string, chatID int64) {
	return m.messageID, m.chatID
}

func SendMessages(b *tele.Bot, recordID string) error {
	rows, err := DB.Query(context.Background(), `
		SELECT message_id, chat_id, album_id from messages
		WHERE record_id = $1
		ORDER BY message_id`, recordID)

	defer rows.Close()
	if err != nil {
		return fmt.Errorf("SendMessages[recordID = %s]: pg messages query error %w", recordID, err)
	}

	var BakedMessage []message
	var lastAlbum string
	var msg message

	for rows.Next() {
		err := rows.Scan(&msg.messageID, &msg.chatID, &msg.albumID)
		if err != nil {
			return fmt.Errorf("SendMessages[recordID = %s]: messages row scan error %w", recordID, err)
		}
		if msg.albumID == "" {
			BakedMessage = append(BakedMessage, msg)
			continue
		}
		if msg.albumID == lastAlbum {
			continue
		}
		BakedMessage = append(BakedMessage, msg)
	}

	rows, err = DB.Query(context.Background(), `
		SELECT user_id from users
		WHERE user_class = 'USER'`)
	defer rows.Close()
	if err != nil {
		return fmt.Errorf("SendMessages[recordID = %s]: pg query error %w", recordID, err)
	}

	var userID int64
	for rows.Next() {
		err := rows.Scan(&userID)
		if err != nil {
			return fmt.Errorf("SendMessages[recordID = %s]: users row scan error %w", recordID, err)
		}
		for _, bm := range BakedMessage {
			_, err = b.Copy(UserIDType{userID}, bm)
			if err != nil {
				fmt.Println(fmt.Errorf("SendMessages[recordID = %s]: sending message error for user [%d]: %w",
					recordID, userID, err))
			}
		}
	}
	return nil
}

func SendMessageToUser(b *tele.Bot, userID int64, recordID string) error {
	rows, err := DB.Query(context.Background(), `
		SELECT message_id, chat_id, album_id from messages
		WHERE record_id = $1
		ORDER BY message_id`, recordID)

	defer rows.Close()
	if err != nil {
		return fmt.Errorf("SendMessageToUser[recordID = %s]: pg messages query error %w", recordID, err)
	}

	var BakedMessage []message
	var lastAlbum string
	var msg message

	for rows.Next() {
		err := rows.Scan(&msg.messageID, &msg.chatID, &msg.albumID)
		if err != nil {
			return fmt.Errorf("SendMessageToUser[recordID = %s]: messages row scan error %w", recordID, err)
		}
		if msg.albumID == "" {
			BakedMessage = append(BakedMessage, msg)
			continue
		}
		if msg.albumID == lastAlbum {
			continue
		}
		BakedMessage = append(BakedMessage, msg)
	}

	for _, bm := range BakedMessage {
		_, err = b.Copy(UserIDType{userID}, bm)
		if err != nil {
			return fmt.Errorf("SendMessages[recordID = %s]: sending message error for user [%d]: %w",
				recordID, userID, err)
		}
	}
	return nil
}
