package jwtw

import (
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type userData struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

func TestJWT_CreateAndParse_Success(t *testing.T) {
	mgr := New(&Config{SecretKey: "test-secret"})

	user := userData{ID: "123", Username: "andree"}
	claims := NewClaims(user, 5*time.Minute, "test-issuer", "test-subject")

	tokenStr, err := mgr.Create(claims)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if tokenStr == "" {
		t.Fatal("Expected non-empty token string")
	}

	var parsed MyClaims[userData]
	if err := mgr.Parse(tokenStr, &parsed); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.Data.ID != user.ID {
		t.Errorf("Expected ID %q, got %q", user.ID, parsed.Data.ID)
	}
	if parsed.Data.Username != user.Username {
		t.Errorf("Expected Username %q, got %q", user.Username, parsed.Data.Username)
	}
	if parsed.Issuer != "test-issuer" {
		t.Errorf("Expected Issuer %q, got %q", "test-issuer", parsed.Issuer)
	}
	if parsed.Subject != "test-subject" {
		t.Errorf("Expected Subject %q, got %q", "test-subject", parsed.Subject)
	}
}

func TestJWT_Parse_ExpiredToken(t *testing.T) {
	mgr := New(&Config{SecretKey: "test-secret"})

	user := userData{ID: "123", Username: "andree"}
	claims := NewClaims(user, -1*time.Minute, "test-issuer", "test-subject")

	tokenStr, err := mgr.Create(claims)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var parsed MyClaims[userData]
	if err := mgr.Parse(tokenStr, &parsed); err == nil {
		t.Fatal("Expected error for expired token, got nil")
	}
}

func TestJWT_Parse_InvalidSigningMethod(t *testing.T) {
	hs256Mgr := New(&Config{SecretKey: "test-secret"})

	user := userData{ID: "123", Username: "andree"}
	claims := NewClaims(user, 5*time.Minute, "test-issuer", "test-subject")

	tokenStr, err := hs256Mgr.Create(claims)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Try to parse with HS384 config — should fail with ErrInvalidSigningMethod
	hs384Mgr := New(&Config{SecretKey: "test-secret", SigningAlg: jwtv5.SigningMethodHS384})
	var parsed MyClaims[userData]
	err = hs384Mgr.Parse(tokenStr, &parsed)
	if err == nil {
		t.Fatal("Expected error for wrong signing method, got nil")
	}
}

func TestJWT_Parse_InvalidToken(t *testing.T) {
	mgr := New(&Config{SecretKey: "test-secret"})

	var parsed MyClaims[userData]
	if err := mgr.Parse("garbage-token", &parsed); err == nil {
		t.Fatal("Expected error for invalid token, got nil")
	}
}

func TestNewClaims(t *testing.T) {
	user := userData{ID: "456", Username: "testuser"}
	claims := NewClaims(user, 10*time.Minute, "myapp", "auth")

	if claims.Data.ID != "456" {
		t.Errorf("Expected ID %q, got %q", "456", claims.Data.ID)
	}
	if claims.Issuer != "myapp" {
		t.Errorf("Expected Issuer %q, got %q", "myapp", claims.Issuer)
	}
	if claims.Subject != "auth" {
		t.Errorf("Expected Subject %q, got %q", "auth", claims.Subject)
	}
	if claims.IssuedAt == nil || claims.ExpiresAt == nil {
		t.Fatal("Expected IssuedAt and ExpiresAt to be set")
	}
}