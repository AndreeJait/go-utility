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
