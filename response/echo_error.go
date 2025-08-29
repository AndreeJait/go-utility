package response

import (
	"context"
	"github.com/AndreeJait/go-utility/errow"
	"github.com/AndreeJait/go-utility/loggerw"
	"github.com/labstack/echo/v4"
	"net/http"
	"runtime/debug"
	"sort"

	validation "github.com/go-ozzo/ozzo-validation/v4"

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

type ErrResponseFunc func(errInner error) ErrorResponse

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

func CustomHttpErrorHandler(log loggerw.Logger,
	mapErrorResponse map[errow.ErrorWCode]ErrResponseFunc, withStack, withStackContext bool) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		var requestID = loggerw.GetRequestID(c.Request().Context())
		var ctx = c.Request().Context()
		err = ConvertError(err, mapErrorResponse)

		var errorResponse ErrorResponse
		if !errors.As(err, &errorResponse) {
			errorResponse = ErrorResponse{
				Success:   false,
				HTTPCode:  http.StatusInternalServerError,
				Message:   errow.ErrInternalServer.Message,
				ErrorCode: errow.ErrInternalServer.Code,
				Internal:  err,
			}
			err = errorResponse
		}
		errorResponse.RequestID = requestID

		// handles resource not found errors
		if errors.Is(errorResponse.Internal, echo.ErrNotFound) {
			err = HTTPError(errorResponse.Internal, http.StatusNotFound, errow.ErrResourceNotFound.Code, "requested endpoint is not registered")
		}

		// Handles validation error
		if errors.As(errorResponse.Internal, &validation.Errors{}) || errors.As(errorResponse.Internal, &validation.ErrorObject{}) {
			err = HTTPError(errorResponse.Internal, http.StatusBadRequest, errow.ErrBadRequest.Code, errorResponse.Internal.Error())
		}

		if !errors.As(err, &errorResponse) {
			errorResponse = ErrInternalServerError(err)
		}
		errorResponse.RequestID = loggerw.GetRequestID(c.Request().Context())

		if withStack {
			if stderr, ok := errorResponse.Internal.(stackTracer); ok {
				log.Errorf(ctx, errorResponse.Internal, "%v", stderr)
			}
		}
		if withStackContext {
			if st := GetStack(ctx); st != nil {
				log.Infof(ctx, "Context: %v", st)
			}
		}

		log.Error(ctx, errorResponse.Internal)

		errJson := c.JSON(errorResponse.HTTPCode, errorResponse)
		if errJson != nil {
			log.Error(ctx, errJson)
		}
	}
}

func ConvertError(err error, mapError map[errow.ErrorWCode]ErrResponseFunc) error {

	var arrKey []int
	for key, _ := range mapError {
		arrKey = append(arrKey, int(key))
	}
	sort.Slice(arrKey, func(i, j int) bool {
		return arrKey[i] > arrKey[j]
	})

	var val errow.ErrorW
	if errors.As(err, &val) {
		for _, key := range arrKey {
			if int(val.Code) >= key {
				return mapError[errow.ErrorWCode(key)](val)
			}
		}
	}
	return err
}

var MapDefaultErrResponse = map[errow.ErrorWCode]ErrResponseFunc{
	errow.ErrInternalServer.Code:   ErrInternalServerError,
	errow.ErrSessionExpired.Code:   ErrSessionExpired,
	errow.ErrForbidden.Code:        ErrForbidden,
	errow.ErrUnauthorized.Code:     ErrUnauthorized,
	errow.ErrBadRequest.Code:       ErrBadRequest,
	errow.ErrResourceNotFound.Code: ErrNotFound,
}

type ctxKey string

const stackKey ctxKey = "stacktrace"

func WithStack(ctx context.Context) context.Context {
	return context.WithValue(ctx, stackKey, debug.Stack())
}

func GetStack(ctx context.Context) []byte {
	if v, ok := ctx.Value(stackKey).([]byte); ok {
		return v
	}
	return nil
}
