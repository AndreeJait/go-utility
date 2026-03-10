// Package botw provides a unified, platform-agnostic interface for chat bots.
// It abstracts the complexities of different platforms (Discord, Telegram),
// allowing your business logic to remain decoupled from underlying SDKs.
package botw

import "context"

// File represents an attachment received in a chat message.
// To prevent memory overflow (OOM) from large concurrent uploads, this struct
// provides a Direct Download URL instead of loading raw bytes into RAM.
// Handlers can decide whether to stream this URL to storage (e.g., S3) or ignore it.
type File struct {
	ID       string // Unique file ID from the chat platform
	Filename string // Name of the file (if available)
	URL      string // Direct download URL (valid temporarily depending on the platform)
	Size     int64  // File size in bytes
	MimeType string // Content type (e.g., "image/png", "application/pdf")
}

// Message represents a standardized incoming chat message.
// Internal usecases will exclusively interact with this unified struct.
type Message struct {
	ID       string // The unique identifier of the message
	ChatID   string // The channel ID or Chat ID where the message was sent
	SenderID string // The unique user ID of the sender
	Username string // The display name or username of the sender
	Text     string // The raw text content of the message
	Files    []File // Array of attached files (Images, Documents, Videos, etc.)
}

// Handler defines the function signature for processing incoming messages.
// It receives a unified Message struct and a context for cancellation management.
type Handler func(ctx context.Context, msg *Message) error

// Bot defines the strict contract that all chat platforms must implement.
type Bot interface {
	// SendMessage delivers a standard text message to a specific chat or channel.
	SendMessage(ctx context.Context, chatID, text string) error

	// SendReply replies directly to a specific message ID within a chat.
	// This maintains conversational context in busy groups.
	SendReply(ctx context.Context, chatID, replyToID, text string) error

	// SendFile uploads and sends a file (document, image, pdf) to the chat.
	SendFile(ctx context.Context, chatID, filename string, payload []byte) error

	// SendReplyWithFile replies to a specific message ID and attaches a file.
	// Highly effective for returning user-requested reports or generated AI media.
	SendReplyWithFile(ctx context.Context, chatID, replyToID, filename string, payload []byte) error

	// SendTyping triggers the "Bot is typing..." or "Bot is uploading..." indicator.
	// Call this before executing long-running tasks to improve user experience.
	SendTyping(ctx context.Context, chatID string) error

	// Listen starts the bot's event loop. It blocks the current goroutine, receives
	// incoming messages, parses attachments, and passes them to your provided Handler.
	Listen(ctx context.Context, handler Handler) error

	// Close disconnects the bot gracefully, terminating active listeners.
	Close() error
}
