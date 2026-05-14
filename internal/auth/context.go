package auth

import (
	"context"
	"slices"
)

// AuthContext carries authenticated identity through the request lifecycle.
type AuthContext struct {
	Identity   string   // user ID or API key ID
	UserID     string   // real user identifier (OIDC sub / JWT sub); empty for API keys
	TenantID   string   // derived from credential, not from header
	Roles      []string // e.g. ["user"], ["admin"], ["operator"]
	Scopes     []string // fine-grained scopes; empty = legacy (role-only check)
	AuthMethod string   // "static_token", "jwt", "api_key"
	ACRLevel   string   // OIDC acr claim value; empty if not provided
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

// HasScope reports whether the AuthContext contains the given scope.
//
// Compatibility policy (accepted, not a bug):
//   - Keys created before scopes were introduced have an empty Scopes slice.
//   - These legacy keys are granted read/write access to non-admin endpoints to
//     avoid breaking existing integrations.
//   - The "admin" scope always requires an explicit grant; legacy keys are never
//     silently elevated.
//   - New keys created via the API carry explicit scopes and are evaluated strictly.
//
// To migrate: re-issue API keys with explicit scopes and retire legacy ones.
func (ac *AuthContext) HasScope(scope string) bool {
	if len(ac.Scopes) == 0 {
		return scope != "admin"
	}
	return slices.Contains(ac.Scopes, scope)
}
