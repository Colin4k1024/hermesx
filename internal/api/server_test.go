package api

import (
	"context"
	"fmt"
	"testing"

	"github.com/hermes-agent/hermes-agent-go/internal/store"
)

// stubSessionStore implements store.SessionStore with no-op methods.
type stubSessionStore struct{}

func (stubSessionStore) Create(_ context.Context, _ string, _ *store.Session) error         { return nil }
func (stubSessionStore) Get(_ context.Context, _, _ string) (*store.Session, error)          { return nil, nil }
func (stubSessionStore) End(_ context.Context, _, _, _ string) error                         { return nil }
func (stubSessionStore) List(_ context.Context, _ string, _ store.ListOptions) ([]*store.Session, int, error) {
	return nil, 0, nil
}
func (stubSessionStore) Delete(_ context.Context, _, _ string) error                         { return nil }
func (stubSessionStore) UpdateTokens(_ context.Context, _, _ string, _ store.TokenDelta) error { return nil }
func (stubSessionStore) SetTitle(_ context.Context, _, _, _ string) error                    { return nil }

// stubMessageStore implements store.MessageStore with no-op methods.
type stubMessageStore struct{}

func (stubMessageStore) Append(_ context.Context, _, _ string, _ *store.Message) (int64, error) {
	return 0, nil
}
func (stubMessageStore) List(_ context.Context, _, _ string, _, _ int) ([]*store.Message, error) {
	return nil, nil
}
func (stubMessageStore) Search(_ context.Context, _, _ string, _ int) ([]*store.SearchResult, error) {
	return nil, nil
}
func (stubMessageStore) CountBySession(_ context.Context, _, _ string) (int, error) { return 0, nil }

// stubUserStore implements store.UserStore with no-op methods.
type stubUserStore struct{}

func (stubUserStore) GetOrCreate(_ context.Context, _, _, _ string) (*store.User, error) { return nil, nil }
func (stubUserStore) IsApproved(_ context.Context, _, _, _ string) (bool, error)         { return false, nil }
func (stubUserStore) Approve(_ context.Context, _, _, _ string) error                    { return nil }
func (stubUserStore) Revoke(_ context.Context, _, _, _ string) error                     { return nil }
func (stubUserStore) ListApproved(_ context.Context, _, _ string) ([]string, error)      { return nil, nil }

// stubTenantStore implements store.TenantStore with no-op methods.
type stubTenantStore struct{}

func (stubTenantStore) Create(_ context.Context, _ *store.Tenant) error                       { return nil }
func (stubTenantStore) Get(_ context.Context, _ string) (*store.Tenant, error)                { return nil, nil }
func (stubTenantStore) Update(_ context.Context, _ *store.Tenant) error                       { return nil }
func (stubTenantStore) Delete(_ context.Context, _ string) error                              { return nil }
func (stubTenantStore) List(_ context.Context, _ store.ListOptions) ([]*store.Tenant, int, error) {
	return nil, 0, nil
}

// stubAuditLogStore implements store.AuditLogStore with no-op methods.
type stubAuditLogStore struct{}

func (stubAuditLogStore) Append(_ context.Context, _ *store.AuditLog) error { return nil }
func (stubAuditLogStore) List(_ context.Context, _ string, _ store.AuditListOptions) ([]*store.AuditLog, int, error) {
	return nil, 0, nil
}

// stubAPIKeyStore implements store.APIKeyStore with no-op methods.
type stubAPIKeyStore struct{}

func (stubAPIKeyStore) Create(_ context.Context, _ *store.APIKey) error                { return nil }
func (stubAPIKeyStore) GetByHash(_ context.Context, _ string) (*store.APIKey, error)   { return nil, nil }
func (stubAPIKeyStore) GetByID(_ context.Context, _ string) (*store.APIKey, error)     { return nil, nil }
func (stubAPIKeyStore) List(_ context.Context, _ string) ([]*store.APIKey, error)      { return nil, nil }
func (stubAPIKeyStore) Revoke(_ context.Context, _ string) error                       { return nil }

// stubStore implements store.Store, returning stub sub-stores.
type stubStore struct{}

func (stubStore) Sessions() store.SessionStore   { return stubSessionStore{} }
func (stubStore) Messages() store.MessageStore   { return stubMessageStore{} }
func (stubStore) Users() store.UserStore         { return stubUserStore{} }
func (stubStore) Tenants() store.TenantStore     { return stubTenantStore{} }
func (stubStore) AuditLogs() store.AuditLogStore { return stubAuditLogStore{} }
func (stubStore) APIKeys() store.APIKeyStore     { return stubAPIKeyStore{} }
func (stubStore) Close() error                   { return nil }
func (stubStore) Migrate(_ context.Context) error { return nil }

func TestNewAPIServer(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		wantAddr string
	}{
		{
			name:     "returns non-nil server",
			port:     8080,
			wantAddr: ":8080",
		},
		{
			name:     "server addr matches configured port",
			port:     9090,
			wantAddr: ":9090",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewAPIServer(APIServerConfig{
				Port:  tt.port,
				Store: stubStore{},
				DB:    nil,
			})

			if srv == nil {
				t.Fatal("NewAPIServer returned nil")
			}

			gotAddr := fmt.Sprintf(":%d", srv.cfg.Port)
			if gotAddr != tt.wantAddr {
				t.Errorf("server addr = %q, want %q", gotAddr, tt.wantAddr)
			}
		})
	}
}
