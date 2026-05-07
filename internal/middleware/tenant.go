package middleware

import (
	"context"
	"net/http"
	"regexp"

	"github.com/Colin4k1024/hermesx/internal/auth"
)

type tenantCtxKey string

const tenantKey tenantCtxKey = "tenant_id"

var tenantIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// TenantFromContext extracts the tenant ID from context.
func TenantFromContext(ctx context.Context) string {
	v, _ := ctx.Value(tenantKey).(string)
	return v
}

// WithTenant injects a tenant ID into the context.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey, tenantID)
}

// TenantMiddleware derives tenant ID from AuthContext (set by auth middleware).
// Falls back to "default" only when AuthContext is absent (anonymous/dev mode).
// Never trusts the X-Tenant-ID header for tenant identity.
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := "default"

		if ac, ok := auth.FromContext(r.Context()); ok && ac != nil {
			if ac.TenantID != "" {
				tenantID = ac.TenantID
			}
		}

		if !tenantIDPattern.MatchString(tenantID) {
			http.Error(w, "invalid tenant ID", http.StatusBadRequest)
			return
		}
		ctx := WithTenant(r.Context(), tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
