package telegramw

import (
	"testing"

	"github.com/AndreeJait/go-utility/v2/botw"
)

// TestInterfaceCompliance ensures telegramBot implements the botw.Bot interface.
func TestInterfaceCompliance(t *testing.T) {
	var _ botw.Bot = (*telegramBot)(nil)
}

// TestParseChatID verifies that string chat IDs are correctly parsed to int64.
func TestParseChatID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{"Valid Positive ID", "123456789", 123456789},
		{"Valid Negative ID (Group Chat)", "-987654321", -987654321},
		{"Invalid ID (Letters)", "abc", 0},
		{"Empty ID", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseChatID(tt.input)
			if result != tt.expected {
				t.Errorf("parseChatID(%q) = %d; want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestNew_InvalidToken ensures that initializing a bot with a fake token fails gracefully
// instead of panicking.
func TestNew_InvalidToken(t *testing.T) {
	fakeToken := "12345:INVALID_TOKEN_FORMAT"

	bot, err := New(fakeToken)

	if err == nil {
		t.Error("Expected an error when initializing with an invalid token, got nil")
	}
	if bot != nil {
		t.Errorf("Expected bot instance to be nil, got %v", bot)
	}
}
