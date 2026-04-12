package jwtw

import (
	"errors"
	"fmt"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// ErrInvalidSigningMethod is returned when the token uses an unexpected signing algorithm.
var ErrInvalidSigningMethod = errors.New("jwtw: invalid signing method")

// JWT defines the contract for creating and parsing JWT tokens.
type JWT interface {
	// Create signs the given claims and returns the encoded token string.
	Create(claims jwtv5.Claims) (string, error)
	// Parse validates the token string and populates the given claims struct.
	Parse(tokenStr string, claims jwtv5.Claims) error
}

// Config holds the configuration for the JWT manager.
type Config struct {
	SecretKey  string             // HMAC shared secret (required for HS256)
	SigningAlg jwtv5.SigningMethod // Defaults to HS256 if nil
}

type jwtManager struct {
	secret []byte
	alg    jwtv5.SigningMethod
}

// New initializes a new JWT manager using the provided Config.
func New(cfg *Config) JWT {
	alg := cfg.SigningAlg
	if alg == nil {
		alg = jwtv5.SigningMethodHS256
	}
	return &jwtManager{
		secret: []byte(cfg.SecretKey),
		alg:    alg,
	}
}

// Create signs the given claims using the configured algorithm and secret.
func (m *jwtManager) Create(claims jwtv5.Claims) (string, error) {
	tok := jwtv5.NewWithClaims(m.alg, claims)
	signed, err := tok.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("jwtw: failed to sign token: %w", err)
	}
	return signed, nil
}

// Parse validates the token string, verifies the signing algorithm, and populates claims.
func (m *jwtManager) Parse(tokenStr string, claims jwtv5.Claims) error {
	_, err := jwtv5.ParseWithClaims(tokenStr, claims, func(t *jwtv5.Token) (any, error) {
		if t.Method.Alg() != m.alg.Alg() {
			return nil, ErrInvalidSigningMethod
		}
		return m.secret, nil
	})
	if err != nil {
		return fmt.Errorf("jwtw: failed to parse token: %w", err)
	}
	return nil
}

// MyClaims is a generic claims struct that carries arbitrary data alongside registered claims.
type MyClaims[T any] struct {
	Data T `json:"data"`
	jwtv5.RegisteredClaims
}

// NewClaims builds a MyClaims[T] with the given data, TTL, issuer, and subject.
func NewClaims[T any](data T, ttl time.Duration, issuer, subject string) MyClaims[T] {
	now := time.Now()
	return MyClaims[T]{
		Data: data,
		RegisteredClaims: jwtv5.RegisteredClaims{
			Issuer:    issuer,
			Subject:   subject,
			IssuedAt:  jwtv5.NewNumericDate(now),
			ExpiresAt: jwtv5.NewNumericDate(now.Add(ttl)),
		},
	}
}

// Compile-time interface check.
var _ JWT = (*jwtManager)(nil)