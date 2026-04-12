package ginw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AndreeJait/go-utility/v2/authw"
	"github.com/gin-gonic/gin"
)

func setupGinWithAuthAndRBAC(rbac *authw.RBAC) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := New(&Config{DebugMode: true})

	mockAuth := &ginMockAuth{result: &authw.Result{UserID: "user-1", Username: "tester"}}
	r.Use(AuthMiddleware(mockAuth))

	return r
}

func TestGin_RequireRole_Success(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{
		RoleFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"admin"}, nil
		},
	})

	r := setupGinWithAuthAndRBAC(rbac)
	r.GET("/admin", RequireRole(rbac, "admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", rec.Code)
	}
}

func TestGin_RequireRole_Denied(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{
		RoleFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"viewer"}, nil
		},
	})

	r := setupGinWithAuthAndRBAC(rbac)
	r.GET("/admin", RequireRole(rbac, "admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected HTTP 403, got %d", rec.Code)
	}
}

func TestGin_RequirePermission_Success(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{
		PermissionFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"users:write"}, nil
		},
	})

	r := setupGinWithAuthAndRBAC(rbac)
	r.POST("/users", RequirePermission(rbac, "users:write"), func(c *gin.Context) {
		c.JSON(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", rec.Code)
	}
}

func TestGin_RequirePermission_Denied(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{
		PermissionFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"users:read"}, nil
		},
	})

	r := setupGinWithAuthAndRBAC(rbac)
	r.POST("/users", RequirePermission(rbac, "users:write"), func(c *gin.Context) {
		c.JSON(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected HTTP 403, got %d", rec.Code)
	}
}

func TestGin_RequireRole_NoAuth(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{})

	gin.SetMode(gin.TestMode)
	r := New(&Config{DebugMode: true})
	r.GET("/admin", RequireRole(rbac, "admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected HTTP 401 when not authenticated, got %d", rec.Code)
	}
}