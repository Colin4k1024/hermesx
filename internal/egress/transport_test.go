package egress

import (
	"context"
	"errors"
	"net"
	"testing"
)

type mockPolicy struct {
	allowed bool
	err     error
}

func (m *mockPolicy) IsAllowed(_ context.Context, _, _, _ string) (bool, error) {
	return m.allowed, m.err
}

func (m *mockPolicy) Reload(_ context.Context) error { return nil }

type mockResolver struct {
	ips []net.IPAddr
	err error
}

func (m *mockResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	return m.ips, m.err
}

type mockAuditLogger struct {
	entries []auditEntry
}

type auditEntry struct {
	tenantID string
	host     string
	allowed  bool
	reason   string
}

func (m *mockAuditLogger) Log(_ context.Context, tenantID string, host string, allowed bool, reason string) {
	m.entries = append(m.entries, auditEntry{tenantID, host, allowed, reason})
}

func TestDialContext_BlocksPrivateIP(t *testing.T) {
	resolver := &mockResolver{
		ips: []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}},
	}
	policy := &mockPolicy{allowed: true}
	logger := &mockAuditLogger{}

	transport := &SecureTransport{
		policy:   policy,
		resolver: resolver,
		logger:   logger,
		base:     &net.Dialer{},
	}

	ctx := WithTenant(context.Background(), "tenant-1")
	_, err := transport.DialContext(ctx, "tcp", "evil.com:443")

	if !errors.Is(err, ErrBlockedIP) {
		t.Fatalf("expected ErrBlockedIP, got %v", err)
	}
	if len(logger.entries) != 1 || logger.entries[0].reason != "blocked_ip" {
		t.Fatalf("expected blocked_ip audit entry, got %+v", logger.entries)
	}
}

func TestDialContext_BlocksNotAllowed(t *testing.T) {
	resolver := &mockResolver{
		ips: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}},
	}
	policy := &mockPolicy{allowed: false}
	logger := &mockAuditLogger{}

	transport := &SecureTransport{
		policy:   policy,
		resolver: resolver,
		logger:   logger,
		base:     &net.Dialer{},
	}

	ctx := WithTenant(context.Background(), "tenant-1")
	ctx = WithPath(ctx, "/v1/chat")
	_, err := transport.DialContext(ctx, "tcp", "blocked.example.com:443")

	if !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("expected ErrNotAllowed, got %v", err)
	}
	if len(logger.entries) != 1 || logger.entries[0].reason != "not_allowed" {
		t.Fatalf("expected not_allowed audit entry, got %+v", logger.entries)
	}
}

func TestDialContext_DNSFailure(t *testing.T) {
	resolver := &mockResolver{err: errors.New("no such host")}
	policy := &mockPolicy{allowed: true}
	logger := &mockAuditLogger{}

	transport := &SecureTransport{
		policy:   policy,
		resolver: resolver,
		logger:   logger,
		base:     &net.Dialer{},
	}

	ctx := context.Background()
	_, err := transport.DialContext(ctx, "tcp", "nonexistent.test:80")

	if err == nil {
		t.Fatal("expected error for DNS failure")
	}
	if len(logger.entries) != 1 || logger.entries[0].reason != "dns_failure" {
		t.Fatalf("expected dns_failure audit entry, got %+v", logger.entries)
	}
}

func TestDialContext_PolicyError(t *testing.T) {
	resolver := &mockResolver{
		ips: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}},
	}
	policy := &mockPolicy{allowed: false, err: errors.New("db down")}
	logger := &mockAuditLogger{}

	transport := &SecureTransport{
		policy:   policy,
		resolver: resolver,
		logger:   logger,
		base:     &net.Dialer{},
	}

	ctx := WithTenant(context.Background(), "tenant-1")
	_, err := transport.DialContext(ctx, "tcp", "example.com:443")

	if err == nil {
		t.Fatal("expected error for policy failure")
	}
	if len(logger.entries) != 1 || logger.entries[0].reason != "policy_error" {
		t.Fatalf("expected policy_error audit entry, got %+v", logger.entries)
	}
}

func TestDialContext_BlocksLoopback(t *testing.T) {
	resolver := &mockResolver{
		ips: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}},
	}
	policy := &mockPolicy{allowed: true}

	transport := &SecureTransport{
		policy:   policy,
		resolver: resolver,
		logger:   &mockAuditLogger{},
		base:     &net.Dialer{},
	}

	ctx := context.Background()
	_, err := transport.DialContext(ctx, "tcp", "localhost:8080")

	if !errors.Is(err, ErrBlockedIP) {
		t.Fatalf("expected ErrBlockedIP for loopback, got %v", err)
	}
}

func TestDialContext_BlocksCGNAT(t *testing.T) {
	resolver := &mockResolver{
		ips: []net.IPAddr{{IP: net.ParseIP("100.64.0.1")}},
	}
	policy := &mockPolicy{allowed: true}

	transport := &SecureTransport{
		policy:   policy,
		resolver: resolver,
		logger:   &mockAuditLogger{},
		base:     &net.Dialer{},
	}

	ctx := context.Background()
	_, err := transport.DialContext(ctx, "tcp", "cgnat.test:443")

	if !errors.Is(err, ErrBlockedIP) {
		t.Fatalf("expected ErrBlockedIP for CGNAT, got %v", err)
	}
}

func TestDialContext_MultipleIPs_OneBlocked(t *testing.T) {
	resolver := &mockResolver{
		ips: []net.IPAddr{
			{IP: net.ParseIP("93.184.216.34")},
			{IP: net.ParseIP("192.168.1.1")},
		},
	}
	policy := &mockPolicy{allowed: true}

	transport := &SecureTransport{
		policy:   policy,
		resolver: resolver,
		logger:   &mockAuditLogger{},
		base:     &net.Dialer{},
	}

	ctx := context.Background()
	_, err := transport.DialContext(ctx, "tcp", "dual.test:443")

	if !errors.Is(err, ErrBlockedIP) {
		t.Fatalf("expected ErrBlockedIP when any resolved IP is private, got %v", err)
	}
}
