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

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

const (
	AdminSGRecordMessage     = "AdminSG_RecordMessage"
	AdminSGRecordCheerup     = "AdminSG_RecordCheerup"
	AdminSGAddGroupMenu      = "AdminSG_AddGroupMenu"
	AdminSGRenameWarmupGroup = "AdminSG_RenameWarmupGroup"

	AdminSGAddWarmup        = "AdminSG_AddWarmup"
	AdminSGWarmupSetName    = "AdminSG_WarmupSetName"
	AdminSGWarmupSetContent = "AdminSG_WarmupSetContent"

	ChangeWarmupSetGroup = "changeWarmupSetGroup"
	ChangeWarmupSetName  = "ChangeWarmupSetName"
)

const storageFolder = "./message_storage/"

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

	err = adminFSM.RegisterOneShotState(&BotExt.State{
		Name:           ChangeWarmupSetGroup,
		OnTrigger:      "В какую группу поместить распевку?",
		OnTriggerExtra: []interface{}{warmupGroupAdminMenu},
		KeepVarsOnQuit: true,
		Validator: func(c tele.Context) string {
			_, ok := BotExt.GetStateVar(c.Sender().ID, "selectedWarmupGroup")
			if !ok {
				return "Выбери группу из списка!"
			}
			return ""
		},
		Manipulator: func(c tele.Context) error {
			warmupGroup, _ := BotExt.GetStateVar(c.Sender().ID, "selectedWarmupGroup")
			warmupID, _ := BotExt.GetStateVar(c.Sender().ID, "selectedWarmup")
			_, err = DB.Exec(context.Background(), `
				UPDATE warmups
				SET warmup_group = $1
				WHERE warmup_id = $2
			`, warmupGroup, warmupID)
			return err
		},
		OnSuccess: "Готово!",
	})

	err = adminFSM.RegisterOneShotState(&BotExt.State{
		Name:           ChangeWarmupSetName,
		OnTrigger:      "Новое имя?",
		KeepVarsOnQuit: true,
		OnSuccess:      "Done!",
		Validator:      nameMax50Validator,
		Manipulator: func(c tele.Context) error {
			warmupID, _ := BotExt.GetStateVar(c.Sender().ID, "selectedWarmup")
			_, err = DB.Exec(context.Background(), `
				UPDATE warmups
				SET warmup_name = $1
				WHERE warmup_id = $2
			`, c.Text(), warmupID)
			return err
		},
	})

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
		return fmt.Errorf("RecordCheerup: no RecordID in database")
	}

	if strings.ToLower(c.Text()) == "стоп" {
		_, err := DB.Exec(context.Background(), `
		INSERT INTO warmup_cheerups (record_id)
		VALUES ($1 :: uuid)`, recordID)
		if err != nil {
			return fmt.Errorf("RecordCheerup: cannot update database, %w", err)
		}
		return nil
	}

	if strings.ToLower(c.Text()) == "отмена" {
		_, err := DB.Exec(context.Background(), `
		DELETE FROM messages 
		WHERE record_id = $1`, recordID)
		if err != nil {
			return fmt.Errorf("RecordCheerup: cannot delete record, %w", err)
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
		return fmt.Errorf("RecordWarmup: no RecordID in database")
	}

	if strings.ToLower(c.Text()) == "стоп" {
		values := BotExt.GetStateVars(userID)
		warmupGroup, ok := values["selectedWarmupGroup"]
		if !ok {
			return fmt.Errorf("RecordWarmup: can't fetch selectedWarmupGroup")
		}
		warmupName, ok := values["WarmupName"]
		if !ok {
			return fmt.Errorf("RecordWarmup: can't fetch WarmupName")
		}

		_, err := DB.Exec(context.Background(), `
		INSERT INTO warmups (warmup_group, warmup_name, record_id)
		VALUES ($1::int, $2, $3::uuid)`, warmupGroup, warmupName, recordID)
		if err != nil {
			return fmt.Errorf("RecordWarmup: cannot update database, %w", err)
		}
		return nil
	}

	if strings.ToLower(c.Text()) == "отмена" {
		_, err := DB.Exec(context.Background(), `
		DELETE FROM messages 
		WHERE record_id = $1`, recordID)
		if err != nil {
			return fmt.Errorf("RecordWarmup: cannot delete record, %w", err)
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
		return fmt.Errorf("RecordOneTimeMessage: no RecordID in database")
	}

	if strings.ToLower(c.Text()) == "стоп" {
		if err := c.Send("Отправка сообщений..."); err != nil {
			logger.Error("can't send message", zap.Int64("user", userID), zap.Error(err))
		}

		err := SendMessages(c.Bot(), recordID)
		if err != nil {
			logger.Error("can't send messages", zap.String("recordID", recordID), zap.Error(err))
		}

		// delete because onetime
		// TODO: clear lost messages from time to time (that are not cheerup or warmup)
		_, err = DB.Exec(context.Background(), `
		DELETE FROM messages 
		WHERE record_id = $1`, recordID)
		if err != nil {
			return fmt.Errorf("RecordOneTimeMessage: cannot delete record, %w", err)
		}
		return nil
	}

	if strings.ToLower(c.Text()) == "отмена" {
		_, err := DB.Exec(context.Background(), `
		DELETE FROM messages 
		WHERE record_id = $1`, recordID)
		if err != nil {
			return fmt.Errorf("RecordOneTimeMessage: cannot delete record, %w", err)
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
	var strMediaJSON string
	var err error
	if media != nil {
		mediaType = media.MediaType()
		mediaFile := media.MediaFile()
		mediaJSON, err = json.Marshal(mediaFile)
		if err != nil {
			logger.Error("can't unmarshal json", zap.Int64("user", userID), zap.Error(err))
		}
		strMediaJSON = string(mediaJSON)
		if strMediaJSON == "" {
			strMediaJSON = "{}"
		}

		fileName := storageFolder + mediaFile.UniqueID
		checkOrCreateStorageFolder()
		if _, err := os.Stat(fileName); errors.Is(err, os.ErrNotExist) {
			err = c.Bot().Download(mediaFile, fileName)
			if err != nil {
				logger.Error("can't save message", zap.Int64("user", userID), zap.Error(err))
			}
		} else {
			logger.Warn("file exists", zap.Int64("user", userID),
				zap.String("fileUniqueID", mediaFile.UniqueID), zap.Error(err))
		}
	}

	_, err = DB.Exec(context.Background(), `
		INSERT INTO messages (record_id, message_id, chat_id, album_id, message_type, message_text, entity_json)
		VALUES ($1 :: uuid, $2, $3, $4, $5, $6, $7)`,
		recordID, messageID, chatID, albumID, mediaType, messageText, strMediaJSON)
	if err != nil {
		return fmt.Errorf("saveMessageToDBandDisk: cannot update database, %w", err)
	}
	return nil
}

func checkOrCreateStorageFolder() {
	path := "./message_storage"
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			logger.Error("", zap.Error(err))
		}
	}
}

type message struct {
	messageID string
	chatID    int64
	albumID   string

	Type string
	Text string
	Json string
}

func (m message) MessageSig() (messageID string, chatID int64) {
	return m.messageID, m.chatID
}

func (m message) HasFile() bool {
	if (m.Type != "text") && m.Json != "{}" {
		return true
	}
	return false
}

func SendMessages(b *tele.Bot, recordID string) error {
	BakedMessage, err := bakeMessage(recordID)
	if err != nil {
		return fmt.Errorf("SendMessages: %w", err)
	}

	rows, err := DB.Query(context.Background(), `
		SELECT user_id from users
		WHERE user_class = 'USER'`)
	if err != nil {
		return fmt.Errorf("SendMessages[recordID = %s]: pg query error %w", recordID, err)
	}
	defer rows.Close()

	var userID int64
	for rows.Next() {
		err := rows.Scan(&userID)
		if err != nil {
			return fmt.Errorf("SendMessages[recordID = %s]: users row scan error %w", recordID, err)
		}
		user := UserIDType{userID}
		for _, bm := range BakedMessage {
			_, err = b.Copy(UserIDType{userID}, bm)
			if err != nil {
				if err.Error() == "telegram: Bad Request: message to copy not found (400)" {
					err = sendFromDatabase(b, user, &bm, false)
				}
				logger.Error("can't SendMessages to specific user", zap.Error(err),
					zap.Int64("userID", userID), zap.String("recordID", recordID))
			}
		}
	}
	return nil
}

func SendMessageToUser(b *tele.Bot, userID int64, recordID string, secured bool) error {
	BakedMessage, err := bakeMessage(recordID)
	if err != nil {
		return fmt.Errorf("SendMessageToUser: %w", err)
	}

	user := UserIDType{userID}
	for _, bm := range BakedMessage {
		if secured {
			_, err = b.Copy(user, bm, tele.Protected)
		} else {
			_, err = b.Copy(user, bm)
		}

		if err != nil {
			if err.Error() == "telegram: Bad Request: message to copy not found (400)" {
				err = sendFromDatabase(b, user, &bm, secured)
			} else {
				return fmt.Errorf("SendMessageToUser[recordID = %s]: sending message error for user [%d]: %w",
					recordID, userID, err)
			}
		}
	}
	return nil
}

func bakeMessage(recordID string) ([]message, error) {
	rows, err := DB.Query(context.Background(), `
		SELECT message_id, chat_id, album_id, message_type, message_text, entity_json from messages
		WHERE record_id = $1
		ORDER BY message_id`, recordID)

	defer rows.Close()
	if err != nil {
		return nil, fmt.Errorf("bakeMessage[recordID = %s]: pg messages query error %w", recordID, err)
	}

	var BakedMessage []message
	var lastAlbum string
	var msg message

	for rows.Next() {
		err := rows.Scan(&msg.messageID, &msg.chatID, &msg.albumID, &msg.Type, &msg.Text, &msg.Json)
		if err != nil {
			return nil, fmt.Errorf("bakeMessage[recordID = %s]: messages row scan error %w", recordID, err)
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

	if len(BakedMessage) == 0 {
		return nil, fmt.Errorf("bakeMessage: can't find record %s", recordID)
	}
	return BakedMessage, nil
}

func sendFromDatabase(b *tele.Bot, user tele.Recipient, bm *message, secured bool) (err error) {
	if !bm.HasFile() {
		_, err := b.Send(user, bm.Text)
		return err
	}
	var file tele.File
	err = json.Unmarshal([]byte(bm.Json), &file)
	if err != nil {
		return err
	}

	var loadedFromDisk bool
	if file.FilePath == "" {
		// if no filepath -> find it on telegram server
		file, err = b.FileByID(file.FileID)
		if err != nil {
			err = nil
			// or get it from local storage
			file = tele.FromDisk(storageFolder + file.UniqueID)
			loadedFromDisk = true
		}
	}

	sendable := createSendable(bm, &file)

	if secured {
		_, err = b.Send(user, sendable, tele.Protected)
	} else {
		_, err = b.Send(user, sendable)
	}
	if (err != nil) && !loadedFromDisk {
		file = tele.FromDisk(storageFolder + file.UniqueID)
		sendable = createSendable(bm, &file)
		var err2 error
		if secured {
			_, err2 = b.Send(user, sendable, tele.Protected)
		} else {
			_, err2 = b.Send(user, sendable)
		}
		if err2 != nil {
			return fmt.Errorf("can't send neither cached and local file '%s': %s - >%w", file.UniqueID, err.Error(), err2)
		}
		logger.Warn("sent local file successfully, but couldn't send cached file",
			zap.String("fileUniqueID", file.UniqueID), zap.Error(err))
	}
	logger.Warn("sent cached file successfully",
		zap.String("fileUniqueID", file.UniqueID), zap.Error(err))
	return nil
}

func createSendable(bm *message, file *tele.File) tele.Sendable {
	var sendable tele.Sendable
	switch bm.Type {
	case "photo":
		sendable = &tele.Photo{File: *file, Caption: bm.Text}
	case "audio":
		sendable = &tele.Audio{File: *file, Caption: bm.Text}
	case "document":
		sendable = &tele.Document{File: *file, Caption: bm.Text}
	case "video":
		sendable = &tele.Video{File: *file, Caption: bm.Text}
	case "voice":
		sendable = &tele.Voice{File: *file, Caption: bm.Text}
	case "videoNote":
		sendable = &tele.VideoNote{File: *file}
	case "sticker":
		sendable = &tele.Sticker{File: *file}
	}
	return sendable
}
