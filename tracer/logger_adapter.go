// tracer/loggerw_adapter.go
package tracer

import (
	"context"
	"github.com/pkg/errors"

	"github.com/AndreeJait/go-utility/loggerw"
)

// WireLogger makes tracer emit through your loggerw instance.
func WireLogger(l loggerw.Logger) {
	SetLogger(func(ctx context.Context, level, msg string, fields map[string]any) {
		// ensure request_id stays in sync with trace_id
		if loggerw.GetRequestID(ctx) == "" {
			if tid := TraceID(ctx); tid != "" {
				ctx = loggerw.WithRequestID(ctx, tid)
			}
		}

		ll := l.With(ctx, fields)

		switch level {
		case "error":
			// Build a non-nil error for loggerw
			var e error
			if v, ok := fields["error"]; ok {
				if s, ok := v.(string); ok && s != "" {
					e = errors.New(s)
				}
			}
			if e == nil {
				e = errors.New(msg) // fallback to message
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
