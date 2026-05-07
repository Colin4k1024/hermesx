package middleware

import (
	"net/http"

	"github.com/Colin4k1024/hermesx/internal/auth"
)

// RequireScope returns a middleware that enforces the given scope on the
// authenticated request. If the AuthContext has no scopes defined (empty slice),
// the request is allowed through for legacy compatibility. Otherwise the
// required scope must be present or a 403 Forbidden is returned.
func RequireScope(scope string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := auth.FromContext(r.Context())
			if !ok || ac == nil {
				http.Error(w, "authorization required", http.StatusUnauthorized)
				return
			}

			if !ac.HasScope(scope) {
				http.Error(w, "insufficient scope", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
