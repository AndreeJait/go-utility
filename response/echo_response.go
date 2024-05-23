package response

import (
	"github.com/AndreeJait/go-utility/loggerw"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Response struct
type Response struct {
	Success      bool        `json:"success" example:"true"`
	Message      string      `json:"message" example:"success"`
	RequestID    string      `json:"request_id" example:"request_id"`
	ResponseTime int64       `json:"response_time" example:"100000"`
	Data         interface{} `json:"data"`
}

// Data is an alias for map
type Data map[string]interface{}

func buildResponseMsg(defaultMsg string, msg ...string) string {
	if len(msg) == 0 {
		return defaultMsg
	}
	var response string
	for i, item := range msg {
		response += item
		if len(msg)-1 != i {
			response += ", "
		}
	}
	return response
}

// Success responses with JSON format-responseMsg
func Success(c echo.Context, code int, data interface{}, msg ...string) error {

	responseMsg := buildResponseMsg("Success", msg...)

	if data == nil {
		data = map[string]interface{}{}
	}

	requestID := loggerw.GetRequestID(c.Request().Context())

	res := Response{
		Success:   true,
		Message:   responseMsg,
		RequestID: requestID,
		Data:      data,
	}
	return c.JSON(code, res)
}

// SuccessOK returns code 200
func SuccessOK(c echo.Context, data interface{}, msg ...string) error {
	return Success(c, http.StatusOK, data, msg...)
}

// SuccessCreated returns code 201
func SuccessCreated(c echo.Context, data interface{}, msg ...string) error {
	return Success(c, http.StatusCreated, data, msg...)
}
