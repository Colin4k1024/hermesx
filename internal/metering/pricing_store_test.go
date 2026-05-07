package metering

import (
	"context"
	"testing"
	"time"

	"github.com/hermes-agent/hermes-agent-go/internal/store"
)

type mockPricingRuleStore struct {
	rules []store.PricingRule
	calls int
}

func (m *mockPricingRuleStore) List(_ context.Context) ([]store.PricingRule, error) {
	m.calls++
	return m.rules, nil
}
func (m *mockPricingRuleStore) Get(_ context.Context, key string) (*store.PricingRule, error) {
	for _, r := range m.rules {
		if r.ModelKey == key {
			return &r, nil
		}
	}
	return nil, nil
}
func (m *mockPricingRuleStore) Upsert(_ context.Context, _ *store.PricingRule) error { return nil }
func (m *mockPricingRuleStore) Delete(_ context.Context, _ string) error             { return nil }

func TestPricingStore_GetCost_Hit(t *testing.T) {
	mock := &mockPricingRuleStore{
		rules: []store.PricingRule{
			{ModelKey: "gpt-4o", InputPer1K: 0.005, OutputPer1K: 0.02, CacheReadPer1K: 0.001},
		},
	}
	ps := NewPricingStore(mock)

	rule := ps.GetCost(context.Background(), "gpt-4o")
	if rule == nil {
		t.Fatal("expected pricing rule for gpt-4o")
	}
	if rule.InputPer1K != 0.005 {
		t.Errorf("expected InputPer1K=0.005, got %f", rule.InputPer1K)
	}
	if mock.calls != 1 {
		t.Errorf("expected 1 DB call, got %d", mock.calls)
	}
}

func TestPricingStore_GetCost_Miss(t *testing.T) {
	mock := &mockPricingRuleStore{
		rules: []store.PricingRule{
			{ModelKey: "gpt-4o", InputPer1K: 0.005, OutputPer1K: 0.02},
		},
	}
	ps := NewPricingStore(mock)

	rule := ps.GetCost(context.Background(), "unknown-model")
	if rule != nil {
		t.Error("expected nil for unknown model")
	}
}

func TestPricingStore_CacheTTL(t *testing.T) {
	mock := &mockPricingRuleStore{
		rules: []store.PricingRule{
			{ModelKey: "gpt-4o", InputPer1K: 0.005, OutputPer1K: 0.02},
		},
	}
	ps := NewPricingStore(mock)
	ps.ttl = 50 * time.Millisecond

	ps.GetCost(context.Background(), "gpt-4o")
	ps.GetCost(context.Background(), "gpt-4o")
	if mock.calls != 1 {
		t.Errorf("expected 1 DB call within TTL, got %d", mock.calls)
	}

	time.Sleep(60 * time.Millisecond)
	ps.GetCost(context.Background(), "gpt-4o")
	if mock.calls != 2 {
		t.Errorf("expected 2 DB calls after TTL expiry, got %d", mock.calls)
	}
}

func TestPricingStore_Invalidate(t *testing.T) {
	mock := &mockPricingRuleStore{
		rules: []store.PricingRule{
			{ModelKey: "gpt-4o", InputPer1K: 0.005, OutputPer1K: 0.02},
		},
	}
	ps := NewPricingStore(mock)

	ps.GetCost(context.Background(), "gpt-4o")
	ps.Invalidate()
	ps.GetCost(context.Background(), "gpt-4o")
	if mock.calls != 2 {
		t.Errorf("expected 2 DB calls after invalidation, got %d", mock.calls)
	}
}

func TestCostCalculator_DBFirst(t *testing.T) {
	mock := &mockPricingRuleStore{
		rules: []store.PricingRule{
			{ModelKey: "gpt-4o", InputPer1K: 0.01, OutputPer1K: 0.04},
		},
	}
	ps := NewPricingStore(mock)
	cc := NewCostCalculator(ps)

	cost := cc.Calculate(context.Background(), "gpt-4o", 1000, 500)
	expected := (1000.0/1000.0)*0.01 + (500.0/1000.0)*0.04
	if cost != expected {
		t.Errorf("expected %f, got %f", expected, cost)
	}
}

func TestCostCalculator_FallbackToHardcoded(t *testing.T) {
	mock := &mockPricingRuleStore{rules: nil}
	ps := NewPricingStore(mock)
	cc := NewCostCalculator(ps)

	cost := cc.Calculate(context.Background(), "gpt-4o", 1000, 500)
	hardcoded := CalculateCost("gpt-4o", 1000, 500)
	if cost != hardcoded {
		t.Errorf("expected hardcoded fallback %f, got %f", hardcoded, cost)
	}
}
