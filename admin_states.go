package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"vocal_training_bot/BotExt"

	tele "gopkg.in/telebot.v3"
)

const (
	AdminSGRecordMessage = "AdminSG_RecordMessage"
)

func SetupAdminStates() {
	err := adminFSM.RegisterOneShotState(&BotExt.State{
		Name: AdminSGRecordMessage,
		OnTrigger: `Начни писать одно или несколько сообщений. Когда закончишь - просто напиши слово 'СТОП' - и сообщение отправится всем пользователям.
Если надо отменить запись сообщений напиши 'ОТМЕНА'`,
		Manipulator: RecordMessage,
		OnSuccess:   "DONE!",
	})
	if err != nil {
		panic(err)
	}
}

func RecordMessage(c tele.Context) error {
	userID := c.Sender().ID
	recordID, ok := BotExt.GetStateVar(userID, "RecordID")
	if !ok {
		return fmt.Errorf("RecordMessage[%d]: no RecordID in database", userID)
	}

	if c.Text() == "СТОП" {
		return SendMessages(c.Bot(), recordID)
	}

	if c.Text() == "ОТМЕНА" {
		_, err := DB.Exec(context.Background(), `
		DELETE FROM messages 
		WHERE record_id = $1`, recordID)
		if err != nil {
			return fmt.Errorf("RecordMessage[%d]: cannot delete record, %w", userID, err)
		}
		return nil
	}

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
			fmt.Println(fmt.Errorf("RecordMessage[%d]: cannot unmarshal json, %w", userID, err))
		}

		fileName := "./message_storage/" + mediaFile.UniqueID
		checkOrCreateStorageFolder()
		if _, err := os.Stat(fileName); errors.Is(err, os.ErrNotExist) {
			err = c.Bot().Download(mediaFile, fileName)
			if err != nil {
				fmt.Println(fmt.Errorf("RecordMessage[%d]: cannot save message, %w", userID, err))
			}
		} else {
			fmt.Println("exists")
		}
	}

	_, err := DB.Exec(context.Background(), `
		INSERT INTO messages (record_id, message_id, chat_id, album_id, message_type, message_text, entity_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		recordID, messageID, chatID, albumID, mediaType, messageText, mediaJSON)
	if err != nil {
		return fmt.Errorf("RecordMessage[%d]: cannot update database, %w", userID, err)
	}
	return BotExt.ContinueState
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
		ORDER BY message_order`, recordID)

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
