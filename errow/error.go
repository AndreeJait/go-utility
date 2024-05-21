package errow

import "fmt"

type ErrorWCode int

type ErrorW struct {
	Message string
	Code    ErrorWCode
}

func (e ErrorW) Error() string {
	return fmt.Sprintf("[%s] - %s \n", e.Code, e.Message)
}

const (
	InvalidSigningMethod = iota + 700000
	InvalidToken
)

var (
	ErrInvalidSigningMethod = ErrorW{Code: InvalidSigningMethod, Message: "invalid signing method"}
	ErrInvalidToken         = ErrorW{Code: InvalidToken, Message: "invalid token"}
)
