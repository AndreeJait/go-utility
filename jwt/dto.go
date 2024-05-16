package jwt

import (
	"github.com/AndreeJait/go-utility/timew"
	"github.com/golang-jwt/jwt/v4"
	"time"
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
	UserID   T      `json:"user_id"`
	Username string `json:"username"`
}

type CreateTokenRequest[T interface{}] struct {
	SecretToken     string
	Identify        Identify[T]
	ExpiredDuration time.Duration
	ServiceName     string

	TimeW timew.Time
}
