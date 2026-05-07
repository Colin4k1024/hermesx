package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hermes-agent/hermes-agent-go/internal/store"
)

type mockPricingRuleStore struct {
	rules map[string]store.PricingRule
}

func newMockPricingRuleStore() *mockPricingRuleStore {
	return &mockPricingRuleStore{rules: make(map[string]store.PricingRule)}
}

func (m *mockPricingRuleStore) List(_ context.Context) ([]store.PricingRule, error) {
	var out []store.PricingRule
	for _, r := range m.rules {
		out = append(out, r)
	}
	return out, nil
}

func (m *mockPricingRuleStore) Get(_ context.Context, key string) (*store.PricingRule, error) {
	r, ok := m.rules[key]
	if !ok {
		return nil, nil
	}
	return &r, nil
}

func (m *mockPricingRuleStore) Upsert(_ context.Context, rule *store.PricingRule) error {
	m.rules[rule.ModelKey] = *rule
	return nil
}

func (m *mockPricingRuleStore) Delete(_ context.Context, key string) error {
	if _, ok := m.rules[key]; !ok {
		return store.ErrNotFound
	}
	delete(m.rules, key)
	return nil
}

type mockStoreForPricing struct {
	store.Store
	pricingStore store.PricingRuleStore
}

func (m *mockStoreForPricing) PricingRules() store.PricingRuleStore {
	return m.pricingStore
}

func TestAdminListPricingRules_Empty(t *testing.T) {
	ms := newMockPricingRuleStore()
	h := NewAdminHandler(&mockStoreForPricing{pricingStore: ms}, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/pricing-rules", nil)
	rec := httptest.NewRecorder()
	h.listPricingRules(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var rules []store.PricingRule
	json.NewDecoder(rec.Body).Decode(&rules)
	if len(rules) != 0 {
		t.Errorf("expected empty list, got %d", len(rules))
	}
}

func TestAdminUpsertPricingRule(t *testing.T) {
	ms := newMockPricingRuleStore()
	h := NewAdminHandler(&mockStoreForPricing{pricingStore: ms}, nil)

	body := `{"input_per_1k": 0.003, "output_per_1k": 0.015, "cache_read_per_1k": 0.001}`
	req := httptest.NewRequest(http.MethodPut, "/admin/v1/pricing-rules/gpt-4o", bytes.NewBufferString(body))
	req.SetPathValue("model", "gpt-4o")
	rec := httptest.NewRecorder()
	h.upsertPricingRule(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if ms.rules["gpt-4o"].InputPer1K != 0.003 {
		t.Error("expected rule to be stored")
	}
}

func TestAdminDeletePricingRule(t *testing.T) {
	ms := newMockPricingRuleStore()
	ms.rules["gpt-4o"] = store.PricingRule{ModelKey: "gpt-4o", InputPer1K: 0.003}

	h := NewAdminHandler(&mockStoreForPricing{pricingStore: ms}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/pricing-rules/gpt-4o", nil)
	req.SetPathValue("model", "gpt-4o")
	rec := httptest.NewRecorder()
	h.deletePricingRule(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if _, exists := ms.rules["gpt-4o"]; exists {
		t.Error("expected rule to be deleted")
	}
}

func TestAdminDeletePricingRule_NotFound(t *testing.T) {
	ms := newMockPricingRuleStore()
	h := NewAdminHandler(&mockStoreForPricing{pricingStore: ms}, nil)

	req := httptest.NewRequest(http.MethodDelete, "/admin/v1/pricing-rules/nonexistent", nil)
	req.SetPathValue("model", "nonexistent")
	rec := httptest.NewRecorder()
	h.deletePricingRule(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
