package echow

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AndreeJait/go-utility/v2/authw"
	"github.com/AndreeJait/go-utility/v2/jwtw"
	"github.com/AndreeJait/go-utility/v2/statusw"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	echov5 "github.com/labstack/echo/v5"
)

type echoUserData struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type echoMockAuth struct {
	result *authw.Result
	err    error
}

func (m *echoMockAuth) Authenticate(r *http.Request) (*authw.Result, error) {
	return m.result, m.err
}

func TestEcho_AuthMiddleware_Success(t *testing.T) {
	e := New(&Config{DebugMode: true})

	mock := &echoMockAuth{result: &authw.Result{UserID: "123", Username: "testuser"}}
	e.Use(AuthMiddleware(mock))

	e.GET("/protected", func(c *echov5.Context) error {
		authResult := authw.FromContext(c.Request().Context())
		return c.JSON(http.StatusOK, map[string]string{"user_id": authResult.GetUserID()})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", rec.Code)
	}
}

func TestEcho_AuthMiddleware_Failure(t *testing.T) {
	e := New(&Config{DebugMode: true})

	failAuth := &echoMockAuth{err: statusw.InvalidCredential.WithCustomMessage("Login required")}
	e.Use(AuthMiddleware(failAuth))

	e.GET("/protected", func(c *echov5.Context) error {
		return c.JSON(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected HTTP 401, got %d", rec.Code)
	}
}

func TestEcho_AuthMiddleware_ResultInContext(t *testing.T) {
	e := New(&Config{DebugMode: true})

	expectedResult := &authw.Result{UserID: "user-42", Username: "alice", Data: map[string]string{"role": "admin"}}
	mock := &echoMockAuth{result: expectedResult}
	e.Use(AuthMiddleware(mock))

	var capturedResult *authw.Result
	e.GET("/check", func(c *echov5.Context) error {
		capturedResult = authw.FromContext(c.Request().Context())
		return c.JSON(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/check", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if capturedResult == nil {
		t.Fatal("Expected auth result in context, got nil")
	}
	if capturedResult.UserID != "user-42" {
		t.Errorf("Expected UserID %q, got %q", "user-42", capturedResult.UserID)
	}
}

func TestEcho_AuthMiddleware_JWT_Integration(t *testing.T) {
	e := New(&Config{DebugMode: true})

	mgr := jwtw.New(&jwtw.Config{SecretKey: "test-secret"})
	jwtAuth := authw.NewJWT(&authw.JWTConfig{
		JWT:       mgr,
		NewClaims: func() jwtv5.Claims { return &jwtw.MyClaims[echoUserData]{} },
		ExtractResult: func(claims jwtv5.Claims) *authw.Result {
			c := claims.(*jwtw.MyClaims[echoUserData])
			return &authw.Result{UserID: c.Data.ID, Username: c.Data.Username, Data: c.Data}
		},
	})

	e.Use(AuthMiddleware(jwtAuth))

	e.GET("/profile", func(c *echov5.Context) error {
		authResult := authw.FromContext(c.Request().Context())
		return c.JSON(http.StatusOK, map[string]string{"user": authResult.GetUserID()})
	})

	// Create a valid token
	user := echoUserData{ID: "uid-1", Username: "bob"}
	claims := jwtw.NewClaims(user, 5*time.Minute, "test", "auth")
	tokenStr, _ := mgr.Create(claims)

	// Request with valid Bearer token
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", rec.Code)
	}

	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["user"] != "uid-1" {
		t.Errorf("Expected user %q, got %q", "uid-1", resp["user"])
	}

	// Request without token → 401
	reqNoToken := httptest.NewRequest(http.MethodGet, "/profile", nil)
	recNoToken := httptest.NewRecorder()
	e.ServeHTTP(recNoToken, reqNoToken)

	if recNoToken.Code != http.StatusUnauthorized {
		t.Errorf("Expected HTTP 401 for missing token, got %d", recNoToken.Code)
	}
}