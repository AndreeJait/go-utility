package discordw

import (
	"testing"

	"github.com/AndreeJait/go-utility/v2/botw"
)

// TestInterfaceCompliance ensures discordBot implements the botw.Bot interface.
func TestInterfaceCompliance(t *testing.T) {
	var _ botw.Bot = (*discordBot)(nil)
}

// TestNew_Initialization verifies the session creation logic doesn't panic
// and returns the proper struct even without an immediate connection.
func TestNew_Initialization(t *testing.T) {
	fakeToken := "fake.discord.token.123"

	bot, err := New(fakeToken)

	if err != nil {
		t.Errorf("Expected nil error for offline token initialization, got %v", err)
	}
	if bot == nil {
		t.Error("Expected a valid bot instance, got nil")
	}
}
