package tracer

import (
	"context"
	"errors"

	"github.com/AndreeJait/go-utility/loggerw"
)

func WireLogger(l loggerw.Logger) {
	SetLogger(func(ctx context.Context, level, msg string, fields map[string]any) {
		// Keep request_id aligned with trace_id if missing
		if loggerw.GetRequestID(ctx) == "" {
			if tid := TraceID(ctx); tid != "" {
				ctx = loggerw.WithRequestID(ctx, tid)
			}
		}
		ll := l.With(ctx, fields)

		switch level {
		case "error":
			var e error
			// Prefer the handler_error field if provided
			if s, ok := fields["handler_error"].(string); ok && s != "" {
				e = errors.New(s)
			} else {
				e = errors.New(msg)
			}
			ll.Error(ctx, e, msg)
		case "warn":
			ll.Warning(ctx, msg)
		case "debug":
			ll.Debug(ctx, msg)
		default:
			ll.Info(ctx, msg)
		}
	})
}
