package errow

var (
	ErrInvalidSigningMethod = ErrorW{Code: InvalidSigningMethod, Message: "invalid signing method"}
	ErrInvalidToken         = ErrorW{Code: InvalidToken, Message: "invalid token"}
)

var (
	ErrConfigNotFound = ErrorW{Code: ConfigModeNotFound, Message: "config method not found"}
)

var (
	ErrInternalServer   = ErrorW{Code: 500000, Message: "internal server error"}
	ErrUnauthorized     = ErrorW{Code: 401000, Message: "unauthorized"}
	ErrBadRequest       = ErrorW{Code: 400000, Message: "bad request"}
	ErrForbidden        = ErrorW{Code: 403000, Message: "you don't have access to this resource"}
	ErrSessionExpired   = ErrorW{Code: 440000, Message: "the client's session has expired"}
	ErrResourceNotFound = ErrorW{Code: 404000, Message: "resource not found"}
)
