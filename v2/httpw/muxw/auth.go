package muxw

import (
	"encoding/json"
	"net/http"

	"github.com/AndreeJait/go-utility/v2/authw"
	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/AndreeJait/go-utility/v2/responsew"
)

// AuthMiddleware returns a standard net/http middleware that authenticates
// every request using the provided Authenticator. On success, the auth Result
// is injected into the request context (accessible via authw.FromContext).
// On failure, it writes a 401 JSON error response and stops the chain.
func AuthMiddleware(authenticator authw.Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			result, err := authenticator.Authenticate(r)
			if err != nil {
				logw.CtxInfof(r.Context(), "[MUX-AUTH] authentication failed: %v", err)

				if globalErrorHandler != nil {
					globalErrorHandler(w, r, err)
					return
				}

				httpCode, payload := responsew.Error(err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(httpCode)
				json.NewEncoder(w).Encode(payload)
				return
			}

			ctx := authw.WithResult(r.Context(), result)
			logw.CtxInfof(ctx, "[MUX-AUTH] authenticated user: %s", result.GetUserID())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}