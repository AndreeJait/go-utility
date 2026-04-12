package authw

import (
	"context"
	"testing"

	"github.com/AndreeJait/go-utility/v2/localcachew"
)

func TestRBAC_RegisterRole(t *testing.T) {
	rbac := NewRBAC(&RBACConfig{})
	rbac.RegisterRole("admin", "users:read", "users:write", "users:delete")
	rbac.RegisterRole("editor", "users:read", "users:write")

	rbac.mu.RLock()
	adminPerms := rbac.rolePerms["admin"]
	editorPerms := rbac.rolePerms["editor"]
	rbac.mu.RUnlock()

	if len(adminPerms) != 3 {
		t.Errorf("Expected 3 admin permissions, got %d", len(adminPerms))
	}
	if len(editorPerms) != 2 {
		t.Errorf("Expected 2 editor permissions, got %d", len(editorPerms))
	}
}

func TestRBAC_CheckRole_Success(t *testing.T) {
	rbac := NewRBAC(&RBACConfig{
		RoleFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"admin", "editor"}, nil
		},
	})

	// Inject auth result into context
	result := &Result{UserID: "user-1", Roles: []string{"admin", "editor"}}
	ctx := WithResult(context.Background(), result)

	ok, err := rbac.CheckRole(ctx, "user-1", "admin")
	if err != nil {
		t.Fatalf("CheckRole failed: %v", err)
	}
	if !ok {
		t.Error("Expected CheckRole to return true for 'admin'")
	}
}

func TestRBAC_CheckRole_FromContext(t *testing.T) {
	rbac := NewRBAC(&RBACConfig{}) // no fetcher needed if roles in context

	result := &Result{UserID: "user-1", Roles: []string{"viewer"}}
	ctx := WithResult(context.Background(), result)

	ok, err := rbac.CheckRole(ctx, "user-1", "viewer")
	if err != nil {
		t.Fatalf("CheckRole failed: %v", err)
	}
	if !ok {
		t.Error("Expected CheckRole to return true for 'viewer'")
	}
}

func TestRBAC_CheckRole_Denied(t *testing.T) {
	rbac := NewRBAC(&RBACConfig{
		RoleFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"viewer"}, nil
		},
	})

	result := &Result{UserID: "user-1"}
	ctx := WithResult(context.Background(), result)

	ok, err := rbac.CheckRole(ctx, "user-1", "admin")
	if err != nil {
		t.Fatalf("CheckRole failed: %v", err)
	}
	if ok {
		t.Error("Expected CheckRole to return false for 'admin'")
	}
}

func TestRBAC_CheckRole_EmptyUserID(t *testing.T) {
	rbac := NewRBAC(&RBACConfig{})
	ok, err := rbac.CheckRole(context.Background(), "", "admin")
	if ok {
		t.Error("Expected CheckRole to return false for empty userID")
	}
	if err == nil {
		t.Fatal("Expected error for empty userID")
	}
}

func TestRBAC_CheckPermission_WithFetcher(t *testing.T) {
	rbac := NewRBAC(&RBACConfig{
		PermissionFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"users:read", "users:write"}, nil
		},
	})

	result := &Result{UserID: "user-1"}
	ctx := WithResult(context.Background(), result)

	ok, err := rbac.CheckPermission(ctx, "user-1", "users:write")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if !ok {
		t.Error("Expected CheckPermission to return true for 'users:write'")
	}
}

func TestRBAC_CheckPermission_FromContext(t *testing.T) {
	rbac := NewRBAC(&RBACConfig{}) // no fetcher needed if perms in context

	result := &Result{UserID: "user-1", Permissions: []string{"users:read"}}
	ctx := WithResult(context.Background(), result)

	ok, err := rbac.CheckPermission(ctx, "user-1", "users:read")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if !ok {
		t.Error("Expected CheckPermission to return true for 'users:read'")
	}
}

