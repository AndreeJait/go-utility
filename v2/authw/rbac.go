package authw

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/AndreeJait/go-utility/v2/statusw"
)

// Cache defines the interface for a key-value cache used by RBAC
// to store and retrieve user roles and permissions.
type Cache interface {
	Get(ctx context.Context, key string) ([]string, error)
	Set(ctx context.Context, key string, values []string, ttlSeconds int64) error
	Delete(ctx context.Context, key string) error
}

// RBACConfig holds the configuration for the RBAC authorization system.
type RBACConfig struct {
	// RoleFetcher fetches user roles from an external source (e.g., database).
	// Required for CheckRole to work with dynamic data.
	RoleFetcher func(ctx context.Context, userID string) ([]string, error)

	// PermissionFetcher fetches user permissions from an external source.
	// Optional: if nil, permissions are derived from registered role→permission mappings
	// plus any permissions already in the auth Result.
	PermissionFetcher func(ctx context.Context, userID string) ([]string, error)

	// Cache stores role/permission data to avoid repeated lookups.
	// Optional: if nil, no caching is used.
	Cache Cache

	// CacheTTL is the time-to-live in seconds for cached role/permission entries.
	// Defaults to 300 (5 minutes) if Cache is set and CacheTTL is 0.
	CacheTTL int64
}

// RBAC provides role-based and permission-based authorization with optional caching.
// It supports two data sources: user-provided fetcher functions and a registration API
// for defining role→permission mappings.
type RBAC struct {
	roleFetcher       func(ctx context.Context, userID string) ([]string, error)
	permissionFetcher func(ctx context.Context, userID string) ([]string, error)
	cache             Cache
	cacheTTL          int64
	rolePerms         map[string][]string // registered role→permissions
	mu                sync.RWMutex
}

// NewRBAC creates a new RBAC authorization instance.
func NewRBAC(cfg *RBACConfig) *RBAC {
	ttl := cfg.CacheTTL
	if ttl == 0 && cfg.Cache != nil {
		ttl = 300 // default 5 minutes
	}
	return &RBAC{
		roleFetcher:       cfg.RoleFetcher,
		permissionFetcher: cfg.PermissionFetcher,
		cache:             cfg.Cache,
		cacheTTL:          ttl,
		rolePerms:         make(map[string][]string),
	}
}

// RegisterRole defines the permissions associated with a role.
// This mapping is used by CheckPermission when no PermissionFetcher is provided
// to derive permissions from the user's roles.
func (r *RBAC) RegisterRole(role string, permissions ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rolePerms[role] = append(r.rolePerms[role], permissions...)
}

// CheckRole verifies that a user has a specific role.
// It first checks the auth Result in context, then the cache, then calls the RoleFetcher.
func (r *RBAC) CheckRole(ctx context.Context, userID string, requiredRole string) (bool, error) {
	if userID == "" {
		return false, statusw.InvalidCredential.WithCustomMessage("Authentication required")
	}

	roles, err := r.GetRoles(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, role := range roles {
		if role == requiredRole {
			return true, nil
		}
	}
	return false, nil
}

// CheckPermission verifies that a user has a specific permission.
// It first checks the auth Result in context, then the cache, then calls
// the PermissionFetcher (or derives from roles + registered mappings).
func (r *RBAC) CheckPermission(ctx context.Context, userID string, requiredPermission string) (bool, error) {
	if userID == "" {
		return false, statusw.InvalidCredential.WithCustomMessage("Authentication required")
	}

	perms, err := r.GetPermissions(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, perm := range perms {
		if perm == requiredPermission {
			return true, nil
		}
	}
	return false, nil
}

// GetRoles returns the roles for a user. It checks the auth Result in context first,
// then the cache, then calls the RoleFetcher.
func (r *RBAC) GetRoles(ctx context.Context, userID string) ([]string, error) {
	// 1. Check context result first
	if result := FromContext(ctx); result != nil && len(result.Roles) > 0 {
		return result.Roles, nil
	}

	// 2. Try cache
	if r.cache != nil {
		cacheKey := fmt.Sprintf("rbac:roles:%s", userID)
		if roles, err := r.cache.Get(ctx, cacheKey); err == nil {
			return roles, nil
		}
	}

	// 3. Fetch from source
	if r.roleFetcher == nil {
		return nil, statusw.InvalidAccess.WithCustomMessage("No role fetcher configured")
	}

	roles, err := r.roleFetcher(ctx, userID)
	if err != nil {
		return nil, statusw.InvalidAccess.WithError(err).WithCustomMessage("Failed to fetch user roles")
	}

	// 4. Cache the result
	if r.cache != nil {
		cacheKey := fmt.Sprintf("rbac:roles:%s", userID)
		_ = r.cache.Set(ctx, cacheKey, roles, r.cacheTTL)
	}

	return roles, nil
}

// GetPermissions returns the permissions for a user. It checks the auth Result in context first,
// then the cache, then calls the PermissionFetcher or derives from roles + registered mappings.
func (r *RBAC) GetPermissions(ctx context.Context, userID string) ([]string, error) {
	// 1. Check context result first
	if result := FromContext(ctx); result != nil && len(result.Permissions) > 0 {
		return result.Permissions, nil
	}

	// 2. Try cache
	if r.cache != nil {
		cacheKey := fmt.Sprintf("rbac:perms:%s", userID)
		if perms, err := r.cache.Get(ctx, cacheKey); err == nil {
			return perms, nil
		}
	}

	// 3. Fetch from source
	var perms []string
	if r.permissionFetcher != nil {
		fetched, err := r.permissionFetcher(ctx, userID)
		if err != nil {
			return nil, statusw.InvalidAccess.WithError(err).WithCustomMessage("Failed to fetch user permissions")
		}
		perms = fetched
	} else {
		// Derive permissions from roles + registered role→permission mapping
		roles, err := r.GetRoles(ctx, userID)
		if err != nil {
			return nil, err
		}
		permSet := make(map[string]struct{})
		r.mu.RLock()
		for _, role := range roles {
			if rolePerms, ok := r.rolePerms[role]; ok {
				for _, p := range rolePerms {
					permSet[p] = struct{}{}
				}
			}
		}
		r.mu.RUnlock()

		// Also include permissions from the auth Result
		if result := FromContext(ctx); result != nil {
			for _, p := range result.Permissions {
				permSet[p] = struct{}{}
			}
		}

		for p := range permSet {
			perms = append(perms, p)
		}
	}

	// 4. Cache the result
	if r.cache != nil {
		cacheKey := fmt.Sprintf("rbac:perms:%s", userID)
		_ = r.cache.Set(ctx, cacheKey, perms, r.cacheTTL)
	}

	return perms, nil
}

// InvalidateUser clears cached roles and permissions for a user.
// Call this when a user's roles or permissions change.
func (r *RBAC) InvalidateUser(ctx context.Context, userID string) error {
	if r.cache == nil {
		return nil
	}
	rolesKey := fmt.Sprintf("rbac:roles:%s", userID)
	permsKey := fmt.Sprintf("rbac:perms:%s", userID)
	if err := r.cache.Delete(ctx, rolesKey); err != nil {
		// Ignore cache miss errors, only return real errors
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
	}
	if err := r.cache.Delete(ctx, permsKey); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
	}
	return nil
}