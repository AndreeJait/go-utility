package echow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AndreeJait/go-utility/v2/authw"
	echov5 "github.com/labstack/echo/v5"
)

type echoRBACMock struct {
	roleOk       bool
	permOk       bool
	roleErr      error
	permErr      error
	fetchRoleErr error
}

func (m *echoRBACMock) CheckRole(ctx context.Context, userID string, role string) (bool, error) {
	if m.fetchRoleErr != nil {
		return false, m.fetchRoleErr
	}
	return m.roleOk, m.roleErr
}

func (m *echoRBACMock) CheckPermission(ctx context.Context, userID string, permission string) (bool, error) {
	return m.permOk, m.permErr
}

func setupEchoWithAuthAndRBAC(rbac *authw.RBAC) *echov5.Echo {
	e := New(&Config{DebugMode: true})

	// Apply auth middleware that injects a mock result
	mockAuth := &echoMockAuth{result: &authw.Result{UserID: "user-1", Username: "tester"}}
	e.Use(AuthMiddleware(mockAuth))

	return e
}

func TestEcho_RequireRole_Success(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{
		RoleFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"admin"}, nil
		},
	})

	e := setupEchoWithAuthAndRBAC(rbac)

	e.GET("/admin", func(c *echov5.Context) error {
		return c.JSON(http.StatusOK, "ok")
	}, RequireRole(rbac, "admin"))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", rec.Code)
	}
}

func TestEcho_RequireRole_Denied(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{
		RoleFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"viewer"}, nil
		},
	})

	e := setupEchoWithAuthAndRBAC(rbac)

	e.GET("/admin", func(c *echov5.Context) error {
		return c.JSON(http.StatusOK, "ok")
	}, RequireRole(rbac, "admin"))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected HTTP 403, got %d", rec.Code)
	}
}

func TestEcho_RequirePermission_Success(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{
		PermissionFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"users:write"}, nil
		},
	})

	e := setupEchoWithAuthAndRBAC(rbac)

	e.POST("/users", func(c *echov5.Context) error {
		return c.JSON(http.StatusOK, "ok")
	}, RequirePermission(rbac, "users:write"))

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", rec.Code)
	}
}

func TestEcho_RequirePermission_Denied(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{
		PermissionFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"users:read"}, nil
		},
	})

	e := setupEchoWithAuthAndRBAC(rbac)

	e.POST("/users", func(c *echov5.Context) error {
		return c.JSON(http.StatusOK, "ok")
	}, RequirePermission(rbac, "users:write"))

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected HTTP 403, got %d", rec.Code)
	}
}

func TestEcho_RequireRole_NoAuth(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{})

	// No AuthMiddleware applied — context has no auth result
	e := New(&Config{DebugMode: true})
	e.GET("/admin", func(c *echov5.Context) error {
		return c.JSON(http.StatusOK, "ok")
	}, RequireRole(rbac, "admin"))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected HTTP 401 when not authenticated, got %d", rec.Code)
	}
}