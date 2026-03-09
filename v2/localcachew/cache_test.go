package localcachew

import (
	"context"
	"sync"
	"testing"
	"time"
)

// setupTest memastikan cache bersih sebelum test dijalankan
func setupTest() {
	Init(0) // 0 untuk mematikan auto-cleanup selama test logika
	_ = Clear()
}

func TestGlobalCache_SetAndGet(t *testing.T) {
	setupTest()
	ctx := context.Background()

	err := SetKV(ctx, "framework", "echo", WithTTL(1*time.Minute))
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	val, err := Get(ctx, "framework")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if val.(string) != "echo" {
		t.Errorf("Expected 'echo', got '%v'", val)
	}
}

func TestGlobalCache_ExpirationLazyEviction(t *testing.T) {
	setupTest()
	ctx := context.Background()

	// Set dengan TTL super singkat (50 millisecond)
	_ = SetKV(ctx, "temp", "data", WithTTL(50*time.Millisecond))

	// Get seketika harusnya masih ada
	if !IsKeyExists(ctx, "temp") {
		t.Errorf("Expected key to exist immediately after set")
	}

	// Tunggu sampai expired
	time.Sleep(100 * time.Millisecond)

	// Get sekarang harusnya gagal (Lazy Eviction bekerja)
	_, err := Get(ctx, "temp")
	if err != ErrKeyNotFound {
		t.Errorf("Expected ErrKeyNotFound after expiration, got %v", err)
	}
}

func TestGlobalCache_DeleteAndClear(t *testing.T) {
	setupTest()
	ctx := context.Background()

	_ = SetKV(ctx, "A", 1)
	_ = SetKV(ctx, "B", 2)

	_ = Delete(ctx, "A")
	if IsKeyExists(ctx, "A") {
		t.Errorf("Key A should have been deleted")
	}

	_ = Clear()
	if Length() != 0 {
		t.Errorf("Cache should be empty after Clear()")
	}
}

// Test ini membuktikan bahwa "Safety Net" kita bekerja:
// Jika developer lupa panggil Init(), kode tidak akan panic nil pointer.
func TestGlobalCache_SafetyNetWithoutInit(t *testing.T) {
	// Sengaja kita bikin nil globalCache-nya
	globalCache = nil
	// Reset initOnce agar bisa diinisialisasi ulang oleh safety net
	initOnce = sync.Once{}

	ctx := context.Background()

	// SetKV harusnya otomatis memanggil Init() default di balik layar
	err := SetKV(ctx, "safety", "net")
	if err != nil {
		t.Errorf("Expected SetKV to handle uninitialized cache gracefully, got: %v", err)
	}

	if val, _ := Get(ctx, "safety"); val != "net" {
		t.Errorf("Safety net failed to retrieve value")
	}
}
