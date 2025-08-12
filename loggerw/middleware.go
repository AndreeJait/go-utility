package loggerw

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// LoggerWithRequestID ensures/propagates X-Request-ID, attaches it to ctx,
// echoes it back to the client, and (optionally) logs request info & body.
func LoggerWithRequestID(log Logger, logBody bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			r := c.Request()
			w := c.Response()

			// ensure request id on ctx, request header, and response header
			ctx, rid := ensureRequestID(r.Context())
			r = r.WithContext(ctx)
			r.Header.Set(RequestIDHeader, rid)
			w.Header().Set(RequestIDHeader, rid)

			start := time.Now()

			var bodyCopy []byte
			if logBody && (r.Method == http.MethodPost || r.Method == http.MethodPut ||
				r.Method == http.MethodPatch || r.Method == http.MethodDelete) {
				if r.Body != nil {
					b, err := io.ReadAll(r.Body)
					if err == nil {
						bodyCopy = b
					} else {
						log.Errorf(ctx, err, "read request body failed")
					}
					r.Body = io.NopCloser(bytes.NewBuffer(b))
				}
			}

			c.SetRequest(r)

			err := next(c)

			fields := Fields{
				"status":  c.Response().Status,
				"latency": time.Since(start).String(),
				"method":  r.Method,
				"path":    r.URL.Path,
			}
			if q := r.URL.RawQuery; q != "" {
				fields["query"] = q
			}
			if logBody && len(bodyCopy) > 0 {
				fields["body"] = string(bodyCopy)
			}

			log.With(ctx, fields).Info(ctx, "http_request")
			return err
		}
	}
}
