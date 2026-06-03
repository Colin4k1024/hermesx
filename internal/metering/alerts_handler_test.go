package metering

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/middleware"
)

func alertReq(method, path string, tenantID string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	ctx := req.Context()
	if tenantID != "" {
		ctx = middleware.WithTenant(ctx, tenantID)
	}
	return req.WithContext(ctx)
}

func TestAlertHandler_CreateAndList(t *testing.T) {
	rules := &memAlertRuleStore{}
	handler := NewAlertHandler(rules, nil)

	// Create a rule.
	body := map[string]any{"metric": "total_tokens", "threshold": 5000, "window": "daily"}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, alertReq(http.MethodPost, "/v1/usage-alerts", "t1", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body = %s", rec.Code, rec.Body.String())
	}

	var created AlertRule
	json.NewDecoder(rec.Body).Decode(&created)
	if created.TenantID != "t1" || created.Metric != MetricTotalTokens || created.Threshold != 5000 {
		t.Fatalf("unexpected rule: %+v", created)
	}

	// List rules.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, alertReq(http.MethodGet, "/v1/usage-alerts", "t1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", rec.Code)
	}
	var listed []*AlertRule
	json.NewDecoder(rec.Body).Decode(&listed)
	if len(listed) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(listed))
	}
}

func TestAlertHandler_UpdateRule(t *testing.T) {
	rules := &memAlertRuleStore{
		rules: []*AlertRule{
			{ID: "r1", TenantID: "t1", Metric: MetricCostUSD, Threshold: 10.0, Window: "daily", Enabled: true},
		},
	}
	handler := NewAlertHandler(rules, nil)

	body := map[string]any{"threshold": 20.0, "enabled": false}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, alertReq(http.MethodPut, "/v1/usage-alerts/r1", "t1", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var updated AlertRule
	json.NewDecoder(rec.Body).Decode(&updated)
	if updated.Threshold != 20.0 || updated.Enabled {
		t.Fatalf("unexpected updated rule: %+v", updated)
	}
}

func TestAlertHandler_DeleteRule(t *testing.T) {
	rules := &memAlertRuleStore{
		rules: []*AlertRule{
			{ID: "r1", TenantID: "t1", Metric: MetricCostUSD, Threshold: 10.0, Window: "daily", Enabled: true},
		},
	}
	handler := NewAlertHandler(rules, nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, alertReq(http.MethodDelete, "/v1/usage-alerts/r1", "t1", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}

	remaining, _ := rules.List(context.Background(), "t1")
	if len(remaining) != 0 {
		t.Fatalf("expected 0 rules after delete, got %d", len(remaining))
	}
}

func TestAlertHandler_NoTenant(t *testing.T) {
	handler := NewAlertHandler(&memAlertRuleStore{}, nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, alertReq(http.MethodGet, "/v1/usage-alerts", "", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAlertHandler_InvalidCreate(t *testing.T) {
	handler := NewAlertHandler(&memAlertRuleStore{}, nil)

	tests := []struct {
		name string
		body map[string]any
	}{
		{"missing metric", map[string]any{"threshold": 100}},
		{"zero threshold", map[string]any{"metric": "cost_usd", "threshold": 0}},
		{"invalid window", map[string]any{"metric": "cost_usd", "threshold": 100, "window": "yearly"}},
		{"invalid metric", map[string]any{"metric": "invalid", "threshold": 100}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, alertReq(http.MethodPost, "/v1/usage-alerts", "t1", tt.body))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestAlertHandler_ListEvents(t *testing.T) {
	events := &memAlertEventStore{
		events: []*AlertEvent{
			{ID: "e1", TenantID: "t1", Metric: MetricCostUSD, Threshold: 10, Current: 12},
		},
	}
	handler := NewAlertHandler(&memAlertRuleStore{}, events)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, alertReq(http.MethodGet, "/v1/usage-alerts/events", "t1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var listed []*AlertEvent
	json.NewDecoder(rec.Body).Decode(&listed)
	if len(listed) != 1 {
		t.Fatalf("expected 1 event, got %d", len(listed))
	}
}
