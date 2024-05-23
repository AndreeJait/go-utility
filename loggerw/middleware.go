package loggerw

import (
	"bytes"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
)

func LoggerWitRequestID(log Logger, showLog bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {

			ctx := c.Request().Context()
			r := c.Request()
			newContext, requestID := WithRequest(ctx, r)
			r = r.WithContext(newContext)

			bodyCopy := new(bytes.Buffer)
			// Read the whole body
			_, err := io.Copy(bodyCopy, r.Body)
			if showLog && err == nil {
				// Log query parameters for GET requests
				if r.Method == http.MethodGet {
					queryParams := r.URL.Query()
					log.Infof("[%s] Query Parameters: %v", requestID, queryParams)
				}

				if r.Method == http.MethodPost ||
					r.Method == http.MethodPut ||
					r.Method == http.MethodPatch ||
					r.Method == http.MethodDelete {

					// Read the body
					bodyBytes, err := io.ReadAll(r.Body)
					if err != nil {
						return err
					}

					// Restore the body to its original state
					r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

					// Log the body
					log.Infof("[%s] request Body: %s", requestID, bodyBytes)
				}
			}

			c.SetRequest(r)
			return next(c)
		}
	}
}
