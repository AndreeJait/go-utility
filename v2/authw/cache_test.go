package authw

import (
	"context"
	"testing"
	"time"

	"github.com/AndreeJait/go-utility/v2/localcachew"
)

func TestLocalCache_SetAndGet(t *testing.T) {
	localcachew.Init(5 * time.Minute)
	cache := NewLocalCache()
	ctx := context.Background()

	err := cache.Set(ctx, "test-key", []string{"a", "b"}, 60)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	values, err := cache.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if len(values) != 2 || values[0] != "a" || values[1] != "b" {
		t.Errorf("Expected [a b], got %v", values)
	}
}

func TestLocalCache_GetMiss(t *testing.T) {
	localcachew.Init(5 * time.Minute)
	cache := NewLocalCache()
	ctx := context.Background()

	_, err := cache.Get(ctx, "nonexistent")
	if err != ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss, got %v", err)
	}
}

func TestLocalCache_Delete(t *testing.T) {
	localcachew.Init(5 * time.Minute)
	cache := NewLocalCache()
	ctx := context.Background()

	cache.Set(ctx, "del-key", []string{"x"}, 60)
	cache.Delete(ctx, "del-key")

	_, err := cache.Get(ctx, "del-key")
	if err != ErrCacheMiss {
		t.Errorf("Expected ErrCacheMiss after delete, got %v", err)
	}
}