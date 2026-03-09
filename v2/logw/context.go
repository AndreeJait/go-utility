package logw

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const logIDKey contextKey = "x-log-id"

// InjectLogID generates a new UUID and injects it into the context as x-log-id.
// It is highly recommended to call this in the outermost middleware (inbound layer).
// If an x-log-id already exists in the context, it will return the context unmodified.
func InjectLogID(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Value(logIDKey).(string); ok {
		return ctx
	}
	return context.WithValue(ctx, logIDKey, uuid.New().String())
}

// GetLogID extracts the x-log-id from the given context.
// It returns an empty string if the context is nil or the log ID is not found.
func GetLogID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if val, ok := ctx.Value(logIDKey).(string); ok {
		return val
	}
	return ""
}
