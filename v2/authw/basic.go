package authw

import (
	"net/http"

	"github.com/AndreeJait/go-utility/v2/statusw"
)

// BasicAuthValidator is a function that validates a username/password pair.
// Return a *Result on success, or an error on failure.
type BasicAuthValidator func(username, password string) (*Result, error)

// BasicAuthConfig holds the configuration for the Basic Auth authenticator.
type BasicAuthConfig struct {
	// Validator is called with the extracted username and password.
	// Required if StaticUsers is not provided.
	Validator BasicAuthValidator

	// StaticUsers is a map of username -> password for simple static validation.
	// When provided, Validator is ignored and a default validator is generated.
	StaticUsers map[string]string
}

type basicAuthenticator struct {
	validator BasicAuthValidator
}

// NewBasicAuth creates a new Basic Auth Authenticator.
// It supports both static username/password maps and custom validator functions.
func NewBasicAuth(cfg *BasicAuthConfig) Authenticator {
	validator := cfg.Validator
	if cfg.StaticUsers != nil {
		staticUsers := cfg.StaticUsers
		validator = func(username, password string) (*Result, error) {
			expectedPwd, ok := staticUsers[username]
			if !ok || expectedPwd != password {
				return nil, statusw.InvalidCredential.WithCustomMessage("Invalid username or password")
			}
			return &Result{
				UserID:   username,
				Username: username,
			}, nil
		}
	}

	return &basicAuthenticator{
		validator: validator,
	}
}

// Authenticate extracts the username and password from the Authorization header
// (Basic scheme), validates them, and returns the authentication result.
func (a *basicAuthenticator) Authenticate(r *http.Request) (*Result, error) {
	username, password, ok := r.BasicAuth()
	if !ok {
		return nil, statusw.InvalidCredential.WithCustomMessage("Missing or invalid Basic Auth header")
	}

	result, err := a.validator(username, password)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Compile-time interface check.
var _ Authenticator = (*basicAuthenticator)(nil)