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

// RequireAnyScope returns a middleware that enforces one of the supplied scopes.
// The explicit "admin" scope is always accepted as a break-glass compatibility
// grant. Unlike AuthContext.HasScope, empty legacy scopes do not satisfy these
// domain-specific admin checks.
func RequireAnyScope(scopes ...string) Middleware {
	allowed := append([]string{"admin"}, scopes...)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := auth.FromContext(r.Context())
			if !ok || ac == nil {
				http.Error(w, "authorization required", http.StatusUnauthorized)
				return
			}

			for _, scope := range allowed {
				if hasExplicitScope(ac, scope) {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "insufficient scope", http.StatusForbidden)
		})
	}
}

func hasExplicitScope(ac *auth.AuthContext, scope string) bool {
	for _, s := range ac.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}
