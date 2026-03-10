// Package discordw implements the botw.Bot interface for Discord using the discordgo SDK.
package discordw

import (
	"bytes"
	"context"
	"fmt"

	"github.com/AndreeJait/go-utility/v2/botw"
	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/bwmarrin/discordgo"
)

// discordBot implements the botw.Bot interface.
type discordBot struct {
	session *discordgo.Session
}

// New initializes a new Discord bot connection.
// It automatically configures the required Discord Intents to read message content.
func New(token string) (botw.Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("discordw: failed to create session: %w", err)
	}

	// Request intents required to read message content and observe channels
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentMessageContent

	return &discordBot{session: dg}, nil
}

func (d *discordBot) SendMessage(ctx context.Context, chatID, text string) error {
	_, err := d.session.ChannelMessageSend(chatID, text)
	return err
}

func (d *discordBot) SendReply(ctx context.Context, chatID, replyToID, text string) error {
	_, err := d.session.ChannelMessageSendReply(chatID, text, &discordgo.MessageReference{
		MessageID: replyToID,
		ChannelID: chatID,
	})
	return err
}

func (d *discordBot) SendFile(ctx context.Context, chatID, filename string, payload []byte) error {
	_, err := d.session.ChannelFileSend(chatID, filename, bytes.NewReader(payload))
	return err
}

func (d *discordBot) SendReplyWithFile(ctx context.Context, chatID, replyToID, filename string, payload []byte) error {
	msgSend := &discordgo.MessageSend{
		Reference: &discordgo.MessageReference{
			MessageID: replyToID,
			ChannelID: chatID,
		},
		Files: []*discordgo.File{
			{
				Name:   filename,
				Reader: bytes.NewReader(payload),
			},
		},
	}
	_, err := d.session.ChannelMessageSendComplex(chatID, msgSend)
	return err
}

func (d *discordBot) SendTyping(ctx context.Context, chatID string) error {
	return d.session.ChannelTyping(chatID)
}

func (d *discordBot) Listen(ctx context.Context, handler botw.Handler) error {
	d.session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		// Ignore messages originating from the bot itself
		if m.Author.ID == s.State.User.ID {
			return
		}

		// Parse incoming files/attachments
		var files []botw.File
		for _, att := range m.Attachments {
			files = append(files, botw.File{
				ID:       att.ID,
				Filename: att.Filename,
				URL:      att.URL, // Discord provides a direct CDN URL natively
				Size:     int64(att.Size),
				MimeType: att.ContentType,
			})
		}

		stdMsg := &botw.Message{
			ID:       m.ID,
			ChatID:   m.ChannelID,
			SenderID: m.Author.ID,
			Username: m.Author.Username,
			Text:     m.Content,
			Files:    files,
		}

		// Process the message asynchronously
		go func() {
			if err := handler(ctx, stdMsg); err != nil {
				logw.CtxErrorf(ctx, "discordw: handler failed for message %s: %v", m.ID, err)
			}
		}()
	})

	if err := d.session.Open(); err != nil {
		return fmt.Errorf("discordw: failed to open connection: %w", err)
	}
	logw.Info("Discord bot connected and listening")

	<-ctx.Done()
	logw.Info("Context canceled, stopping Discord bot")
	return d.Close()
}

func (d *discordBot) Close() error {
	return d.session.Close()
}
