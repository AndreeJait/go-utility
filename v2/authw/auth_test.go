package authw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AndreeJait/go-utility/v2/jwtw"
	"github.com/AndreeJait/go-utility/v2/statusw"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type userData struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

func TestContextInjection(t *testing.T) {
	result := &Result{UserID: "123", Username: "testuser"}
	ctx := WithResult(context.Background(), result)

	got := FromContext(ctx)
	if got == nil {
		t.Fatal("Expected non-nil Result from context")
	}
	if got.UserID != "123" {
		t.Errorf("Expected UserID %q, got %q", "123", got.UserID)
	}
	if got.Username != "testuser" {
		t.Errorf("Expected Username %q, got %q", "testuser", got.Username)
	}
}

func TestFromContext_Nil(t *testing.T) {
	got := FromContext(context.Background())
	if got != nil {
		t.Fatal("Expected nil Result from empty context")
	}
}

func TestResult_GetUserID(t *testing.T) {
	r := &Result{UserID: "abc"}
	if got := r.GetUserID(); got != "abc" {
		t.Errorf("Expected %q, got %q", "abc", got)
	}

	rNil := &Result{UserID: nil}
	if got := rNil.GetUserID(); got != "" {
		t.Errorf("Expected empty string for nil UserID, got %q", got)
	}

	rInt := &Result{UserID: 42}
	if got := rInt.GetUserID(); got != "" {
		t.Errorf("Expected empty string for non-string UserID, got %q", got)
	}
}

func TestJWTAuth_Success(t *testing.T) {
	mgr := jwtw.New(&jwtw.Config{SecretKey: "secret"})
	authenticator := NewJWT(&JWTConfig{
		JWT:       mgr,
		NewClaims: func() jwtv5.Claims { return &jwtw.MyClaims[userData]{} },
		ExtractResult: func(claims jwtv5.Claims) *Result {
			c := claims.(*jwtw.MyClaims[userData])
			return &Result{UserID: c.Data.ID, Username: c.Data.Username, Data: c.Data}
		},
	})

	user := userData{ID: "123", Username: "andree"}
	claims := jwtw.NewClaims(user, 5*time.Minute, "test", "auth")
	tokenStr, _ := mgr.Create(claims)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	result, err := authenticator.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if result.UserID != "123" {
		t.Errorf("Expected UserID %q, got %q", "123", result.UserID)
	}
	if result.Username != "andree" {
		t.Errorf("Expected Username %q, got %q", "andree", result.Username)
	}
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	mgr := jwtw.New(&jwtw.Config{SecretKey: "secret"})
	authenticator := NewJWT(&JWTConfig{
		JWT:       mgr,
		NewClaims: func() jwtv5.Claims { return &jwtw.MyClaims[userData]{} },
		ExtractResult: func(claims jwtv5.Claims) *Result {
			return &Result{}
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatal("Expected error for missing header, got nil")
	}
}

func TestJWTAuth_InvalidFormat(t *testing.T) {
	mgr := jwtw.New(&jwtw.Config{SecretKey: "secret"})
	authenticator := NewJWT(&JWTConfig{
		JWT:       mgr,
		NewClaims: func() jwtv5.Claims { return &jwtw.MyClaims[userData]{} },
		ExtractResult: func(claims jwtv5.Claims) *Result {
			return &Result{}
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic some-credentials")
	_, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatal("Expected error for wrong auth scheme, got nil")
	}
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	mgr := jwtw.New(&jwtw.Config{SecretKey: "secret"})
	authenticator := NewJWT(&JWTConfig{
		JWT:       mgr,
		NewClaims: func() jwtv5.Claims { return &jwtw.MyClaims[userData]{} },
		ExtractResult: func(claims jwtv5.Claims) *Result {
			return &Result{}
		},
	})

	user := userData{ID: "123", Username: "andree"}
	claims := jwtw.NewClaims(user, -1*time.Minute, "test", "auth")
	tokenStr, _ := mgr.Create(claims)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	_, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatal("Expected error for expired token, got nil")
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	mgr := jwtw.New(&jwtw.Config{SecretKey: "secret"})
	authenticator := NewJWT(&JWTConfig{
		JWT:       mgr,
		NewClaims: func() jwtv5.Claims { return &jwtw.MyClaims[userData]{} },
		ExtractResult: func(claims jwtv5.Claims) *Result {
			return &Result{}
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer garbage-token")
	_, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatal("Expected error for invalid token, got nil")
	}
}

func TestBasicAuth_Static_Success(t *testing.T) {
	authenticator := NewBasicAuth(&BasicAuthConfig{
		StaticUsers: map[string]string{"admin": "password123"},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "password123")

	result, err := authenticator.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if result.UserID != "admin" {
		t.Errorf("Expected UserID %q, got %q", "admin", result.UserID)
	}
	if result.Username != "admin" {
		t.Errorf("Expected Username %q, got %q", "admin", result.Username)
	}
}

func TestBasicAuth_Static_InvalidPassword(t *testing.T) {
	authenticator := NewBasicAuth(&BasicAuthConfig{
		StaticUsers: map[string]string{"admin": "password123"},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "wrong")

	_, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatal("Expected error for wrong password, got nil")
	}
}

func TestBasicAuth_Static_UnknownUser(t *testing.T) {
	authenticator := NewBasicAuth(&BasicAuthConfig{
		StaticUsers: map[string]string{"admin": "password123"},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("unknown", "password123")

	_, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatal("Expected error for unknown user, got nil")
	}
}

func TestBasicAuth_CustomValidator_Success(t *testing.T) {
	authenticator := NewBasicAuth(&BasicAuthConfig{
		Validator: func(username, password string) (*Result, error) {
			if username == "service" && password == "token" {
				return &Result{UserID: username, Username: username}, nil
			}
			return nil, statusw.InvalidCredential.WithCustomMessage("Invalid credentials")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("service", "token")

	result, err := authenticator.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if result.UserID != "service" {
		t.Errorf("Expected UserID %q, got %q", "service", result.UserID)
	}
}

func TestBasicAuth_CustomValidator_Failure(t *testing.T) {
	authenticator := NewBasicAuth(&BasicAuthConfig{
		Validator: func(username, password string) (*Result, error) {
			return nil, statusw.InvalidCredential.WithCustomMessage("Invalid credentials")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("user", "wrong")

	_, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatal("Expected error from custom validator, got nil")
	}
}

func TestBasicAuth_MissingHeader(t *testing.T) {
	authenticator := NewBasicAuth(&BasicAuthConfig{
		StaticUsers: map[string]string{"admin": "password123"},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := authenticator.Authenticate(req)
	if err == nil {
		t.Fatal("Expected error for missing header, got nil")
	}
}