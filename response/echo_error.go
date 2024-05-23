package response

import (
	"github.com/AndreeJait/go-utility/errow"
	"github.com/labstack/echo/v4"
	"net/http"

	"github.com/pkg/errors"
)

// ErrorResponse is the response that represents an error.
type ErrorResponse struct {
	HTTPCode  int              `json:"-"`
	Success   bool             `json:"success" example:"false"`
	Message   string           `json:"message"`
	ErrorCode errow.ErrorWCode `json:"error_code,omitempty"`
	RequestID string           `json:"request_id"`
	Internal  error            `json:"-"`
}

// Error is required by the error interface.
func (e ErrorResponse) Error() string {
	return e.Message
}

// StatusCode is required by CustomHTTPErrorHandler
func (e ErrorResponse) StatusCode() int {
	return e.HTTPCode
}

type stackTracer interface {
	StackTrace() errors.StackTrace
}

// ErrInternalServerError creates a new error response representing an internal server error (HTTP 500)
func ErrInternalServerError(err error) ErrorResponse {

	if _, ok := err.(stackTracer); !ok {
		err = errors.WithStack(err)
	}

	originalErr := errors.Cause(err)
	var errorCode errow.ErrorWCode
	var errorMessage string

	var val = errow.ErrInternalServer
	if errors.As(originalErr, &val) {
		errorCode = val.Code
		errorMessage = val.Message
	}

	return ErrorResponse{
		HTTPCode:  http.StatusUnauthorized,
		Message:   errorMessage,
		ErrorCode: errorCode,
		Internal:  err,
	}
}

func ErrUnauthorized(err error) ErrorResponse {

	if _, ok := err.(stackTracer); !ok {
		err = errors.WithStack(err)
	}

	originalErr := errors.Cause(err)
	var errorCode errow.ErrorWCode
	var errorMessage string

	var val = errow.ErrUnauthorized
	if errors.As(originalErr, &val) {
		errorCode = val.Code
		errorMessage = val.Message
	}

	return ErrorResponse{
		HTTPCode:  http.StatusUnauthorized,
		Message:   errorMessage,
		ErrorCode: errorCode,
		Internal:  err,
	}
}

// ErrForbidden creates a new error response representing an authorization failure (HTTP 403)
func ErrForbidden(err error) ErrorResponse {

	if _, ok := err.(stackTracer); !ok {
		err = errors.WithStack(err)
	}

	originalErr := errors.Cause(err)
	var errorCode errow.ErrorWCode
	var errorMessage string

	var val = errow.ErrForbidden
	if errors.As(originalErr, &val) {
		errorCode = val.Code
		errorMessage = val.Message
	}

	return ErrorResponse{
		HTTPCode:  http.StatusForbidden,
		Message:   errorMessage,
		ErrorCode: errorCode,
		Internal:  err,
	}
}

// ErrSessionExpired creates a new error response representing an session expired error
func ErrSessionExpired(err error) ErrorResponse {

	if _, ok := err.(stackTracer); !ok {
		err = errors.WithStack(err)
	}

	originalErr := errors.Cause(err)
	var errorCode errow.ErrorWCode
	var errorMessage string

	var val = errow.ErrSessionExpired
	if errors.As(originalErr, &val) {
		errorCode = val.Code
		errorMessage = val.Message
	}

	return ErrorResponse{
		HTTPCode:  440,
		Message:   errorMessage,
		ErrorCode: errorCode,
		Internal:  err,
	}
}

// ErrNotFound creates a new error response representing a resource not found (HTTP 404)
func ErrNotFound(err error) ErrorResponse {

	if _, ok := err.(stackTracer); !ok {
		err = errors.WithStack(err)
	}

	originalErr := errors.Cause(err)
	var errorCode errow.ErrorWCode
	var errorMessage string

	var val = errow.ErrBadRequest
	if errors.As(originalErr, &val) {
		errorCode = val.Code
		errorMessage = val.Message
	}

	return ErrorResponse{
		HTTPCode:  http.StatusNotFound,
		Message:   errorMessage,
		ErrorCode: errorCode,
		Internal:  err,
	}
}

// ErrBadRequest creates a new error response representing a bad request (HTTP 400)
func ErrBadRequest(err error) ErrorResponse {

	if _, ok := err.(stackTracer); !ok {
		err = errors.WithStack(err)
	}

	originalErr := errors.Cause(err)
	var errorCode errow.ErrorWCode
	var errorMessage string

	var val errow.ErrorW
	if errors.As(originalErr, &val) {
		errorCode = val.Code
		errorMessage = val.Message
	}

	return ErrorResponse{
		HTTPCode:  http.StatusBadRequest,
		Message:   errorMessage,
		ErrorCode: errorCode,
		Internal:  err,
	}
}

func HTTPError(err error, statusCode int, errorCode errow.ErrorWCode, message string) ErrorResponse {

	if _, ok := err.(stackTracer); !ok {
		err = errors.WithStack(err)
	}

	return ErrorResponse{
		HTTPCode:  statusCode,
		Message:   message,
		ErrorCode: errorCode,
		Internal:  err,
	}
}

func MiddlewareHandleError(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		var val errow.ErrorW
		if errors.As(err, &val) {
			if val.Code >= 500000 {
				return ErrInternalServerError(err)
			} else if val.Code >= 440000 {
				return ErrSessionExpired(err)
			} else if val.Code >= 403000 {
				return ErrForbidden(err)
			} else if val.Code >= 401000 {
				return ErrUnauthorized(err)
			} else if val.Code >= 400000 {
				return ErrBadRequest(err)
			}
		}
		return err
	}
}
