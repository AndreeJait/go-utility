package logw

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

var globalLogger *slog.Logger

func init() {
	globalLogger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

// Init configures and initializes the global logger based on the provided LogConfig.
// It sets up the output destinations (console, file, and broker), log level, and log format.
// This function should be called once at the start of the application.
func Init(cfg *LogConfig) error {
	var writers []io.Writer

	writers = append(writers, os.Stdout)

	if cfg.WriteToFile && cfg.FilePath != "" {
		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("failed to open log file %s: %w", cfg.FilePath, err)
		}
		writers = append(writers, file)
	}

	if cfg.SendToBroker && cfg.BrokerWriter != nil {
		writers = append(writers, cfg.BrokerWriter)
	}

	multiWriter := io.MultiWriter(writers...)

	var programLevel slog.LevelVar
	switch cfg.Level {
	case "debug":
		programLevel.Set(slog.LevelDebug)
	case "error":
		programLevel.Set(slog.LevelError)
	case "warn":
		programLevel.Set(slog.LevelWarn)
	default:
		programLevel.Set(slog.LevelInfo)
	}

	opts := &slog.HandlerOptions{
		Level: &programLevel,
	}

	var handler slog.Handler
	if cfg.Format == FormatText {
		handler = slog.NewTextHandler(multiWriter, opts)
	} else {
		handler = slog.NewJSONHandler(multiWriter, opts)
	}

	globalLogger = slog.New(handler)
	return nil
}

// BuildLogAttributes extracts base attributes from the context to be appended to log entries.
// Currently, it extracts x-log-id, and it serves as an integration point for spanw (tracing).
func BuildLogAttributes(ctx context.Context) []any {
	var attrs []any
	if logID := GetLogID(ctx); logID != "" {
		attrs = append(attrs, slog.String("x-log-id", logID))
	}
	return attrs
}

// CtxInfo logs a message at Info level, including base attributes extracted from the context.
func CtxInfo(ctx context.Context, msg string) {
	globalLogger.InfoContext(ctx, msg, BuildLogAttributes(ctx)...)
}

// CtxInfof logs a formatted message at Info level, including base attributes extracted from the context.
// It uses fmt.Sprintf to construct the message from the format string and arguments.
func CtxInfof(ctx context.Context, format string, args ...any) {
	globalLogger.InfoContext(ctx, fmt.Sprintf(format, args...), BuildLogAttributes(ctx)...)
}

// CtxWarning logs a message at Warning level, including base attributes extracted from the context.
func CtxWarning(ctx context.Context, msg string) {
	globalLogger.WarnContext(ctx, msg, BuildLogAttributes(ctx)...)
}

// CtxWarningf logs a formatted message at Warning level, including base attributes extracted from the context.
// It uses fmt.Sprintf to construct the message from the format string and arguments.
func CtxWarningf(ctx context.Context, format string, args ...any) {
	globalLogger.WarnContext(ctx, fmt.Sprintf(format, args...), BuildLogAttributes(ctx)...)
}

// CtxError logs a message at Error level, including base attributes extracted from the context.
func CtxError(ctx context.Context, msg string) {
	globalLogger.ErrorContext(ctx, msg, BuildLogAttributes(ctx)...)
}

// CtxErrorf logs a formatted message at Error level, including base attributes extracted from the context.
// It uses fmt.Sprintf to construct the message from the format string and arguments.
func CtxErrorf(ctx context.Context, format string, args ...any) {
	globalLogger.ErrorContext(ctx, fmt.Sprintf(format, args...), BuildLogAttributes(ctx)...)
}

// Info logs a message at Info level without context attributes.
func Info(msg string) { globalLogger.Info(msg) }

// Infof logs a formatted message at Info level without context attributes.
// It uses fmt.Sprintf to construct the message from the format string and arguments.
func Infof(format string, args ...any) { globalLogger.Info(fmt.Sprintf(format, args...)) }

// Warning logs a message at Warning level without context attributes.
func Warning(msg string) { globalLogger.Warn(msg) }

// Warningf logs a formatted message at Warning level without context attributes.
// It uses fmt.Sprintf to construct the message from the format string and arguments.
func Warningf(format string, args ...any) { globalLogger.Warn(fmt.Sprintf(format, args...)) }

// Error logs a message at Error level without context attributes.
func Error(msg string) { globalLogger.Error(msg) }

// Errorf logs a formatted message at Error level without context attributes.
// It uses fmt.Sprintf to construct the message from the format string and arguments.
func Errorf(format string, args ...any) { globalLogger.Error(fmt.Sprintf(format, args...)) }
