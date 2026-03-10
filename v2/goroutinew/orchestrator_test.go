package goroutinew

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestOrchestrator_SingleAndBatchExecution(t *testing.T) {
	ctx := context.Background()
	orc := New(WithMaxWorkers(5), WithTimeout(2*time.Second))

	// Define Single Step
	orc.AddStep("GetLog", func(ctx context.Context, input any) (any, error) {
		reqID := input.(string)
		return fmt.Sprintf("Log_%s", reqID), nil
	})

	// Define Batch Step
	orc.AddStep("GetList", func(ctx context.Context, input any) (any, error) {
		page := input.(int)
		return fmt.Sprintf("Page_%d", page), nil
	})

	// Add Inputs
	orc.AddInput("GetLog", "123")
	orc.AddBatchInput("GetList", []any{10, 20, 30})

	// Run Execution
	err := orc.Run(ctx)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify Single Result
	logResp := orc.GetResp("GetLog")
	if logResp == nil || logResp.(string) != "Log_123" {
		t.Errorf("Expected 'Log_123', got %v", logResp)
	}

	// Verify Batch Result & Ordering Guarantee
	listResp := orc.GetResp("GetList")
	if listResp == nil {
		t.Fatalf("Expected batch response, got nil")
	}

	slice, ok := listResp.([]any)
	if !ok || len(slice) != 3 {
		t.Fatalf("Expected []any of length 3, got %v", listResp)
	}

	// Ensure the output order perfectly matches the input order []any{10, 20, 30}
	expected := []string{"Page_10", "Page_20", "Page_30"}
	for i, res := range slice {
		if res.(string) != expected[i] {
			t.Errorf("Expected %s at index %d, got %v", expected[i], i, res)
		}
	}
}

func TestOrchestrator_Timeout(t *testing.T) {
	ctx := context.Background()
	// Set an aggressively short timeout
	orc := New(WithMaxWorkers(2), WithTimeout(50*time.Millisecond))

	orc.AddStep("SlowTask", func(ctx context.Context, input any) (any, error) {
		select {
		case <-time.After(200 * time.Millisecond): // This task takes too long
			return "done", nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	orc.AddInput("SlowTask", "payload")

	err := orc.Run(ctx)

	if err == nil {
		t.Fatal("Expected timeout error, but execution succeeded")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected DeadlineExceeded, got: %v", err)
	}
}

func TestOrchestrator_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	orc := New(WithMaxWorkers(2))

	expectedErr := errors.New("database connection lost")

	orc.AddStep("FailingTask", func(ctx context.Context, input any) (any, error) {
		return nil, expectedErr
	})
	orc.AddInput("FailingTask", "payload")

	err := orc.Run(ctx)

	if err == nil {
		t.Fatal("Expected an error, got nil")
	}

	if !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Errorf("Expected error to contain '%v', got '%v'", expectedErr, err)
	}

	// Check if GetErrors captures it
	errs := orc.GetErrors()
	if len(errs) != 1 {
		t.Errorf("Expected exactly 1 error in GetErrors(), got %d", len(errs))
	}
}
