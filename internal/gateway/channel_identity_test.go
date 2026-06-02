package gateway

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/channel"
	"github.com/Colin4k1024/hermesx/internal/store"
)

type fakeChannelAppStore struct {
	app *store.ChannelApp
}

func (s *fakeChannelAppStore) Create(_ context.Context, app *store.ChannelApp) error {
	s.app = app
	return nil
}

func (s *fakeChannelAppStore) GetByID(_ context.Context, tenantID, id string) (*store.ChannelApp, error) {
	if s.app != nil && s.app.TenantID == tenantID && s.app.ID == id {
		return s.app, nil
	}
	return nil, store.ErrNotFound
}

func (s *fakeChannelAppStore) GetByPlatformAppKey(_ context.Context, platform, appKey string) (*store.ChannelApp, error) {
	if s.app != nil && s.app.Platform == platform && s.app.AppKey == appKey {
		return s.app, nil
	}
	return nil, store.ErrNotFound
}

func (s *fakeChannelAppStore) List(_ context.Context, _ string, _ store.ListOptions) ([]*store.ChannelApp, int, error) {
	return nil, 0, nil
}

func (s *fakeChannelAppStore) Update(_ context.Context, app *store.ChannelApp) error {
	s.app = app
	return nil
}

func (s *fakeChannelAppStore) Delete(_ context.Context, _, _ string) error { return nil }

type fakeChannelIdentityStore struct {
	identity *store.ChannelIdentity
}

func (s *fakeChannelIdentityStore) Upsert(_ context.Context, identity *store.ChannelIdentity) error {
	s.identity = identity
	return nil
}

func (s *fakeChannelIdentityStore) GetByID(_ context.Context, tenantID, id string) (*store.ChannelIdentity, error) {
	if s.identity != nil && s.identity.TenantID == tenantID && s.identity.ID == id {
		return s.identity, nil
	}
	return nil, store.ErrNotFound
}

func (s *fakeChannelIdentityStore) GetByProviderHash(_ context.Context, channelAppID, providerUserHash string) (*store.ChannelIdentity, error) {
	if s.identity != nil && s.identity.ChannelAppID == channelAppID && s.identity.ProviderUserHash == providerUserHash {
		return s.identity, nil
	}
	return nil, store.ErrNotFound
}

func (s *fakeChannelIdentityStore) ListByUser(_ context.Context, _, _ string) ([]*store.ChannelIdentity, error) {
	return nil, nil
}

func (s *fakeChannelIdentityStore) List(_ context.Context, _ string, _ store.ListOptions) ([]*store.ChannelIdentity, int, error) {
	return nil, 0, nil
}

func (s *fakeChannelIdentityStore) Revoke(_ context.Context, _, _ string) error { return nil }

type fakeGatewayAuditStore struct {
	logs []*store.AuditLog
}

func (s *fakeGatewayAuditStore) Append(_ context.Context, log *store.AuditLog) error {
	s.logs = append(s.logs, log)
	return nil
}

func (s *fakeGatewayAuditStore) List(_ context.Context, _ string, _ store.AuditListOptions) ([]*store.AuditLog, int, error) {
	return nil, 0, nil
}

func (s *fakeGatewayAuditStore) DeleteByTenant(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func TestGatewayIdentityResolver_BoundRewritesSource(t *testing.T) {
	app := &store.ChannelApp{ID: "app-1", TenantID: "tenant-1", Platform: string(PlatformWeixin), AppKey: "wx-app", Enabled: true}
	providerHash, err := channel.HashProviderUser("secret", app.Platform, app.AppKey, "openid-1")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	resolver := NewGatewayIdentityResolver(GatewayIdentityResolverConfig{
		Apps:       &fakeChannelAppStore{app: app},
		Identities: &fakeChannelIdentityStore{identity: &store.ChannelIdentity{ChannelAppID: app.ID, ProviderUserHash: providerHash, UserID: "user-1"}},
		Challenges: channel.NewChallengeStore(0),
		HashSecret: "secret",
		PublicURL:  "https://saas.example.com",
	})
	event := &MessageEvent{
		Source:   SessionSource{Platform: PlatformWeixin, UserID: "openid-1", ChatID: "openid-1"},
		Metadata: map[string]string{"app_key": "wx-app"},
	}

	result := resolver.Resolve(context.Background(), event)
	if result.Status != GatewayIdentityBound {
		t.Fatalf("status = %s, want bound: %s", result.Status, result.Message)
	}
	if event.Source.TenantID != "tenant-1" || event.Source.UserID != "user-1" {
		t.Fatalf("source not rewritten: %+v", event.Source)
	}
	if event.Metadata["provider_user_hash"] != providerHash {
		t.Fatalf("provider hash metadata missing")
	}
}

func TestGatewayIdentityResolver_UnboundCreatesLoginLink(t *testing.T) {
	app := &store.ChannelApp{ID: "app-1", TenantID: "tenant-1", Platform: string(PlatformWeixin), AppKey: "wx-app", Enabled: true}
	audit := &fakeGatewayAuditStore{}
	resolver := NewGatewayIdentityResolver(GatewayIdentityResolverConfig{
		Apps:       &fakeChannelAppStore{app: app},
		Identities: &fakeChannelIdentityStore{},
		AuditLogs:  audit,
		Challenges: channel.NewChallengeStore(time.Minute),
		HashSecret: "secret",
		PublicURL:  "https://saas.example.com",
		ReturnTo:   "/dashboard",
	})
	event := &MessageEvent{
		Source:   SessionSource{Platform: PlatformWeixin, UserID: "openid-1", ChatID: "openid-1"},
		Metadata: map[string]string{"app_key": "wx-app"},
	}

	result := resolver.Resolve(context.Background(), event)
	if result.Status != GatewayIdentityUnbound {
		t.Fatalf("status = %s, want unbound: %s", result.Status, result.Message)
	}
	if !strings.Contains(result.Message, "https://saas.example.com/auth/channel/weixin/start?") {
		t.Fatalf("login url missing from message: %s", result.Message)
	}
	if !strings.Contains(result.Message, "app_key=wx-app") || !strings.Contains(result.Message, "return_to=%2Fdashboard") {
		t.Fatalf("login url missing query params: %s", result.Message)
	}
	if len(audit.logs) != 1 || audit.logs[0].Action != "GATEWAY_UNBOUND_MESSAGE" {
		t.Fatalf("audit logs = %+v, want GATEWAY_UNBOUND_MESSAGE", audit.logs)
	}
}
