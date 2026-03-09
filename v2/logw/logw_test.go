package logw

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestContextLogID verifies the injection and extraction of x-log-id.
func TestContextLogID(t *testing.T) {
	ctx := context.Background()

	// 1. Should be empty initially
	if id := GetLogID(ctx); id != "" {
		t.Errorf("expected empty log ID, got %s", id)
	}

	// 2. Should inject a valid UUID
	ctxWithID := InjectLogID(ctx)
	logID := GetLogID(ctxWithID)
	if logID == "" {
		t.Errorf("expected a generated log ID, got empty string")
	}

	// 3. Should not overwrite an existing ID
	ctxWithSameID := InjectLogID(ctxWithID)
	if newID := GetLogID(ctxWithSameID); newID != logID {
		t.Errorf("expected log ID to remain %s, got %s", logID, newID)
	}
}

// TestLoggerOutput verifies that the logger formats and outputs data correctly.
func TestLoggerOutput(t *testing.T) {
	// We use a bytes.Buffer to capture the output in-memory
	var buf bytes.Buffer

	cfg := &LogConfig{
		Level:        "debug",
		Format:       FormatJSON,
		SendToBroker: true,
		BrokerWriter: &buf, // Injecting buffer to intercept logs
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("failed to initialize logger: %v", err)
	}

	t.Run("Log without Context", func(t *testing.T) {
		msg := "system is starting"
		Info(msg)

		output := buf.String()
		if !strings.Contains(output, msg) {
			t.Errorf("expected log to contain %q, but got: %s", msg, output)
		}

		// Reset buffer for the next test
		buf.Reset()
	})

	t.Run("Log with Context and LogID", func(t *testing.T) {
		ctx := InjectLogID(context.Background())
		logID := GetLogID(ctx)
		msg := "processing request"

		CtxInfof(ctx, "status: %s", msg)

		output := buf.String()
		if !strings.Contains(output, msg) {
			t.Errorf("expected log to contain %q, but got: %s", msg, output)
		}
		if !strings.Contains(output, "x-log-id") {
			t.Errorf("expected log to contain 'x-log-id' key, but got: %s", output)
		}
		if !strings.Contains(output, logID) {
			t.Errorf("expected log to contain the specific logID %q, but got: %s", logID, output)
		}

		buf.Reset()
	})
}
