package channel

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/secrets"
	"github.com/Colin4k1024/hermesx/internal/store"
)

func TestSplitWeComAppKey_WithColon(t *testing.T) {
	corpID, agentID := splitWeComAppKey("corp123:agent456")
	if corpID != "corp123" {
		t.Fatalf("corpID = %q, want %q", corpID, "corp123")
	}
	if agentID != "agent456" {
		t.Fatalf("agentID = %q, want %q", agentID, "agent456")
	}
}

func TestSplitWeComAppKey_WithoutColon(t *testing.T) {
	corpID, agentID := splitWeComAppKey("corp123")
	if corpID != "corp123" {
		t.Fatalf("corpID = %q, want %q", corpID, "corp123")
	}
	if agentID != "" {
		t.Fatalf("agentID = %q, want empty string", agentID)
	}
}

func TestSplitWeComAppKey_EmptyString(t *testing.T) {
	corpID, agentID := splitWeComAppKey("")
	if corpID != "" {
		t.Fatalf("corpID = %q, want empty string", corpID)
	}
	if agentID != "" {
		t.Fatalf("agentID = %q, want empty string", agentID)
	}
}

func TestSplitWeComAppKey_MultipleColons(t *testing.T) {
	// SplitN with n=2 means only split on first colon
	corpID, agentID := splitWeComAppKey("corp:agent:extra")
	if corpID != "corp" {
		t.Fatalf("corpID = %q, want %q", corpID, "corp")
	}
	if agentID != "agent:extra" {
		t.Fatalf("agentID = %q, want %q", agentID, "agent:extra")
	}
}

func TestNewProviderRegistry(t *testing.T) {
	reg := NewProviderRegistry(nil)
	if reg == nil {
		t.Fatal("NewProviderRegistry returned nil")
	}

	// Should have feishu, weixin, wecom providers by default
	if _, ok := reg.Get(PlatformFeishu); !ok {
		t.Fatal("expected feishu provider to be registered")
	}
	if _, ok := reg.Get(PlatformWeixin); !ok {
		t.Fatal("expected weixin provider to be registered")
	}
	if _, ok := reg.Get(PlatformWeCom); !ok {
		t.Fatal("expected wecom provider to be registered")
	}
}

func TestProviderRegistry_Get_NotFound(t *testing.T) {
	reg := NewProviderRegistry(nil)
	_, ok := reg.Get("unknown-platform")
	if ok {
		t.Fatal("Get should return false for unknown platform")
	}
}

func TestProviderRegistry_Register(t *testing.T) {
	reg := NewProviderRegistry(nil)

	mock := &mockProvider{platform: "custom"}
	reg.Register(mock)

	p, ok := reg.Get("custom")
	if !ok {
		t.Fatal("Get should return true after registering custom provider")
	}
	if p.Platform() != "custom" {
		t.Fatalf("provider.Platform() = %q, want %q", p.Platform(), "custom")
	}
}

func TestProviderRegistry_Register_OverridesExisting(t *testing.T) {
	reg := NewProviderRegistry(nil)

	// Override feishu with a mock
	mock := &mockProvider{platform: PlatformFeishu}
	reg.Register(mock)

	p, ok := reg.Get(PlatformFeishu)
	if !ok {
		t.Fatal("Get should return true for overridden provider")
	}
	// The mock is now the provider
	if _, isMock := p.(*mockProvider); !isMock {
		t.Fatal("expected the overridden provider to be the mock")
	}
}

func TestDefaultHTTPClient(t *testing.T) {
	client := defaultHTTPClient()
	if client == nil {
		t.Fatal("defaultHTTPClient returned nil")
	}
	if client.Timeout != 15*time.Second {
		t.Fatalf("client.Timeout = %v, want %v", client.Timeout, 15*time.Second)
	}
}

func TestProviderRegistry_Register_OnNilMap(t *testing.T) {
	// Test that Register handles nil providers map gracefully
	reg := &ProviderRegistry{}
	mock := &mockProvider{platform: "test"}
	reg.Register(mock) // should not panic

	p, ok := reg.Get("test")
	if !ok {
		t.Fatal("Get should return true after registering on nil-map registry")
	}
	if p.Platform() != "test" {
		t.Fatalf("provider.Platform() = %q, want %q", p.Platform(), "test")
	}
}

