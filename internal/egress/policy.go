package egress

import (
	"context"
	"fmt"
	"os"
	"strings"
)

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

const (
	// EnvDefaultPolicy explicitly overrides the environment-derived egress
	// default. Valid values are allow-all, deny-all, and log-only.
	EnvDefaultPolicy = "HERMES_EGRESS_DEFAULT"
)

var defaultEnvironmentKeys = []string{
	"HERMES_ENV",
	"HERMESX_ENV",
	"APP_ENV",
	"GO_ENV",
	"ENVIRONMENT",
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
// Use this only for local development or an explicit allow-all override.
func NewAllowAllPolicy() EgressPolicy { return &allowAllPolicy{} }

type denyAllPolicy struct{}

func (d *denyAllPolicy) IsAllowed(_ context.Context, _, _, _ string) (bool, error) {
	return false, nil
}

func (d *denyAllPolicy) Reload(_ context.Context) error { return nil }

// NewDenyAllPolicy returns an EgressPolicy that rejects every host unless a
// higher-level allowlist policy is configured with explicit rules.
func NewDenyAllPolicy() EgressPolicy { return &denyAllPolicy{} }

type logOnlyPolicy struct{}

func (l *logOnlyPolicy) IsAllowed(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}

func (l *logOnlyPolicy) Reload(_ context.Context) error { return nil }

// NewLogOnlyPolicy returns an EgressPolicy that allows requests while still
// passing through SecureTransport audit logging.
func NewLogOnlyPolicy() EgressPolicy { return &logOnlyPolicy{} }

// ResolveDefaultPolicy applies HermesX's safe-by-environment default:
// development allows unknown egress for convenience; production denies unless
// tenant allowlist rules are configured. override, when present, must be one of
// allow-all, deny-all, or log-only.
func ResolveDefaultPolicy(environment string, override string) (DefaultPolicy, error) {
	if strings.TrimSpace(override) != "" {
		return parseDefaultPolicy(override)
	}

	switch normalizePolicyToken(environment) {
	case "production", "prod":
		return DefaultDenyAll, nil
	default:
		return DefaultAllowAll, nil
	}
}

// ResolveDefaultPolicyFromEnv resolves HERMES_EGRESS_DEFAULT, falling back to
// environment-derived defaults from HERMES_ENV/HERMESX_ENV/APP_ENV/GO_ENV.
func ResolveDefaultPolicyFromEnv() (DefaultPolicy, error) {
	return ResolveDefaultPolicy(environmentFromEnv(), os.Getenv(EnvDefaultPolicy))
}

// NewAllowlistPolicyFromEnv builds the runtime policy used by production
// transports. A nil store produces an empty allowlist, which means production
// mode fails closed while development remains convenient.
func NewAllowlistPolicyFromEnv(store RuleStore, cache RuleCache) (*AllowlistPolicy, DefaultPolicy, error) {
	defaultPolicy, err := ResolveDefaultPolicyFromEnv()
	if err != nil {
		return nil, "", err
	}
	if store == nil {
		store = EmptyRuleStore{}
	}
	return NewAllowlistPolicy(store, cache, defaultPolicy), defaultPolicy, nil
}

func parseDefaultPolicy(raw string) (DefaultPolicy, error) {
	switch normalizePolicyToken(raw) {
	case string(DefaultAllowAll):
		return DefaultAllowAll, nil
	case string(DefaultDenyAll):
		return DefaultDenyAll, nil
	case string(DefaultLogOnly):
		return DefaultLogOnly, nil
	default:
		return "", fmt.Errorf("invalid %s value %q: expected allow-all, deny-all, or log-only", EnvDefaultPolicy, raw)
	}
}

func environmentFromEnv() string {
	for _, key := range defaultEnvironmentKeys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return "development"
}

func normalizePolicyToken(raw string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(raw), "_", "-"))
}
