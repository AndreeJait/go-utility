package statusw

import (
	"fmt"
)

// StatusCode represents a protocol-agnostic logical error code.
// It can easily be mapped to HTTP status codes or gRPC codes in the inbound layer.
type StatusCode int

const (
	CodeInternal         StatusCode = 500000
	CodeInvalidArgument  StatusCode = 400000
	CodeNotFound         StatusCode = 404000
	CodeUnauthenticated  StatusCode = 401000
	CodePermissionDenied StatusCode = 403000
	CodeConflict         StatusCode = 409000
)

// Error represents a standardized, protocol-agnostic error structure.
// It implements the standard Go 'error' interface.
type Error struct {
	LogicalCode StatusCode // The base logical code (e.g., 404, 500)
	CustomCode  string     // Service-specific code (e.g., "USER-4001")
	Message     string     // Human-readable message for the client
	Cause       error      // The original wrapped error (for logging/debugging)
	Data        any        // Optional additional data payload (e.g., validation errors)
}

// Predefined base errors.
// These act as templates. You must use the With* methods to safely clone and use them.
var (
	InternalServerError = &Error{LogicalCode: CodeInternal, CustomCode: "INTERNAL_ERROR", Message: "Internal server error"}
	InvalidReqParam     = &Error{LogicalCode: CodeInvalidArgument, CustomCode: "INVALID_ARGUMENT", Message: "Invalid request parameters"}
	NotFound            = &Error{LogicalCode: CodeNotFound, CustomCode: "NOT_FOUND", Message: "Resource not found"}
	InvalidCredential   = &Error{LogicalCode: CodeUnauthenticated, CustomCode: "UNAUTHENTICATED", Message: "Invalid or missing credentials"}
	InvalidAccess       = &Error{LogicalCode: CodePermissionDenied, CustomCode: "PERMISSION_DENIED", Message: "You don't have permission to access this resource"}
	Conflict            = &Error{LogicalCode: CodeConflict, CustomCode: "CONFLICT", Message: "Resource conflict or already exists"}
)

// Error implements the standard Go error interface.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.CustomCode, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.CustomCode, e.Message)
}

// Unwrap allows this error to work with standard library errors.Is and errors.As.
func (e *Error) Unwrap() error {
	return e.Cause
}

// clone creates a deep copy of the error to prevent race conditions when using chaining methods on global variables.
func (e *Error) clone() *Error {
	return &Error{
		LogicalCode: e.LogicalCode,
		CustomCode:  e.CustomCode,
		Message:     e.Message,
		Cause:       e.Cause,
		Data:        e.Data,
	}
}

// WithCustomMessage returns a cloned Error with a new user-friendly message.
func (e *Error) WithCustomMessage(msg string) *Error {
	ne := e.clone()
	ne.Message = msg
	return ne
}

// WithCustomCode returns a cloned Error with a specific service-level business code.
func (e *Error) WithCustomCode(code string) *Error {
	ne := e.clone()
	ne.CustomCode = code
	return ne
}

// WithError returns a cloned Error wrapping the original root cause error.
func (e *Error) WithError(err error) *Error {
	ne := e.clone()
	ne.Cause = err
	return ne
}

// WithData returns a cloned Error with an attached optional data payload.
// This is useful for returning validation error details or partial states.
func (e *Error) WithData(data any) *Error {
	ne := e.clone()
	ne.Data = data
	return ne
}

// ToResponse formats the error into a clean map, ready to be serialized to JSON in an HTTP/gRPC handler.
// The 'Data' field is only included if it is not nil.
// Notice that 'Cause' is intentionally omitted here to prevent leaking sensitive system errors to the client.
func (e *Error) ToResponse() map[string]interface{} {
	errMap := map[string]interface{}{
		"code":    e.CustomCode,
		"message": e.Message,
	}

	// Hanya tambahkan data jika tidak kosong (optional)
	if e.Data != nil {
		errMap["data"] = e.Data
	}

	return map[string]interface{}{
		"error": errMap,
	}
}
