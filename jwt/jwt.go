package jwt

import (
	jwtv4 "github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"
	"time"
)

// CreateToken signs the provided claims using HS256.
func CreateToken(req CreateTokenRequest) (string, error) {
	tok := jwtv4.NewWithClaims(jwtv4.SigningMethodHS256, req.Claims)
	return tok.SignedString([]byte(req.SecretToken))
}

// NewClaims is a helper to build correctly-initialized claims with TTL.
func NewClaims[T any](data T, ttl time.Duration, issuer string, subject string) MyClaims[T] {
	now := time.Now()
	return MyClaims[T]{
		Data: data,
		RegisteredClaims: jwtv4.RegisteredClaims{
			Issuer:    issuer,
			Subject:   subject,
			IssuedAt:  jwtv4.NewNumericDate(now),
			ExpiresAt: jwtv4.NewNumericDate(now.Add(ttl)),
			// (optional) NotBefore: now,
		},
	}
}

// ParseToken validates HS256 + expiry and returns your payload.
// ParseToken validates HS256 + expiry and returns your payload.
func ParseToken[T any](tokenStr string, secret string) (out T, err error) {
	out, _, err = ParseTokenWithClaims[T](tokenStr, secret)
	return out, err
}

func ParseTokenWithClaims[T any](tokenStr string, secret string) (out T, claims *MyClaims[T], err error) {
	tok, err := jwtv4.ParseWithClaims(tokenStr, &MyClaims[T]{}, func(t *jwtv4.Token) (interface{}, error) {
		// Explicitly check algorithm here instead of ValidMethods
		if _, ok := t.Method.(*jwtv4.SigningMethodHMAC); !ok || t.Method.Alg() != jwtv4.SigningMethodHS256.Alg() {
			return nil, ErrInvalidSigningMethod
		}
		return []byte(secret), nil
	})
	if err != nil {
		return out, nil, err // includes "token is expired" when exp < now
	}

	c, ok := tok.Claims.(*MyClaims[T])
	if !ok || !tok.Valid {
		return out, nil, errors.New("invalid token")
	}

	return c.Data, c, nil
}
