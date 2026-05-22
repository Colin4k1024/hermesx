package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/store"
)

type handlerAuthStore struct {
	store.Store
	pricing store.PricingRuleStore
}

func (s *handlerAuthStore) PricingRules() store.PricingRuleStore { return s.pricing }

func TestAdminHandler_DomainScopeAllowsRoute(t *testing.T) {
	h := NewAdminHandler(&handlerAuthStore{pricing: newMockPricingRuleStore()}, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/pricing-rules", nil)
	req = req.WithContext(auth.WithContext(req.Context(), &auth.AuthContext{
		Identity: "billing-user",
		TenantID: "platform",
		Scopes:   []string{"billing:read"},
	}))
	rec := httptest.NewRecorder()

	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_LegacyEmptyScopeRejected(t *testing.T) {
	h := NewAdminHandler(&handlerAuthStore{pricing: newMockPricingRuleStore()}, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/pricing-rules", nil)
	req = req.WithContext(auth.WithContext(req.Context(), &auth.AuthContext{
		Identity: "legacy-user",
		TenantID: "platform",
		Scopes:   nil,
	}))
	rec := httptest.NewRecorder()

	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
