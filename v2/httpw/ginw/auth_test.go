package ginw

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
	"github.com/gin-gonic/gin"
)

type ginUserData struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type ginMockAuth struct {
	result *authw.Result
	err    error
}

func (m *ginMockAuth) Authenticate(r *http.Request) (*authw.Result, error) {
	return m.result, m.err
}

func setupGinWithAuth(authenticator authw.Authenticator) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := New(&Config{DebugMode: true})
	r.Use(AuthMiddleware(authenticator))
	return r
}

func TestGin_AuthMiddleware_Success(t *testing.T) {
	r := setupGinWithAuth(&ginMockAuth{result: &authw.Result{UserID: "123", Username: "testuser"}})

	r.GET("/protected", func(c *gin.Context) {
		authResult := authw.FromContext(c.Request.Context())
		c.JSON(http.StatusOK, map[string]string{"user_id": authResult.GetUserID()})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", rec.Code)
	}
}

func TestGin_AuthMiddleware_Failure(t *testing.T) {
	r := setupGinWithAuth(&ginMockAuth{err: statusw.InvalidCredential.WithCustomMessage("Login required")})

	r.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected HTTP 401, got %d", rec.Code)
	}
}

func TestGin_AuthMiddleware_ResultInContext(t *testing.T) {
	expectedResult := &authw.Result{UserID: "user-42", Username: "alice", Data: map[string]string{"role": "admin"}}
	r := setupGinWithAuth(&ginMockAuth{result: expectedResult})

	var capturedResult *authw.Result
	r.GET("/check", func(c *gin.Context) {
		capturedResult = authw.FromContext(c.Request.Context())
		c.JSON(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/check", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if capturedResult == nil {
		t.Fatal("Expected auth result in context, got nil")
	}
	if capturedResult.UserID != "user-42" {
		t.Errorf("Expected UserID %q, got %q", "user-42", capturedResult.UserID)
	}
}

func TestGin_AuthMiddleware_JWT_Integration(t *testing.T) {
	mgr := jwtw.New(&jwtw.Config{SecretKey: "test-secret"})
	jwtAuth := authw.NewJWT(&authw.JWTConfig{
		JWT:       mgr,
		NewClaims: func() jwtv5.Claims { return &jwtw.MyClaims[ginUserData]{} },
		ExtractResult: func(claims jwtv5.Claims) *authw.Result {
			c := claims.(*jwtw.MyClaims[ginUserData])
			return &authw.Result{UserID: c.Data.ID, Username: c.Data.Username, Data: c.Data}
		},
	})

	r := setupGinWithAuth(jwtAuth)

	r.GET("/profile", func(c *gin.Context) {
		authResult := authw.FromContext(c.Request.Context())
		c.JSON(http.StatusOK, map[string]string{"user": authResult.GetUserID()})
	})

	// Create a valid token
	user := ginUserData{ID: "uid-1", Username: "bob"}
	claims := jwtw.NewClaims(user, 5*time.Minute, "test", "auth")
	tokenStr, _ := mgr.Create(claims)

	// Request with valid Bearer token
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

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
	r.ServeHTTP(recNoToken, reqNoToken)

	if recNoToken.Code != http.StatusUnauthorized {
		t.Errorf("Expected HTTP 401 for missing token, got %d", recNoToken.Code)
	}
}