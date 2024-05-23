package loggerw

import (
	"bytes"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"io"
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
			_, err := io.Copy(bodyCopy, c.Request().Body)

			if showLog && err != nil {
				go func(body *bytes.Buffer, log Logger, requestID string) {
					var mapRequest map[string]interface{}
					err := json.NewDecoder(bodyCopy).Decode(&mapRequest)
					if err == nil {
						marshal, _ := json.Marshal(&mapRequest)
						log.Infof("[%s] - %s", requestID, string(marshal))
					}
				}(bodyCopy, log, requestID)
			}

			c.SetRequest(r)
			return next(c)
		}
	}
}
