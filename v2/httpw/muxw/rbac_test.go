package muxw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AndreeJait/go-utility/v2/authw"
	"github.com/gorilla/mux"
)

func TestMux_RequireRole_Success(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{
		RoleFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"admin"}, nil
		},
	})

	r := New(&Config{DebugMode: true})
	mockAuth := &muxMockAuth{result: &authw.Result{UserID: "user-1", Username: "tester"}}
	r.Use(AuthMiddleware(mockAuth))
	r.Use(RequireRole(rbac, "admin"))

	r.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", rec.Code)
	}
}

func TestMux_RequireRole_Denied(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{
		RoleFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"viewer"}, nil
		},
	})

	r := New(&Config{DebugMode: true})
	mockAuth := &muxMockAuth{result: &authw.Result{UserID: "user-1", Username: "tester"}}
	r.Use(AuthMiddleware(mockAuth))
	r.Use(RequireRole(rbac, "admin"))

	r.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected HTTP 403, got %d", rec.Code)
	}
}

func TestMux_RequirePermission_Success(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{
		PermissionFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"users:write"}, nil
		},
	})

	r := New(&Config{DebugMode: true})
	mockAuth := &muxMockAuth{result: &authw.Result{UserID: "user-1", Username: "tester"}}
	r.Use(AuthMiddleware(mockAuth))
	r.Use(RequirePermission(rbac, "users:write"))

	r.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected HTTP 200, got %d", rec.Code)
	}
}

func TestMux_RequirePermission_Denied(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{
		PermissionFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"users:read"}, nil
		},
	})

	r := New(&Config{DebugMode: true})
	mockAuth := &muxMockAuth{result: &authw.Result{UserID: "user-1", Username: "tester"}}
	r.Use(AuthMiddleware(mockAuth))
	r.Use(RequirePermission(rbac, "users:write"))

	r.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected HTTP 403, got %d", rec.Code)
	}
}

func TestMux_RequireRole_NoAuth(t *testing.T) {
	rbac := authw.NewRBAC(&authw.RBACConfig{})

	r := mux.NewRouter()
	r.Use(RequireRole(rbac, "admin"))
	r.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected HTTP 401 when not authenticated, got %d", rec.Code)
	}
}