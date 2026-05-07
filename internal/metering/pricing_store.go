package metering

import (
	"context"
	"sync"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

// PricingStore provides cached access to per-model pricing rules.
// It loads all rules from the database with a 30s TTL and uses
// singleflight semantics to avoid thundering herd on cache miss.
type PricingStore struct {
	db store.PricingRuleStore

	mu       sync.RWMutex
	cache    map[string]store.PricingRule
	loadedAt time.Time
	ttl      time.Duration

	loading sync.Mutex // poor-man's singleflight
}

func NewPricingStore(db store.PricingRuleStore) *PricingStore {
	return &PricingStore{
		db:  db,
		ttl: 30 * time.Second,
	}
}

// GetCost returns the pricing for a model, loading from DB if cache is stale.
// Returns nil if the model has no pricing rule in the database.
func (ps *PricingStore) GetCost(ctx context.Context, modelKey string) *store.PricingRule {
	ps.mu.RLock()
	if time.Since(ps.loadedAt) < ps.ttl && ps.cache != nil {
		rule, ok := ps.cache[modelKey]
		ps.mu.RUnlock()
		if ok {
			return &rule
		}
		return nil
	}
	ps.mu.RUnlock()

	ps.refresh(ctx)

	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if rule, ok := ps.cache[modelKey]; ok {
		return &rule
	}
	return nil
}

func (ps *PricingStore) refresh(ctx context.Context) {
	ps.loading.Lock()
	defer ps.loading.Unlock()

	// Double-check after acquiring lock (singleflight behavior).
	ps.mu.RLock()
	if time.Since(ps.loadedAt) < ps.ttl && ps.cache != nil {
		ps.mu.RUnlock()
		return
	}
	ps.mu.RUnlock()

	rules, err := ps.db.List(ctx)
	if err != nil {
		return // keep stale cache on error
	}

	m := make(map[string]store.PricingRule, len(rules))
	for _, r := range rules {
		m[r.ModelKey] = r
	}

	ps.mu.Lock()
	ps.cache = m
	ps.loadedAt = time.Now()
	ps.mu.Unlock()
}

// Invalidate forces the next GetCost call to reload from DB.
func (ps *PricingStore) Invalidate() {
	ps.mu.Lock()
	ps.loadedAt = time.Time{}
	ps.mu.Unlock()
}
