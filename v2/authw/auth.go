package authw

import (
	"context"
	"net/http"
)

type contextKey string

const authResultKey contextKey = "x-auth-result"

// Result holds the authenticated identity data extracted by an Authenticator.
// It is stored in the request context so that downstream handlers can access it.
type Result struct {
	UserID      any      // The authenticated user's identifier (e.g., string, int64)
	Username    string   // The authenticated user's name
	Roles       []string // The authenticated user's roles
	Permissions []string // The authenticated user's permissions
	Data        any      // Arbitrary payload from the authenticator (e.g., JWT claims data)
}

// GetUserID returns the UserID as a string. Returns empty string if nil or not a string.
func (r *Result) GetUserID() string {
	if r.UserID == nil {
		return ""
	}
	if s, ok := r.UserID.(string); ok {
		return s
	}
	return ""
}

// Authenticator defines the framework-agnostic contract for authenticating a request.
type Authenticator interface {
	// Authenticate extracts and validates credentials from the HTTP request.
	// On failure, it should return statusw.InvalidCredential (or a derived error)
	// so the HTTP error pipeline maps it to 401.
	Authenticate(r *http.Request) (*Result, error)
}

// WithResult injects the authentication Result into the request context.
func WithResult(ctx context.Context, result *Result) context.Context {
	return context.WithValue(ctx, authResultKey, result)
}

// FromContext retrieves the authentication Result from the request context.
// Returns nil if no Result is present.
func FromContext(ctx context.Context) *Result {
	if val, ok := ctx.Value(authResultKey).(*Result); ok {
		return val
	}
	return nil
}