func TestRBAC_CheckPermission_DerivedFromRoles(t *testing.T) {
	rbac := NewRBAC(&RBACConfig{
		RoleFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"editor"}, nil
		},
	})
	rbac.RegisterRole("editor", "users:read", "users:write")

	result := &Result{UserID: "user-1"}
	ctx := WithResult(context.Background(), result)

	ok, err := rbac.CheckPermission(ctx, "user-1", "users:write")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if !ok {
		t.Error("Expected CheckPermission to return true — derived from editor role")
	}

	ok, err = rbac.CheckPermission(ctx, "user-1", "users:delete")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if ok {
		t.Error("Expected CheckPermission to return false for 'users:delete'")
	}
}

func TestRBAC_CheckPermission_Denied(t *testing.T) {
	rbac := NewRBAC(&RBACConfig{
		PermissionFetcher: func(ctx context.Context, userID string) ([]string, error) {
			return []string{"users:read"}, nil
		},
	})

	result := &Result{UserID: "user-1"}
	ctx := WithResult(context.Background(), result)

	ok, err := rbac.CheckPermission(ctx, "user-1", "users:delete")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if ok {
		t.Error("Expected CheckPermission to return false for 'users:delete'")
	}
}

func TestRBAC_NoFetcher(t *testing.T) {
	rbac := NewRBAC(&RBACConfig{}) // no fetchers

	result := &Result{UserID: "user-1"}
	ctx := WithResult(context.Background(), result)

	_, err := rbac.GetRoles(ctx, "user-1")
	if err == nil {
		t.Fatal("Expected error when no role fetcher configured")
	}
}

func TestRBAC_WithCache(t *testing.T) {
	localcachew.Clear() // clean slate
	fetchCount := 0
	rbac := NewRBAC(&RBACConfig{
		RoleFetcher: func(ctx context.Context, userID string) ([]string, error) {
			fetchCount++
			return []string{"admin"}, nil
		},
		Cache:    NewLocalCache(),
		CacheTTL: 60,
	})

	// First call — should fetch and cache
	result := &Result{UserID: "cache-user"}
	ctx := WithResult(context.Background(), result)

	roles, err := rbac.GetRoles(ctx, "cache-user")
	if err != nil {
		t.Fatalf("GetRoles failed: %v", err)
	}
	if len(roles) != 1 || roles[0] != "admin" {
		t.Errorf("Expected roles [admin], got %v", roles)
	}
	if fetchCount != 1 {
		t.Errorf("Expected 1 fetch call, got %d", fetchCount)
	}

	// Second call with fresh context (no Roles in Result) — should hit cache
	result2 := &Result{UserID: "cache-user"}
	ctx2 := WithResult(context.Background(), result2)

	roles2, err := rbac.GetRoles(ctx2, "cache-user")
	if err != nil {
		t.Fatalf("GetRoles (cached) failed: %v", err)
	}
	if len(roles2) != 1 || roles2[0] != "admin" {
		t.Errorf("Expected roles [admin] from cache, got %v", roles2)
	}
	if fetchCount != 1 {
		t.Errorf("Expected fetch count to stay 1 (cache hit), got %d", fetchCount)
	}
}

func TestRBAC_InvalidateUser(t *testing.T) {
	localcachew.Clear() // clean slate
	fetchCount := 0
	rbac := NewRBAC(&RBACConfig{
		RoleFetcher: func(ctx context.Context, userID string) ([]string, error) {
			fetchCount++
			return []string{"admin"}, nil
		},
		Cache:    NewLocalCache(),
		CacheTTL: 60,
	})

	result := &Result{UserID: "inv-user"}
	ctx := WithResult(context.Background(), result)

	// First call — fetches and caches
	rbac.GetRoles(ctx, "inv-user")
	if fetchCount != 1 {
		t.Errorf("Expected 1 fetch, got %d", fetchCount)
	}

	// Invalidate
	rbac.InvalidateUser(ctx, "inv-user")

	// Second call — should fetch again (cache was invalidated)
	result2 := &Result{UserID: "inv-user"}
	ctx2 := WithResult(context.Background(), result2)
	rbac.GetRoles(ctx2, "inv-user")
	if fetchCount != 2 {
		t.Errorf("Expected 2 fetches after invalidation, got %d", fetchCount)
	}
}