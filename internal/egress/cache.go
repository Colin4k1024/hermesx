package egress

import (
	"context"
	"sync"
	"time"
)

// cachedEntry holds the result of an IsAllowed lookup together with its
// expiry timestamp.  A zero expiry means the entry has never been populated.
type cachedEntry struct {
	allowed bool
	expiry  time.Time
}

// CachedEgressPolicy wraps any EgressPolicy with a TTL-based in-memory cache.
// On a cache hit the inner policy is not consulted, saving a DB round-trip for
// every outbound request inside the hot path.
//
// The cache is keyed by "<tenantID>:<host>" because path-level caching would
// produce unbounded key growth for hosts with many distinct request paths;
// AllowlistPolicy already handles path matching inside EgressRule evaluation.
//
// Cache invalidation happens passively through TTL expiry.  Call
// InvalidateTenant to eagerly evict all entries for a given tenant (e.g. after
// a rule update via the admin API).
type CachedEgressPolicy struct {
	inner EgressPolicy
	ttl   time.Duration

	mu    sync.RWMutex
	cache map[string]cachedEntry
}

// NewCachedPolicy wraps inner with a TTL-based cache.  A ttl of zero disables
// caching (every lookup falls through to inner immediately).
func NewCachedPolicy(inner EgressPolicy, ttl time.Duration) *CachedEgressPolicy {
	return &CachedEgressPolicy{
		inner: inner,
		ttl:   ttl,
		cache: make(map[string]cachedEntry),
	}
}

// IsAllowed implements EgressPolicy.  It returns the cached decision when
// available and not expired, otherwise it delegates to the inner policy and
// stores the result.
func (c *CachedEgressPolicy) IsAllowed(ctx context.Context, tenantID string, host string, path string) (bool, error) {
	key := tenantID + ":" + host

	if c.ttl > 0 {
		c.mu.RLock()
		entry, ok := c.cache[key]
		c.mu.RUnlock()
		if ok && time.Now().Before(entry.expiry) {
			return entry.allowed, nil
		}
	}

	allowed, err := c.inner.IsAllowed(ctx, tenantID, host, path)
	if err != nil {
		return false, err
	}

	if c.ttl > 0 {
		c.mu.Lock()
		c.cache[key] = cachedEntry{
			allowed: allowed,
			expiry:  time.Now().Add(c.ttl),
		}
		c.mu.Unlock()
	}

	return allowed, nil
}

// Reload implements EgressPolicy.  It flushes the entire cache and delegates
// to the inner policy's Reload so that stale entries are not served after a
// rule change.
func (c *CachedEgressPolicy) Reload(ctx context.Context) error {
	c.mu.Lock()
	c.cache = make(map[string]cachedEntry)
	c.mu.Unlock()
	return c.inner.Reload(ctx)
}

// InvalidateTenant removes all cache entries whose key starts with
// "<tenantID>:".  Call this after modifying rules for a specific tenant via
// the admin API to avoid serving stale decisions until TTL expiry.
func (c *CachedEgressPolicy) InvalidateTenant(tenantID string) {
	prefix := tenantID + ":"
	c.mu.Lock()
	for k := range c.cache {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.cache, k)
		}
	}
	c.mu.Unlock()
}
