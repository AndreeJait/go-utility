package redisw

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestDebugContextInjection(t *testing.T) {
	ctx := context.Background()

	// Ensure default is false/nil
	if val := ctx.Value(debugKey); val != nil {
		t.Errorf("Expected default debug context to be nil, got %v", val)
	}

	// Ensure injection works
	debugCtx := DebugContext(ctx)
	if val, ok := debugCtx.Value(debugKey).(bool); !ok || !val {
		t.Errorf("Expected debug context to hold 'true', got %v", val)
	}
}

func TestRedisConnectionAndHook(t *testing.T) {
	// Note: This test requires a local Redis instance running on default port.
	// If you want pure unit tests, you can import "github.com/alicebob/miniredis/v2"
	cfg := &Config{
		Address:   "localhost:6379",
		DebugMode: true, // Enable global debug to test the hook
		Password:  "----",
		DB:        1,
	}

	ctx := context.Background()
	client, err := Connect(ctx, cfg)
	if err != nil {
		t.Skipf("Skipping test because Redis is not reachable at localhost:6379: %v", err)
		return
	}
	defer Disconnect(client)(ctx)

	// 1. Test basic SET and GET
	err = client.Set(ctx, "test_key", "hello", 1*time.Minute).Err()
	if err != nil {
		t.Fatalf("Failed to execute SET command: %v", err)
	}

	val, err := client.Get(ctx, "test_key").Result()
	if err != nil || val != "hello" {
		t.Fatalf("Failed to execute GET command. Expected 'hello', got '%s' (err: %v)", val, err)
	}

	// 2. Test On-Demand Debugging
	// Even if DebugMode was false, this would log because of DebugContext.
	debugCtx := DebugContext(ctx)
	err = client.Del(debugCtx, "test_key").Err()
	if err != nil {
		t.Fatalf("Failed to execute DEL command with debug context: %v", err)
	}

	// Ensure the key is deleted
	_, err = client.Get(ctx, "test_key").Result()
	if err != redis.Nil {
		t.Fatalf("Expected redis.Nil (key not found), got %v", err)
	}
}
