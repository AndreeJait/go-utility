// Package telegramw implements the botw.Bot interface for Telegram using Long-Polling.
package telegramw

import (
	"context"
	"fmt"
	"strconv"

	"github.com/AndreeJait/go-utility/v2/botw"
	"github.com/AndreeJait/go-utility/v2/logw"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// telegramBot implements the botw.Bot interface.
type telegramBot struct {
	api *tgbotapi.BotAPI
}

// New initializes a new Telegram bot using the standard HTTP API token.
func New(token string) (botw.Bot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("telegramw: failed to create bot: %w", err)
	}
	return &telegramBot{api: bot}, nil
}

// parseChatID safely converts string IDs to int64.
func parseChatID(chatID string) int64 {
	id, _ := strconv.ParseInt(chatID, 10, 64)
	return id
}

func (t *telegramBot) SendMessage(ctx context.Context, chatID, text string) error {
	msg := tgbotapi.NewMessage(parseChatID(chatID), text)
	_, err := t.api.Send(msg)
	return err
}

func (t *telegramBot) SendReply(ctx context.Context, chatID, replyToID, text string) error {
	msg := tgbotapi.NewMessage(parseChatID(chatID), text)
	msg.ReplyToMessageID, _ = strconv.Atoi(replyToID)
	_, err := t.api.Send(msg)
	return err
}

func (t *telegramBot) SendFile(ctx context.Context, chatID, filename string, payload []byte) error {
	file := tgbotapi.FileBytes{Name: filename, Bytes: payload}
	msg := tgbotapi.NewDocument(parseChatID(chatID), file)
	_, err := t.api.Send(msg)
	return err
}

func (t *telegramBot) SendReplyWithFile(ctx context.Context, chatID, replyToID, filename string, payload []byte) error {
	file := tgbotapi.FileBytes{Name: filename, Bytes: payload}
	msg := tgbotapi.NewDocument(parseChatID(chatID), file)
	msg.ReplyToMessageID, _ = strconv.Atoi(replyToID)

	_, err := t.api.Send(msg)
	return err
}

func (t *telegramBot) SendTyping(ctx context.Context, chatID string) error {
	action := tgbotapi.NewChatAction(parseChatID(chatID), tgbotapi.ChatTyping)
	_, err := t.api.Send(action)
	return err
}

func (t *telegramBot) Listen(ctx context.Context, handler botw.Handler) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := t.api.GetUpdatesChan(u)
	logw.Info("Telegram bot connected and listening via long-polling")

	for {
		select {
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if update.Message == nil {
				continue
			}

			// Parse incoming files (Document or Photo)
			var files []botw.File
			var fileID, fileName, mimeType string

			if update.Message.Document != nil {
				fileID = update.Message.Document.FileID
				fileName = update.Message.Document.FileName
				mimeType = update.Message.Document.MimeType
			} else if len(update.Message.Photo) > 0 {
				// Telegram sends multiple photo sizes; grab the highest resolution (last item)
				photo := update.Message.Photo[len(update.Message.Photo)-1]
				fileID = photo.FileID
				fileName = "photo.jpg"
				mimeType = "image/jpeg"
			}

			// Fetch direct download URL if a file is present
			if fileID != "" {
				fileURL, err := t.api.GetFileDirectURL(fileID)
				if err == nil {
					files = append(files, botw.File{
						ID:       fileID,
						Filename: fileName,
						URL:      fileURL,
						MimeType: mimeType,
					})
				} else {
					logw.Errorf("telegramw: failed to fetch file URL for ID %s: %v", fileID, err)
				}
			}

			stdMsg := &botw.Message{
				ID:       strconv.Itoa(update.Message.MessageID),
				ChatID:   strconv.FormatInt(update.Message.Chat.ID, 10),
				SenderID: strconv.FormatInt(update.Message.From.ID, 10),
				Username: update.Message.From.UserName,
				Text:     update.Message.Text,
				Files:    files,
			}

			go func() {
				if err := handler(ctx, stdMsg); err != nil {
					logw.CtxErrorf(ctx, "telegramw: handler failed for message %s: %v", stdMsg.ID, err)
				}
			}()

		case <-ctx.Done():
			logw.Info("Context canceled, stopping Telegram bot")
			t.api.StopReceivingUpdates()
			return nil
		}
	}
}

func (t *telegramBot) Close() error {
	t.api.StopReceivingUpdates()
	return nil
}
