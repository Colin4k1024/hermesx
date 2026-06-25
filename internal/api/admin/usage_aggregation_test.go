package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/metering"
)

// mockUsageStore is a test double for metering.UsageStore.
type mockUsageStore struct {
	summaries []metering.UsageSummary
	err       error
}

func (m *mockUsageStore) BatchInsert(_ context.Context, _ []metering.UsageRecord) error {
	return nil
}

func (m *mockUsageStore) QueryByTenant(_ context.Context, _ string, _, _ time.Time, _ string) ([]metering.UsageSummary, error) {
	return m.summaries, m.err
}

func (m *mockUsageStore) QueryBySession(_ context.Context, _, _ string) ([]metering.UsageRecord, error) {
	return nil, nil
}

func TestAdminUsageAggregation_DailyGranularity(t *testing.T) {
	store := &mockUsageStore{
		summaries: []metering.UsageSummary{
			{Date: "2026-07-01", InputTokens: 1000, OutputTokens: 500, CostUSD: 0.05},
			{Date: "2026-07-02", InputTokens: 2000, OutputTokens: 1000, CostUSD: 0.10},
		},
	}

	h := &AdminHandler{usageStore: store}

	req := httptest.NewRequest("GET", "/admin/v1/usage?tenant_id=test-tenant&granularity=daily&from=2026-07-01&to=2026-07-03", nil)
	w := httptest.NewRecorder()

	h.adminUsageAggregation(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := result["data"].([]any)
	if !ok {
		t.Fatalf("expected data array, got %T", result["data"])
	}

	if len(data) != 2 {
		t.Errorf("expected 2 items, got %d", len(data))
	}
}

func TestAdminUsageAggregation_MonthlyGranularity(t *testing.T) {
	store := &mockUsageStore{
		summaries: []metering.UsageSummary{
			{Date: "2026-07-01", InputTokens: 5000, OutputTokens: 2500, CostUSD: 0.25},
		},
	}

	h := &AdminHandler{usageStore: store}

	req := httptest.NewRequest("GET", "/admin/v1/usage?tenant_id=test-tenant&granularity=monthly&from=2026-07-01&to=2026-08-01", nil)
	w := httptest.NewRecorder()

	h.adminUsageAggregation(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := result["data"].([]any)
	if !ok {
		t.Fatalf("expected data array, got %T", result["data"])
	}

	if len(data) != 1 {
		t.Errorf("expected 1 item, got %d", len(data))
	}

	item := data[0].(map[string]any)
	if item["total_tokens"].(float64) != 7500 {
		t.Errorf("expected total_tokens 7500, got %v", item["total_tokens"])
	}
}

func TestAdminUsageAggregation_MissingTenantID(t *testing.T) {
	store := &mockUsageStore{}
	h := &AdminHandler{usageStore: store}

	req := httptest.NewRequest("GET", "/admin/v1/usage?granularity=daily", nil)
	w := httptest.NewRecorder()

	h.adminUsageAggregation(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAdminUsageAggregation_InvalidGranularity(t *testing.T) {
	store := &mockUsageStore{}
	h := &AdminHandler{usageStore: store}

	req := httptest.NewRequest("GET", "/admin/v1/usage?tenant_id=test-tenant&granularity=invalid", nil)
	w := httptest.NewRecorder()

	h.adminUsageAggregation(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestAdminUsageAggregation_NilStore(t *testing.T) {
	h := &AdminHandler{usageStore: nil}

	req := httptest.NewRequest("GET", "/admin/v1/usage?tenant_id=test-tenant&granularity=daily", nil)
	w := httptest.NewRecorder()

	h.adminUsageAggregation(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}
}

func TestAdminUsageAggregation_EmptyResult(t *testing.T) {
	store := &mockUsageStore{
		summaries: []metering.UsageSummary{},
	}

	h := &AdminHandler{usageStore: store}

	req := httptest.NewRequest("GET", "/admin/v1/usage?tenant_id=test-tenant&granularity=daily&from=2026-07-01&to=2026-07-02", nil)
	w := httptest.NewRecorder()

	h.adminUsageAggregation(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	data, ok := result["data"].([]any)
	if !ok {
		t.Fatalf("expected data array, got %T", result["data"])
	}

	if len(data) != 0 {
		t.Errorf("expected 0 items, got %d", len(data))
	}
}

func TestParseAdminTimeParam(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty", "", true},
		{"rfc3339", "2026-07-01T00:00:00Z", false},
		{"date", "2026-07-01", false},
		{"invalid", "invalid-date", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseAdminTimeParam(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAdminTimeParam(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
