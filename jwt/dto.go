package jwt

import (
	"github.com/golang-jwt/jwt/v4"
)

const (
	KeyUserID   = "user_id"
	KeyUsername = "username"
)

type Identify[T interface{}] interface {
	GetUserID() T
	GetUsername() string
	GetPassword() string
}

type M map[string]interface{}

type MyClaims[T interface{}] struct {
	jwt.Claims
	Data T `json:"data"`
}

type CreateTokenRequest struct {
	SecretToken string
	Claims      jwt.Claims
}
