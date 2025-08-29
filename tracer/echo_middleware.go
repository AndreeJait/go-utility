package tracer

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
)

func Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			r := c.Request()
			route := c.Path()
			if route == "" {
				route = r.URL.Path
			}
			name := fmt.Sprintf("%s %s", r.Method, route)

			// If upstream set X-Request-ID, reuse it as trace_id for correlation
			if rid := r.Header.Get("X-Request-ID"); rid != "" {
				ctx := context.WithValue(r.Context(), ctxTraceIDKey, rid)
				r = r.WithContext(ctx)
				c.SetRequest(r)
			}

			// Start trace and propagate to handler
			ctx, _ := StartTrace(c.Request().Context(), name)
			c.SetRequest(r.WithContext(ctx))

			// Reflect back to client
			if tid := TraceID(ctx); tid != "" {
				c.Response().Header().Set("X-Request-ID", tid)
			}

			// Optionally also set W3C traceparent (span id not tracked here)
			// c.Response().Header().Set("traceparent", fmt.Sprintf("00-%s-%s-01", TraceID(ctx), "0000000000000000"))

			defer Flush(ctx, &err)
			err = next(c)
			return err
		}
	}
}
