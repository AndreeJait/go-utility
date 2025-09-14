package jwt

import (
	"errors"
	jwtv4 "github.com/golang-jwt/jwt/v4"
)

var (
	ErrInvalidSigningMethod = errors.New("invalid signing method")
)

const (
	KeyUserID   = "user_id"
	KeyUsername = "username"
)

// Optional: a generic identity interface (if you need it elsewhere)
type Identify[T any] interface {
	GetUserID() T
	GetUsername() string
	GetPassword() string
}

type M map[string]interface{}

// MyClaims embeds StandardClaims ANONYMOUSLY so "exp", "iat", etc. are top-level.
type MyClaims[T any] struct {
	Data T `json:"data"`
	jwtv4.RegisteredClaims
}

type CreateTokenRequest struct {
	SecretToken string
	Claims      jwtv4.Claims // pass a value of MyClaims[T] here
	// (optionally add Algorithm, KeyID, etc.)
}
