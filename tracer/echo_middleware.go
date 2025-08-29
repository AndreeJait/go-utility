// tracer/echo_middleware.go
package tracer

import (
	"context"
	"fmt"

	"github.com/labstack/echo/v4"
)

func Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			r := c.Request()
			w := c.Response()

			// Prefer existing request_id from context/header (set by your logger middleware)
			traceID, parentIn, ok := Extract(r.Header)
			if !ok {
				// If LoggerWithRequestID already ran, it put rid in ctx; reuse as traceID
				if rid := r.Header.Get("X-Request-ID"); rid != "" {
					traceID = rid
				}
			}
			if traceID == "" {
				traceID = newTraceID()
			}

			ctx := context.WithValue(r.Context(), ctxTraceIDKey, traceID)

			route := c.Path()
			if route == "" {
				route = r.URL.Path
			}
			name := fmt.Sprintf("%s %s", r.Method, route)
			ctx, sp := StartSpan(ctx, name, WithFields(map[string]any{
				"remote_ip": c.RealIP(),
				"ua":        r.UserAgent(),
				"parent_in": parentIn,
			}))

			// set headers for downstream + client
			r = r.WithContext(ctx)
			c.SetRequest(r)
			w.Header().Set("traceparent", formatTraceParent(traceID, sp.SpanID))
			w.Header().Set("X-Request-ID", traceID)

			err := next(c)
			sp.End(err)
			return err
		}
	}
}
