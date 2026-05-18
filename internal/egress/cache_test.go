package egress

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// countingPolicy records how many times the inner IsAllowed was called.
type countingPolicy struct {
	inner EgressPolicy
	calls atomic.Int64
}

func (p *countingPolicy) IsAllowed(ctx context.Context, tenantID, host, path string) (bool, error) {
	p.calls.Add(1)
	return p.inner.IsAllowed(ctx, tenantID, host, path)
}

func (p *countingPolicy) Reload(ctx context.Context) error {
	return p.inner.Reload(ctx)
}

func TestCachedPolicy_HitAvoidsDelegation(t *testing.T) {
	store := newMemoryStore()
	store.rules["t1"] = []EgressRule{
		{HostPattern: "api.example.com", PathPrefix: "/", Action: ActionAllow, Priority: 1},
	}
	inner := &countingPolicy{inner: NewAllowlistPolicy(store, nil, DefaultDenyAll)}
	cached := NewCachedPolicy(inner, 60*time.Second)

	ctx := context.Background()

	// First call — cache miss, must hit inner.
	allowed, err := cached.IsAllowed(ctx, "t1", "api.example.com", "/v1")
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("expected allowed")
	}
	if inner.calls.Load() != 1 {
		t.Fatalf("expected 1 inner call after cache miss, got %d", inner.calls.Load())
	}

	// Second call for same tenant+host — must be served from cache.
	_, err = cached.IsAllowed(ctx, "t1", "api.example.com", "/v1")
	if err != nil {
		t.Fatal(err)
	}
	if inner.calls.Load() != 1 {
		t.Fatalf("expected still 1 inner call after cache hit, got %d", inner.calls.Load())
	}
}

func TestCachedPolicy_TTLExpiry(t *testing.T) {
	store := newMemoryStore()
	store.rules["t1"] = []EgressRule{
		{HostPattern: "api.example.com", PathPrefix: "/", Action: ActionAllow, Priority: 1},
	}
	inner := &countingPolicy{inner: NewAllowlistPolicy(store, nil, DefaultDenyAll)}
	// Very short TTL so the entry expires immediately.
	cached := NewCachedPolicy(inner, 1*time.Millisecond)

	ctx := context.Background()

	cached.IsAllowed(ctx, "t1", "api.example.com", "/") //nolint:errcheck
	time.Sleep(5 * time.Millisecond)

	cached.IsAllowed(ctx, "t1", "api.example.com", "/") //nolint:errcheck
	if inner.calls.Load() != 2 {
		t.Fatalf("expected 2 inner calls after TTL expiry, got %d", inner.calls.Load())
	}
}

func TestCachedPolicy_ZeroTTLAlwaysDelegates(t *testing.T) {
	store := newMemoryStore()
	inner := &countingPolicy{inner: NewAllowlistPolicy(store, nil, DefaultAllowAll)}
	cached := NewCachedPolicy(inner, 0)

	ctx := context.Background()
	for range 3 {
		cached.IsAllowed(ctx, "t1", "any.com", "/") //nolint:errcheck
	}
	if inner.calls.Load() != 3 {
		t.Fatalf("zero-TTL: expected 3 inner calls, got %d", inner.calls.Load())
	}
}

func TestCachedPolicy_DifferentTenantsSeparateCacheEntries(t *testing.T) {
	store := newMemoryStore()
	store.rules["t1"] = []EgressRule{
		{HostPattern: "api.example.com", PathPrefix: "/", Action: ActionAllow, Priority: 1},
	}
	// t2 has no rules — deny-all applies.
	inner := &countingPolicy{inner: NewAllowlistPolicy(store, nil, DefaultDenyAll)}
	cached := NewCachedPolicy(inner, 60*time.Second)

	ctx := context.Background()

	ok1, _ := cached.IsAllowed(ctx, "t1", "api.example.com", "/")
	ok2, _ := cached.IsAllowed(ctx, "t2", "api.example.com", "/")

	if !ok1 {
		t.Fatal("t1 should be allowed")
	}
	if ok2 {
		t.Fatal("t2 should be denied (no rules)")
	}
	// Each tenant produced its own cache miss.
	if inner.calls.Load() != 2 {
		t.Fatalf("expected 2 inner calls for 2 tenants, got %d", inner.calls.Load())
	}
}

func TestCachedPolicy_InvalidateTenant(t *testing.T) {
	store := newMemoryStore()
	store.rules["t1"] = []EgressRule{
		{HostPattern: "api.example.com", PathPrefix: "/", Action: ActionAllow, Priority: 1},
	}
	inner := &countingPolicy{inner: NewAllowlistPolicy(store, nil, DefaultDenyAll)}
	cached := NewCachedPolicy(inner, 60*time.Second)

	ctx := context.Background()

	cached.IsAllowed(ctx, "t1", "api.example.com", "/") //nolint:errcheck
	if inner.calls.Load() != 1 {
		t.Fatal("expected 1 inner call on first lookup")
	}

	cached.InvalidateTenant("t1")

	// After invalidation the next call must go through to inner again.
	cached.IsAllowed(ctx, "t1", "api.example.com", "/") //nolint:errcheck
	if inner.calls.Load() != 2 {
		t.Fatalf("expected 2 inner calls after InvalidateTenant, got %d", inner.calls.Load())
	}
}

func TestCachedPolicy_ReloadFlushesCache(t *testing.T) {
	store := newMemoryStore()
	store.rules["t1"] = []EgressRule{
		{HostPattern: "old.com", PathPrefix: "/", Action: ActionAllow, Priority: 1},
	}
	inner := &countingPolicy{inner: NewAllowlistPolicy(store, nil, DefaultDenyAll)}
	cached := NewCachedPolicy(inner, 60*time.Second)

	ctx := context.Background()

	// Populate the cache.
	ok, _ := cached.IsAllowed(ctx, "t1", "old.com", "/")
	if !ok {
		t.Fatal("old.com should be allowed before reload")
	}

	// Change the underlying rules and reload.
	store.rules["t1"] = []EgressRule{
		{HostPattern: "new.com", PathPrefix: "/", Action: ActionAllow, Priority: 1},
	}
	if err := cached.Reload(ctx); err != nil {
		t.Fatal(err)
	}

	// old.com must now be denied (cache was flushed, inner re-evaluated).
	ok, _ = cached.IsAllowed(ctx, "t1", "old.com", "/")
	if ok {
		t.Fatal("old.com should be denied after reload")
	}

	ok, _ = cached.IsAllowed(ctx, "t1", "new.com", "/")
	if !ok {
		t.Fatal("new.com should be allowed after reload")
	}
}

func TestCachedPolicy_ImplementsEgressPolicyInterface(t *testing.T) {
	// Compile-time check that *CachedEgressPolicy satisfies EgressPolicy.
	var _ EgressPolicy = (*CachedEgressPolicy)(nil)
}
