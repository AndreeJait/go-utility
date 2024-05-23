package loggerw

import (
	"encoding/json"
	"github.com/labstack/echo/v4"
)

func LoggerWitRequestID(log Logger, showLog bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {

			ctx := c.Request().Context()
			r := c.Request()
			newContext, requestID := WithRequest(ctx, r)
			r = r.WithContext(newContext)

			if showLog {
				go func(c echo.Context, log Logger, requestID string) {
					var mapRequest map[string]interface{}
					err := c.Bind(&mapRequest)
					if err == nil {
						bytes, _ := json.Marshal(&mapRequest)
						log.Infof("[%s] - %s", requestID, string(bytes))
					}
				}(c, log, requestID)
			}

			c.SetRequest(r)
			return next(c)
		}
	}
}
