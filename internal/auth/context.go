package auth

import (
	"context"
	"slices"
)

// AuthContext carries authenticated identity through the request lifecycle.
type AuthContext struct {
	Identity   string   // user ID or API key ID
	TenantID   string   // derived from credential, not from header
	Roles      []string // e.g. ["user"], ["admin"], ["operator"]
	AuthMethod string   // "static_token", "jwt", "api_key"
}

type contextKey struct{}

// FromContext extracts the AuthContext from the request context.
func FromContext(ctx context.Context) (*AuthContext, bool) {
	ac, ok := ctx.Value(contextKey{}).(*AuthContext)
	return ac, ok
}

// WithContext stores an AuthContext into the request context.
func WithContext(ctx context.Context, ac *AuthContext) context.Context {
	return context.WithValue(ctx, contextKey{}, ac)
}

// HasRole reports whether the AuthContext contains the given role.
func (ac *AuthContext) HasRole(role string) bool {
	return slices.Contains(ac.Roles, role)
}
