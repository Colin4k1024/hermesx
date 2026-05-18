package egress

import (
	"context"
	"testing"
)

type memoryStore struct {
	rules map[string][]EgressRule
}

func newMemoryStore() *memoryStore {
	return &memoryStore{rules: make(map[string][]EgressRule)}
}

func (m *memoryStore) LoadRules(_ context.Context, tenantID string) ([]EgressRule, error) {
	return m.rules[tenantID], nil
}

func TestMatchHost_Exact(t *testing.T) {
	tests := []struct {
		pattern string
		host    string
		want    bool
	}{
		{"api.openai.com", "api.openai.com", true},
		{"api.openai.com", "API.OPENAI.COM", true},
		{"api.openai.com", "other.com", false},
		{"api.openai.com", "sub.api.openai.com", false},
	}
	for _, tt := range tests {
		if got := matchHost(tt.pattern, tt.host); got != tt.want {
			t.Errorf("matchHost(%q, %q) = %v, want %v", tt.pattern, tt.host, got, tt.want)
		}
	}
}

func TestMatchHost_Wildcard(t *testing.T) {
	tests := []struct {
		pattern string
		host    string
		want    bool
	}{
		{"*.example.com", "sub.example.com", true},
		{"*.example.com", "deep.sub.example.com", true},
		{"*.example.com", "example.com", false},
		{"*.example.com", "notexample.com", false},
		{"*.internal.corp", "api.internal.corp", true},
		{"*.internal.corp", "internal.corp", false},
	}
	for _, tt := range tests {
		if got := matchHost(tt.pattern, tt.host); got != tt.want {
			t.Errorf("matchHost(%q, %q) = %v, want %v", tt.pattern, tt.host, got, tt.want)
		}
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		prefix string
		path   string
		want   bool
	}{
		{"/", "/anything", true},
		{"", "/anything", true},
		{"/v1/", "/v1/completions", true},
		{"/v1/", "/v2/completions", false},
		{"/api", "/api/users", true},
		{"/api", "/other", false},
	}
	for _, tt := range tests {
		if got := matchPath(tt.prefix, tt.path); got != tt.want {
			t.Errorf("matchPath(%q, %q) = %v, want %v", tt.prefix, tt.path, got, tt.want)
		}
	}
}

func TestEvaluateRules_PriorityBased(t *testing.T) {
	rules := []EgressRule{
		{HostPattern: "api.example.com", PathPrefix: "/", Action: ActionDeny, Priority: 1},
		{HostPattern: "api.example.com", PathPrefix: "/v1/", Action: ActionAllow, Priority: 10},
	}

	action, matched := evaluateRules(rules, "api.example.com", "/v1/chat")
	if !matched {
		t.Fatal("expected a match")
	}
	if action != ActionAllow {
		t.Fatalf("expected ActionAllow (higher priority), got %s", action)
	}
}

func TestEvaluateRules_NoMatch(t *testing.T) {
	rules := []EgressRule{
		{HostPattern: "api.example.com", PathPrefix: "/", Action: ActionAllow, Priority: 1},
	}

	_, matched := evaluateRules(rules, "other.com", "/")
	if matched {
		t.Fatal("expected no match for unrelated host")
	}
}

func TestAllowlistPolicy_BuiltinAlwaysAllowed(t *testing.T) {
	store := newMemoryStore()
	policy := NewAllowlistPolicy(store, nil, DefaultDenyAll)

	ctx := context.Background()
	allowed, err := policy.IsAllowed(ctx, "tenant-1", "api.openai.com", "/v1/chat")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("builtin LLM endpoints should always be allowed")
	}

	allowed, err = policy.IsAllowed(ctx, "tenant-1", "api.anthropic.com", "/v1/messages")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("builtin LLM endpoints should always be allowed")
	}
}

func TestAllowlistPolicy_DenyAll_BlocksUnknown(t *testing.T) {
	store := newMemoryStore()
	policy := NewAllowlistPolicy(store, nil, DefaultDenyAll)

	ctx := context.Background()
	allowed, err := policy.IsAllowed(ctx, "tenant-1", "unknown.com", "/")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("deny-all policy should block unknown hosts")
	}
}

func TestAllowlistPolicy_AllowAll_AllowsUnknown(t *testing.T) {
	store := newMemoryStore()
	policy := NewAllowlistPolicy(store, nil, DefaultAllowAll)

	ctx := context.Background()
	allowed, err := policy.IsAllowed(ctx, "tenant-1", "anything.com", "/")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("allow-all policy should allow unknown hosts")
	}
}

func TestAllowlistPolicy_TenantRulesEnforced(t *testing.T) {
	store := newMemoryStore()
	store.rules["tenant-1"] = []EgressRule{
		{HostPattern: "allowed.com", PathPrefix: "/", Action: ActionAllow, Priority: 1},
		{HostPattern: "blocked.com", PathPrefix: "/", Action: ActionDeny, Priority: 1},
	}

	policy := NewAllowlistPolicy(store, nil, DefaultDenyAll)
	ctx := context.Background()

	allowed, err := policy.IsAllowed(ctx, "tenant-1", "allowed.com", "/api")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("explicitly allowed host should pass")
	}

	allowed, err = policy.IsAllowed(ctx, "tenant-1", "blocked.com", "/secret")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("explicitly denied host should be blocked")
	}
}

func TestAllowlistPolicy_WildcardRules(t *testing.T) {
	store := newMemoryStore()
	store.rules["tenant-1"] = []EgressRule{
		{HostPattern: "*.internal.corp", PathPrefix: "/", Action: ActionAllow, Priority: 1},
	}

	policy := NewAllowlistPolicy(store, nil, DefaultDenyAll)
	ctx := context.Background()

	allowed, err := policy.IsAllowed(ctx, "tenant-1", "api.internal.corp", "/")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("wildcard match should allow subdomain")
	}

	allowed, err = policy.IsAllowed(ctx, "tenant-1", "internal.corp", "/")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("wildcard *.x should not match x itself")
	}
}

func TestAllowlistPolicy_Reload(t *testing.T) {
	store := newMemoryStore()
	store.rules["tenant-1"] = []EgressRule{
		{HostPattern: "old.com", PathPrefix: "/", Action: ActionAllow, Priority: 1},
	}

	policy := NewAllowlistPolicy(store, nil, DefaultDenyAll)
	ctx := context.Background()

	allowed, _ := policy.IsAllowed(ctx, "tenant-1", "old.com", "/")
	if !allowed {
		t.Fatal("should allow old.com before reload")
	}

	store.rules["tenant-1"] = []EgressRule{
		{HostPattern: "new.com", PathPrefix: "/", Action: ActionAllow, Priority: 1},
	}
	policy.Reload(ctx)

	allowed, _ = policy.IsAllowed(ctx, "tenant-1", "old.com", "/")
	if allowed {
		t.Fatal("should deny old.com after reload with new rules")
	}

	allowed, _ = policy.IsAllowed(ctx, "tenant-1", "new.com", "/")
	if !allowed {
		t.Fatal("should allow new.com after reload")
	}
}

func TestIsBuiltinAllowed(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"api.openai.com", true},
		{"API.OPENAI.COM", true},
		{"api.anthropic.com", true},
		{"other.com", false},
		{"openai.com", false},
	}
	for _, tt := range tests {
		if got := isBuiltinAllowed(tt.host); got != tt.want {
			t.Errorf("isBuiltinAllowed(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}
