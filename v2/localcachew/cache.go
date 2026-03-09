package localcachew

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw" // Sesuaikan dengan module kamu
)

var (
	ErrKeyNotFound = errors.New("localcachew: key not found or expired")

	// globalCache menyimpan instance tunggal (singleton) dari local cache
	globalCache *localCache
	initOnce    sync.Once
)

// Item represents a single cached object along with its expiration time.
type Item struct {
	Value      any
	Expiration int64 // Unix timestamp in nanoseconds. 0 means it never expires.
}

// SetConfig holds the configuration applied during a SetKV operation.
type SetConfig struct {
	TTL time.Duration
}

// Option is a functional option for configuring SetKV behavior.
type Option func(*SetConfig)

// WithTTL sets a Time-To-Live (expiration) for the cached item.
func WithTTL(ttl time.Duration) Option {
	return func(c *SetConfig) {
		c.TTL = ttl
	}
}

type localCache struct {
	mu          sync.RWMutex
	items       map[string]Item
	cleanupFreq time.Duration
	stopCleanup chan struct{}
}

// Init menginisialisasi global cache. Fungsi ini idealnya dipanggil sekali di main.go.
// cleanupInterval menentukan seberapa sering Garbage Collector berjalan di background.
func Init(cleanupInterval time.Duration) {
	initOnce.Do(func() {
		globalCache = &localCache{
			items:       make(map[string]Item),
			cleanupFreq: cleanupInterval,
			stopCleanup: make(chan struct{}),
		}

		if cleanupInterval > 0 {
			go globalCache.startBackgroundCleanup()
			logw.Infof("Local cache initialized with %v cleanup interval", cleanupInterval)
		} else {
			logw.Info("Local cache initialized WITHOUT background cleanup")
		}
	})
}

// getCache memastikan globalCache selalu tersedia meskipun Init() lupa dipanggil (Safety Net).
func getCache() *localCache {
	if globalCache == nil {
		Init(5 * time.Minute) // Default aman jika lupa di-init
	}
	return globalCache
}

func (c *localCache) startBackgroundCleanup() {
	ticker := time.NewTicker(c.cleanupFreq)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-c.stopCleanup:
			logw.Info("Local cache background cleanup stopped gracefully.")
			return
		}
	}
}

func (c *localCache) deleteExpired() {
	now := time.Now().UnixNano()

	c.mu.Lock()
	defer c.mu.Unlock()

	for key, item := range c.items {
		if item.Expiration > 0 && now > item.Expiration {
			delete(c.items, key)
		}
	}
}

// SetKV inserts or updates an item in the global cache.
func SetKV(ctx context.Context, key string, value any, opts ...Option) error {
	c := getCache()

	config := &SetConfig{}
	for _, opt := range opts {
		opt(config)
	}

	var exp int64
	if config.TTL > 0 {
		exp = time.Now().Add(config.TTL).UnixNano()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = Item{
		Value:      value,
		Expiration: exp,
	}

	return nil
}

// Get retrieves an item from the global cache.
// Uses RLock, so multiple endpoints can read simultaneously without blocking.
func Get(ctx context.Context, key string) (any, error) {
	c := getCache()

	c.mu.RLock()
	item, found := c.items[key]
	c.mu.RUnlock()

	if !found {
		return nil, ErrKeyNotFound
	}

	// Check expiration (Lazy Eviction)
	if item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
		return nil, ErrKeyNotFound
	}

	return item.Value, nil
}

// IsKeyExists checks if a key exists and is still valid in the global cache.
func IsKeyExists(ctx context.Context, key string) bool {
	_, err := Get(ctx, key)
	return err == nil
}

// Delete forcefully removes a key from the global cache.
func Delete(ctx context.Context, key string) error {
	c := getCache()

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return nil
}

// Length returns the total number of items currently in the global cache.
func Length() int {
	c := getCache()

	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Clear cleanly wipes the entire global cache.
func Clear() error {
	c := getCache()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]Item)
	return nil
}

// StopCleanup halts the background garbage collector.
// Highly recommended to hook this into gracefulw!
func StopCleanup() {
	if globalCache != nil && globalCache.stopCleanup != nil {
		close(globalCache.stopCleanup)
	}
}
