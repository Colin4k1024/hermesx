package egress

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

var (
	ErrBlockedIP  = errors.New("egress: blocked private/internal IP")
	ErrNotAllowed = errors.New("egress: host not in tenant allowlist")
)

type contextKey int

const (
	tenantKey contextKey = iota
	pathKey
)

func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey, tenantID)
}

func TenantFromContext(ctx context.Context) string {
	v, _ := ctx.Value(tenantKey).(string)
	return v
}

func WithPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, pathKey, path)
}

func PathFromContext(ctx context.Context) string {
	v, _ := ctx.Value(pathKey).(string)
	if v == "" {
		return "/"
	}
	return v
}

type AuditLogger interface {
	Log(ctx context.Context, tenantID string, host string, allowed bool, reason string)
}

type defaultAuditLogger struct{}

func (d *defaultAuditLogger) Log(ctx context.Context, tenantID string, host string, allowed bool, reason string) {
	slog.Info("egress decision",
		"tenant_id", tenantID,
		"host", host,
		"allowed", allowed,
		"reason", reason,
	)
}

type SecureTransport struct {
	policy   EgressPolicy
	logger   AuditLogger
	resolver Resolver
	base     *net.Dialer
}

type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type defaultResolver struct{}

func (d *defaultResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

type TransportOption func(*SecureTransport)

func WithAuditLogger(l AuditLogger) TransportOption {
	return func(t *SecureTransport) { t.logger = l }
}

func WithResolver(r Resolver) TransportOption {
	return func(t *SecureTransport) { t.resolver = r }
}

func WithDialTimeout(d time.Duration) TransportOption {
	return func(t *SecureTransport) { t.base.Timeout = d }
}

func NewSecureTransport(policy EgressPolicy, opts ...TransportOption) *http.Transport {
	st := &SecureTransport{
		policy:   policy,
		logger:   &defaultAuditLogger{},
		resolver: &defaultResolver{},
		base:     &net.Dialer{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		opt(st)
	}
	return &http.Transport{
		DialContext:         st.DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
}

func (t *SecureTransport) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("egress: invalid address %q: %w", addr, err)
	}

	ips, err := t.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		t.logger.Log(ctx, TenantFromContext(ctx), host, false, "dns_failure")
		return nil, fmt.Errorf("egress: DNS resolution failed for %s: %w", host, err)
	}
	if len(ips) == 0 {
		t.logger.Log(ctx, TenantFromContext(ctx), host, false, "no_records")
		return nil, fmt.Errorf("egress: no DNS records for %s", host)
	}

	for _, ip := range ips {
		if IsBlockedIP(ip.IP) {
			t.logger.Log(ctx, TenantFromContext(ctx), host, false, "blocked_ip")
			return nil, ErrBlockedIP
		}
	}

	tenantID := TenantFromContext(ctx)
	path := PathFromContext(ctx)

	allowed, err := t.policy.IsAllowed(ctx, tenantID, host, path)
	if err != nil {
		t.logger.Log(ctx, tenantID, host, false, "policy_error")
		return nil, fmt.Errorf("egress: policy check failed: %w", err)
	}
	if !allowed {
		t.logger.Log(ctx, tenantID, host, false, "not_allowed")
		return nil, ErrNotAllowed
	}

	target := net.JoinHostPort(ips[0].IP.String(), port)
	conn, err := t.base.DialContext(ctx, network, target)
	if err != nil {
		return nil, err
	}

	t.logger.Log(ctx, tenantID, host, true, "connected")
	return conn, nil
}