// mockProvider is a minimal Provider implementation for testing.
type mockProvider struct {
	platform string
}

func (m *mockProvider) Platform() string { return m.platform }

func (m *mockProvider) AuthCodeURL(_ *store.ChannelApp, _, _ string) (string, error) {
	return "", nil
}

func (m *mockProvider) ExchangeCode(_ context.Context, _ *store.ChannelApp, _ string) (*Principal, error) {
	return nil, nil
}

func (m *mockProvider) VerifyWebhook(_ context.Context, _ *store.ChannelApp, _ *http.Request, _ []byte) error {
	return nil
}

// mockSecretResolver is a minimal secrets.SecretResolver for testing.
type mockSecretResolver struct {
	resolved map[string]string
	err      error
}

func (r *mockSecretResolver) Resolve(_ context.Context, name string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	if v, ok := r.resolved[name]; ok {
		return v, nil
	}
	return "", errors.New("secret not found: " + name)
}

func (r *mockSecretResolver) RegisterPattern(_ string, _ *regexp.Regexp) {}
func (r *mockSecretResolver) ListRegistered() []string                   { return nil }
func (r *mockSecretResolver) ResolvedValues() map[string]string          { return nil }

var _ secrets.SecretResolver = (*mockSecretResolver)(nil)

func TestResolveAppSecret_NilResolver(t *testing.T) {
	ctx := context.Background()
	app := &store.ChannelApp{AppKey: "app", OAuthSecretRef: "ref"}
	_, err := resolveAppSecret(ctx, nil, app)
	if err == nil {
		t.Fatal("expected error for nil resolver")
	}
}

func TestResolveAppSecret_NoSecretRef(t *testing.T) {
	ctx := context.Background()
	resolver := &mockSecretResolver{resolved: map[string]string{}}
	app := &store.ChannelApp{AppKey: "app"}
	_, err := resolveAppSecret(ctx, resolver, app)
	if err == nil {
		t.Fatal("expected error when no secret ref configured")
	}
}

func TestResolveAppSecret_OAuthSecretRef(t *testing.T) {
	ctx := context.Background()
	resolver := &mockSecretResolver{resolved: map[string]string{"my-secret": "secret-value"}}
	app := &store.ChannelApp{AppKey: "app", OAuthSecretRef: "my-secret"}
	val, err := resolveAppSecret(ctx, resolver, app)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "secret-value" {
		t.Errorf("got %q, want %q", val, "secret-value")
	}
}

func TestResolveAppSecret_FallsBackToAppSecretRef(t *testing.T) {
	ctx := context.Background()
	resolver := &mockSecretResolver{resolved: map[string]string{"app-ref": "app-secret"}}
	app := &store.ChannelApp{AppKey: "app", AppSecretRef: "app-ref"}
	val, err := resolveAppSecret(ctx, resolver, app)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "app-secret" {
		t.Errorf("got %q, want %q", val, "app-secret")
	}
}

func TestProviderPlatformMethods(t *testing.T) {
	resolver := &mockSecretResolver{resolved: map[string]string{}}
	reg := NewProviderRegistry(resolver)

	feishu, ok := reg.Get(PlatformFeishu)
	if !ok || feishu.Platform() != PlatformFeishu {
		t.Errorf("feishu Platform() = %q, want %q", feishu.Platform(), PlatformFeishu)
	}

	weixin, ok := reg.Get(PlatformWeixin)
	if !ok || weixin.Platform() != PlatformWeixin {
		t.Errorf("weixin Platform() = %q, want %q", weixin.Platform(), PlatformWeixin)
	}

	wecom, ok := reg.Get(PlatformWeCom)
	if !ok || wecom.Platform() != PlatformWeCom {
		t.Errorf("wecom Platform() = %q, want %q", wecom.Platform(), PlatformWeCom)
	}
}
