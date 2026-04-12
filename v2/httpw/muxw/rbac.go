package muxw

import (
	"encoding/json"
	"net/http"

	"github.com/AndreeJait/go-utility/v2/authw"
	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/AndreeJait/go-utility/v2/responsew"
	"github.com/AndreeJait/go-utility/v2/statusw"
)

// RequireRole returns a Mux middleware that checks if the authenticated user
// has the specified role. Must be used after AuthMiddleware so the auth Result
// is available in context. Returns 401 if not authenticated, 403 if unauthorized.
func RequireRole(rbac *authw.RBAC, role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			result := authw.FromContext(r.Context())
			if result == nil {
				writeAuthzError(w, r, statusw.InvalidCredential.WithCustomMessage("Authentication required"))
				return
			}

			ok, err := rbac.CheckRole(r.Context(), result.GetUserID(), role)
			if err != nil {
				logw.CtxErrorf(r.Context(), "[MUX-RBAC] role check error: %v", err)
				writeAuthzError(w, r, err)
				return
			}
			if !ok {
				logw.CtxInfof(r.Context(), "[MUX-RBAC] access denied: role %q required", role)
				writeAuthzError(w, r, statusw.InvalidAccess.WithCustomMessage("Insufficient role: "+role))
				return
			}

			logw.CtxInfof(r.Context(), "[MUX-RBAC] role check passed: %s", role)
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission returns a Mux middleware that checks if the authenticated user
// has the specified permission. Must be used after AuthMiddleware so the auth Result
// is available in context. Returns 401 if not authenticated, 403 if unauthorized.
func RequirePermission(rbac *authw.RBAC, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			result := authw.FromContext(r.Context())
			if result == nil {
				writeAuthzError(w, r, statusw.InvalidCredential.WithCustomMessage("Authentication required"))
				return
			}

			ok, err := rbac.CheckPermission(r.Context(), result.GetUserID(), permission)
			if err != nil {
				logw.CtxErrorf(r.Context(), "[MUX-RBAC] permission check error: %v", err)
				writeAuthzError(w, r, err)
				return
			}
			if !ok {
				logw.CtxInfof(r.Context(), "[MUX-RBAC] access denied: permission %q required", permission)
				writeAuthzError(w, r, statusw.InvalidAccess.WithCustomMessage("Insufficient permission: "+permission))
				return
			}

			logw.CtxInfof(r.Context(), "[MUX-RBAC] permission check passed: %s", permission)
			next.ServeHTTP(w, r)
		})
	}
}

// writeAuthzError writes an authorization error response using the existing error pipeline.
func writeAuthzError(w http.ResponseWriter, r *http.Request, err error) {
	if globalErrorHandler != nil {
		globalErrorHandler(w, r, err)
		return
	}

	httpCode, payload := responsew.Error(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(payload)
}