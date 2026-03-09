// Package mongow provides a high-level wrapper around the MongoDB Go Driver v2.
// It includes support for connection pooling, command monitoring for debugging,
// and simplified transaction management.
package mongow

import (
	"context"
	"fmt"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type contextKey string

const debugKey contextKey = "mongow-debug"

// Config holds the necessary configuration to establish a MongoDB connection.
type Config struct {
	URI             string        // Connection string (e.g., "mongodb://localhost:27017")
	MinPoolSize     uint64        // Minimum number of connections in the pool
	MaxPoolSize     uint64        // Maximum number of connections in the pool
	MaxConnIdleTime time.Duration // Maximum time a connection can remain idle
	DebugMode       bool          // If true, logs all MongoDB commands globally via logw
}

// buildCommandMonitor creates an event monitor to intercept and log MongoDB queries.
// It captures the start, success, and failure of commands.
func buildCommandMonitor(globalDebug bool) *event.CommandMonitor {
	return &event.CommandMonitor{
		Started: func(ctx context.Context, evt *event.CommandStartedEvent) {
			// Skip noisy internal commands
			if isInternalCommand(evt.CommandName) {
				return
			}

			isDebug, _ := ctx.Value(debugKey).(bool)
			if globalDebug || isDebug {
				// In v2, evt.Command is a bson.Raw; .String() provides a readable version.
				logw.CtxInfof(ctx, "[MONGO DEBUG] DB: %s | Cmd: %s | Query: %s",
					evt.DatabaseName, evt.CommandName, evt.Command.String())
			}
		},
		Succeeded: func(ctx context.Context, evt *event.CommandSucceededEvent) {
			isDebug, _ := ctx.Value(debugKey).(bool)
			if globalDebug || isDebug {
				duration := evt.Duration
				logw.CtxInfof(ctx, "[MONGO SUCCESS] Cmd: %s | Duration: %v", evt.CommandName, duration)
			}
		},
		Failed: func(ctx context.Context, evt *event.CommandFailedEvent) {
			isDebug, _ := ctx.Value(debugKey).(bool)
			if globalDebug || isDebug {
				duration := evt.Duration
				logw.CtxErrorf(ctx, "[MONGO FAILED] Cmd: %s | Duration: %v | err: %v",
					evt.CommandName, duration, evt.Failure)
			}
		},
	}
}

// isInternalCommand filters out heartbeat and handshake commands to keep logs clean.
func isInternalCommand(name string) bool {
	switch name {
	case "hello", "ismaster", "saslStart", "saslContinue", "ping":
		return true
	default:
		return false
	}
}

// Connect establishes a connection to the MongoDB server using Driver v2 patterns.
// It initializes the client and pings the server to ensure connectivity.
func Connect(ctx context.Context, cfg *Config) (*mongo.Client, error) {
	if cfg.URI == "" {
		return nil, fmt.Errorf("mongow: URI is required")
	}

	clientOptions := options.Client().ApplyURI(cfg.URI)

	if cfg.MinPoolSize > 0 {
		clientOptions.SetMinPoolSize(cfg.MinPoolSize)
	}
	if cfg.MaxPoolSize > 0 {
		clientOptions.SetMaxPoolSize(cfg.MaxPoolSize)
	}
	if cfg.MaxConnIdleTime > 0 {
		clientOptions.SetMaxConnIdleTime(cfg.MaxConnIdleTime)
	}

	// Attach the command monitor
	clientOptions.SetMonitor(buildCommandMonitor(cfg.DebugMode))

	// In Driver v2, mongo.Connect no longer takes a context for initialization.
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("mongow: failed to create client: %w", err)
	}

	// Ping the server to verify the connection is active.
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongow: failed to ping mongo server: %w", err)
	}

	logw.CtxInfof(ctx, "Successfully connected to MongoDB (%s)", cfg.URI)
	return client, nil
}

// Disconnect safely closes the MongoDB connection pool.
// It returns a closure compatible with graceful shutdown patterns.
func Disconnect(client *mongo.Client) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if client != nil {
			logw.CtxInfo(ctx, "Closing MongoDB connection...")
			return client.Disconnect(ctx)
		}
		return nil
	}
}

// DebugContext injects a debug flag into the context.
// Operations using this context will be logged regardless of the global Config.DebugMode.
func DebugContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, debugKey, true)
}

// Transaction executes the provided function within an ACID transaction.
// It handles session lifecycle and automatically commits or aborts based on the returned error.
//
// Note: Requires a MongoDB Replica Set or Sharded Cluster.
func Transaction(ctx context.Context, client *mongo.Client, fn func(sessCtx context.Context) error) error {
	session, err := client.StartSession()
	if err != nil {
		return fmt.Errorf("mongow: failed to start session: %w", err)
	}
	defer session.EndSession(ctx)

	// Driver v2 uses standard context.Context in the transaction callback.
	_, err = session.WithTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
		err := fn(sessCtx)
		return nil, err
	})

	if err != nil {
		return fmt.Errorf("mongow: transaction failed: %w", err)
	}

	return nil
}
