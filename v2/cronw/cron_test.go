package cronw

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestInterfaceCompliance ensures that the internal cronScheduler struct
// strictly implements the public Scheduler interface.
func TestInterfaceCompliance(t *testing.T) {
	var _ Scheduler = (*cronScheduler)(nil)
}

// TestExecuteHandlers verifies that our middleware chaining logic works perfectly.
// It checks both the "happy path" and the "early exit on error" path.
func TestExecuteHandlers(t *testing.T) {
	ctx := context.Background()

	t.Run("Success Chain", func(t *testing.T) {
		h1Called, h2Called := false, false

		h1 := func(ctx context.Context) error { h1Called = true; return nil }
		h2 := func(ctx context.Context) error { h2Called = true; return nil }

		err := ExecuteHandlers(ctx, h1, h2)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !h1Called || !h2Called {
			t.Errorf("Expected both middleware handlers to be executed")
		}
	})

	t.Run("Error Halts Chain", func(t *testing.T) {
		h3Called := false
		expectedErr := errors.New("database connection failed")

		// This middleware returns an error, which should stop the chain
		hErr := func(ctx context.Context) error { return expectedErr }

		// This handler should never be reached
		h3 := func(ctx context.Context) error { h3Called = true; return nil }

		err := ExecuteHandlers(ctx, hErr, h3)
		if err == nil || err.Error() != expectedErr.Error() {
			t.Errorf("Expected error '%v', got '%v'", expectedErr, err)
		}
		if h3Called {
			t.Errorf("Expected handler 3 to NOT be called because the previous middleware failed")
		}
	})
}

// TestRegister_InvalidPattern ensures our wrapper properly catches bad cron syntax.
func TestRegister_InvalidPattern(t *testing.T) {
	scheduler := New()

	// "invalid_cron" is not a 5-field cron expression
	_, err := scheduler.Register("invalid_cron", func(ctx context.Context) error {
		return nil
	})

	if err == nil {
		t.Error("Expected an error for an invalid cron pattern, but got nil")
	}
}

// TestScheduler_Lifecycle ensures the scheduler can start and stop gracefully
// without panicking or deadlocking the application.
func TestScheduler_Lifecycle(t *testing.T) {
	scheduler := New()

	// Register a valid job (Runs at minute 0 past every hour)
	jobID, err := scheduler.Register("0 * * * *", func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to register valid job: %v", err)
	}
	if jobID == 0 {
		t.Errorf("Expected a valid job ID, got 0")
	}

	// Start the scheduler in the background
	scheduler.Start()

	// Give it a tiny fraction of a second to spin up
	// (Simulating application uptime)
	time.Sleep(10 * time.Millisecond)

	// Close the scheduler gracefully
	err = scheduler.Close()
	if err != nil {
		t.Fatalf("Failed to close scheduler gracefully: %v", err)
	}
}
