// tracer/loggerw_adapter.go
package tracer

import (
	"context"

	"github.com/AndreeJait/go-utility/loggerw"
)

// WireLogger makes tracer emit through your loggerw instance.
func WireLogger(l loggerw.Logger) {
	SetLogger(func(ctx context.Context, level, msg string, fields map[string]any) {
		// ensure request_id from tracer trace_id if loggerw doesn’t have it yet
		if loggerw.GetRequestID(ctx) == "" {
			if tid := TraceID(ctx); tid != "" {
				ctx = loggerw.WithRequestID(ctx, tid)
			}
		}
		ll := l.With(ctx, fields)
		switch level {
		case "error":
			ll.Errorf(ctx, nil, "%s", msg) // no err here; error string is in fields["error"]
		case "debug":
			ll.Debug(ctx, msg)
		case "warn":
			ll.Warning(ctx, msg)
		default:
			ll.Info(ctx, msg)
		}
	})
}
