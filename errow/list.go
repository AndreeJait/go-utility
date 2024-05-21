package errow

var (
	ErrInvalidSigningMethod = ErrorW{Code: InvalidSigningMethod, Message: "invalid signing method"}
	ErrInvalidToken         = ErrorW{Code: InvalidToken, Message: "invalid token"}
)

var (
	ErrConfigNotFound = ErrorW{Code: ConfigModeNotFound, Message: "config method not found"}
)
