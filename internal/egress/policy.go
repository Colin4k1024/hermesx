package egress

import "context"

type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
)

type DefaultPolicy string

const (
	DefaultDenyAll  DefaultPolicy = "deny-all"
	DefaultAllowAll DefaultPolicy = "allow-all"
	DefaultLogOnly  DefaultPolicy = "log-only"
)

type EgressRule struct {
	ID          string
	TenantID    string
	HostPattern string
	PathPrefix  string
	Action      Action
	Priority    int
}

type EgressPolicy interface {
	IsAllowed(ctx context.Context, tenantID string, host string, path string) (bool, error)
	Reload(ctx context.Context) error
}

// allowAllPolicy is a pass-through EgressPolicy used during transition.
// It permits all outbound connections and is suitable for single-tenant
// deployments or before per-tenant allowlists are provisioned.
type allowAllPolicy struct{}

func (a *allowAllPolicy) IsAllowed(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}

func (a *allowAllPolicy) Reload(_ context.Context) error { return nil }

// NewAllowAllPolicy returns an EgressPolicy that permits all hosts.
// Use this as a safe transition default until tenant allowlists are configured.
func NewAllowAllPolicy() EgressPolicy { return &allowAllPolicy{} }
