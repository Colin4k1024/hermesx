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
