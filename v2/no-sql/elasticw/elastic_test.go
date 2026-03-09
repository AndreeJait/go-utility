package elasticw

import (
	"context"
	"testing"
	"time"
)

func TestDebugContextInjection(t *testing.T) {
	ctx := context.Background()

	// 1. Validate default state
	if val := ctx.Value(debugKey); val != nil {
		t.Errorf("Expected default debug context to be nil, got %v", val)
	}

	// 2. Validate injection
	debugCtx := DebugContext(ctx)
	if val, ok := debugCtx.Value(debugKey).(bool); !ok || !val {
		t.Errorf("Expected debug context to hold 'true', got %v", val)
	}
}

func TestElasticConnectionAndMonitor(t *testing.T) {
	// Note: This test requires a local Elasticsearch instance running on the default port.
	cfg := &Config{
		Addresses: []string{"http://localhost:9200"},
		DebugMode: false, // Testing on-demand DebugContext instead
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Connect(ctx, cfg)
	if err != nil {
		t.Skipf("Skipping test because Elasticsearch is not reachable at localhost:9200: %v", err)
		return
	}
	defer Disconnect(client)(context.Background())

	// Force debugging to true via context
	debugCtx := DebugContext(ctx)

	// Test a simple Ping operation with DebugContext
	// You should see the HTTP request/response in the terminal output
	ok, err := client.Ping().IsSuccess(debugCtx)
	if err != nil {
		t.Fatalf("Failed to execute Ping command: %v", err)
	}
	if !ok {
		t.Errorf("Expected Ping to return success status")
	}

	// Try fetching info using normal context (should not log if global debug is false)
	_, err = client.Info().Do(ctx)
	if err != nil {
		t.Fatalf("Failed to fetch cluster info: %v", err)
	}
}
