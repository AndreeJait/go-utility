package redisw

import (
	"context"
	"fmt"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/redis/go-redis/v9"
)

type contextKey string

const debugKey contextKey = "redisw-debug"

// Config holds the necessary configuration for a Redis connection.
type Config struct {
	Address      string // e.g., "localhost:6379"
	Password     string // Empty string if no password is set
	DB           int    // Default is 0
	PoolSize     int    // Default is 10 connections per CPU
	MinIdleConns int    // Minimum number of idle connections
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	DebugMode    bool // If true, logs all Redis commands globally
}

// redisHook intercepts Redis commands to log them before execution.
type redisHook struct {
	globalDebug bool
}

func (h *redisHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

// ProcessHook intercepts a single Redis command.
func (h *redisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		// Check if debug is enabled globally or injected locally via context
		isDebug, _ := ctx.Value(debugKey).(bool)

		if h.globalDebug || isDebug {
			start := time.Now()
			err := next(ctx, cmd)
			duration := time.Since(start)

			if err != nil && err != redis.Nil {
				logw.CtxErrorf(ctx, "[REDIS DEBUG] %v | Cmd: %s | err: %v", duration, cmd.String(), err)
			} else {
				logw.CtxInfof(ctx, "[REDIS DEBUG] %v | Cmd: %s", duration, cmd.String())
			}
			return err
		}

		return next(ctx, cmd)
	}
}

// ProcessPipelineHook intercepts pipeline/transaction commands.
func (h *redisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		isDebug, _ := ctx.Value(debugKey).(bool)

		if h.globalDebug || isDebug {
			start := time.Now()
			err := next(ctx, cmds)
			duration := time.Since(start)

			// Build a string representation of the pipeline commands
			cmdStr := "["
			for i, c := range cmds {
				if i > 0 {
					cmdStr += ", "
				}
				cmdStr += c.String()
			}
			cmdStr += "]"

			if err != nil && err != redis.Nil {
				logw.CtxErrorf(ctx, "[REDIS DEBUG PIPELINE] %v | Cmds: %s | err: %v", duration, cmdStr, err)
			} else {
				logw.CtxInfof(ctx, "[REDIS DEBUG PIPELINE] %v | Cmds: %s", duration, cmdStr)
			}
			return err
		}

		return next(ctx, cmds)
	}
}

// Connect establishes a connection pool to the Redis server.
// It automatically adds a logging hook and pings the server to verify connectivity.
func Connect(ctx context.Context, cfg *Config) (*redis.Client, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("redisw: address is required")
	}

	opts := &redis.Options{
		Addr:         cfg.Address,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	client := redis.NewClient(opts)

	// Add our custom hook for debugging
	client.AddHook(&redisHook{globalDebug: cfg.DebugMode})

	// Verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redisw: failed to connect to redis at %s: %w", cfg.Address, err)
	}

	logw.CtxInfof(ctx, "Successfully connected to Redis (%s)", cfg.Address)
	return client, nil
}

// Disconnect safely closes the Redis connection pool.
// It returns a function that perfectly matches gracefulw.CleanupFunc.
func Disconnect(client *redis.Client) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if client != nil {
			logw.CtxInfo(ctx, "Closing Redis connection...")
			return client.Close()
		}
		return nil
	}
}

// DebugContext injects a debug flag into the context.
// When this context is passed to any Redis command, that specific command will be logged,
// even if global DebugMode is set to false.
//
// Usage:
//
//	err := client.Set(redisw.DebugContext(ctx), "key", "value", 0).Err()
func DebugContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, debugKey, true)
}
