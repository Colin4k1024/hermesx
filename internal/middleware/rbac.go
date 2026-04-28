package middleware

import (
	"net/http"

	"github.com/hermes-agent/hermes-agent-go/internal/auth"
)

// RBACConfig maps route patterns to required roles.
type RBACConfig struct {
	DefaultRole string
	Rules       map[string]string // path prefix → required role, e.g. "/v1/admin" → "admin"
}

// RBACMiddleware checks that the authenticated user has the required role.
// Requests with no AuthContext are rejected with 401.
// Requests with insufficient role are rejected with 403.
func RBACMiddleware(cfg RBACConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := auth.FromContext(r.Context())
			if !ok || ac == nil {
				http.Error(w, "authorization required", http.StatusUnauthorized)
				return
			}

			required := cfg.DefaultRole
			for prefix, role := range cfg.Rules {
				if len(r.URL.Path) >= len(prefix) && r.URL.Path[:len(prefix)] == prefix {
					required = role
					break
				}
			}

			if required != "" && !ac.HasRole(required) && !ac.HasRole("admin") {
				http.Error(w, "insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
