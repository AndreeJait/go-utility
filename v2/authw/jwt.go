package authw

import (
	"net/http"
	"strings"

	"github.com/AndreeJait/go-utility/v2/jwtw"
	"github.com/AndreeJait/go-utility/v2/statusw"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// JWTConfig holds the configuration for the JWT authenticator.
type JWTConfig struct {
	// JWT is a required initialized jwtw.JWT instance.
	JWT jwtw.JWT
	// NewClaims is a required factory returning an empty claims struct for each request.
	NewClaims func() jwtv5.Claims
	// ExtractResult is a required function that maps parsed claims to an authw.Result.
	ExtractResult func(claims jwtv5.Claims) *Result
}

type jwtAuthenticator struct {
	jwt           jwtw.JWT
	newClaims     func() jwtv5.Claims
	extractResult func(claims jwtv5.Claims) *Result
}

// NewJWT creates a new JWT-based Authenticator.
func NewJWT(cfg *JWTConfig) Authenticator {
	return &jwtAuthenticator{
		jwt:           cfg.JWT,
		newClaims:     cfg.NewClaims,
		extractResult: cfg.ExtractResult,
	}
}

// Authenticate extracts the Bearer token from the Authorization header,
// parses and validates it, and returns the authentication result.
func (a *jwtAuthenticator) Authenticate(r *http.Request) (*Result, error) {
	tokenStr, err := extractBearerToken(r)
	if err != nil {
		return nil, err
	}

	claims := a.newClaims()
	if err := a.jwt.Parse(tokenStr, claims); err != nil {
		return nil, statusw.InvalidCredential.WithError(err).WithCustomMessage("Invalid or expired token")
	}

	result := a.extractResult(claims)
	return result, nil
}

// extractBearerToken extracts the token from the "Authorization: Bearer <token>" header.
func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", statusw.InvalidCredential.WithCustomMessage("Missing Authorization header")
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", statusw.InvalidCredential.WithCustomMessage("Invalid Authorization header format")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", statusw.InvalidCredential.WithCustomMessage("Empty Bearer token")
	}
	return token, nil
}

// Compile-time interface check.
var _ Authenticator = (*jwtAuthenticator)(nil)