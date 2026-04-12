package authw

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/AndreeJait/go-utility/v2/localcachew"
	"github.com/redis/go-redis/v9"
)

// ErrCacheMiss is returned when a key is not found in the cache.
var ErrCacheMiss = errors.New("authw: cache miss")

// localCache implements Cache using localcachew.
type localCache struct{}

// NewLocalCache creates a Cache backed by the in-memory localcachew singleton.
// localcachew.Init() must have been called before use.
func NewLocalCache() Cache {
	return &localCache{}
}

func (c *localCache) Get(ctx context.Context, key string) ([]string, error) {
	val, err := localcachew.Get(ctx, key)
	if err != nil {
		return nil, ErrCacheMiss
	}
	roles, ok := val.([]string)
	if !ok {
		return nil, ErrCacheMiss
	}
	return roles, nil
}

func (c *localCache) Set(ctx context.Context, key string, values []string, ttlSeconds int64) error {
	return localcachew.SetKV(ctx, key, values, localcachew.WithTTL(time.Duration(ttlSeconds)*time.Second))
}

func (c *localCache) Delete(ctx context.Context, key string) error {
	return localcachew.Delete(ctx, key)
}

// redisCache implements Cache using a Redis client.
type redisCache struct {
	client *redis.Client
}

// NewRedisCache creates a Cache backed by a Redis client.
// The client should already be connected via redisw.Connect().
func NewRedisCache(client *redis.Client) Cache {
	return &redisCache{client: client}
}

func (c *redisCache) Get(ctx context.Context, key string) ([]string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrCacheMiss
		}
		return nil, err
	}

	var values []string
	if err := json.Unmarshal([]byte(val), &values); err != nil {
		return nil, err
	}
	return values, nil
}

func (c *redisCache) Set(ctx context.Context, key string, values []string, ttlSeconds int64) error {
	data, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, time.Duration(ttlSeconds)*time.Second).Err()
}

func (c *redisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}