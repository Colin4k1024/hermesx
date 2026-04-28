package middleware

import (
	"log/slog"
	"net/http"

	"github.com/hermes-agent/hermes-agent-go/internal/auth"
	"github.com/hermes-agent/hermes-agent-go/internal/store"
)

// AuditMiddleware logs key actions to the audit store.
// Captures method + path after the request completes.
func AuditMiddleware(auditStore store.AuditLogStore) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)

			ac, ok := auth.FromContext(r.Context())
			if !ok || ac == nil {
				return
			}

			action := r.Method + " " + r.URL.Path
			entry := &store.AuditLog{
				TenantID: ac.TenantID,
				UserID:   ac.Identity,
				Action:   action,
				Detail:   r.URL.RawQuery,
			}
			if err := auditStore.Append(r.Context(), entry); err != nil {
				slog.Warn("audit log write failed", "error", err)
			}
		})
	}
}
