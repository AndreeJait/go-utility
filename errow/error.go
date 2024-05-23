package errow

import "fmt"

type ErrorWCode int

type ErrorW struct {
	Message string
	Code    ErrorWCode
}

func (e ErrorW) Error() string {
	return fmt.Sprintf("[%d] - %s \n", e.Code, e.Message)
}
