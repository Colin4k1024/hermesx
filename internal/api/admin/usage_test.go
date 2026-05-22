package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/metering"
	"github.com/Colin4k1024/hermesx/internal/store"
)

type mockTenantUsageStore struct {
	query metering.TenantUsageQuery
	rows  []metering.TenantUsageSummary
}

func (m *mockTenantUsageStore) BatchInsert(_ context.Context, _ []metering.UsageRecord) error {
	return nil
}

func (m *mockTenantUsageStore) QueryByTenant(_ context.Context, _ string, _, _ time.Time, _ string) ([]metering.UsageSummary, error) {
	return nil, nil
}

func (m *mockTenantUsageStore) QueryBySession(_ context.Context, _, _ string) ([]metering.UsageRecord, error) {
	return nil, nil
}

func (m *mockTenantUsageStore) QueryTenants(_ context.Context, q metering.TenantUsageQuery) ([]metering.TenantUsageSummary, error) {
	m.query = q
	return m.rows, nil
}

type mockStoreForUsage struct {
	store.Store
}

func TestAdminListTenantUsage_UsesMeteringAggregator(t *testing.T) {
	usage := &mockTenantUsageStore{
		rows: []metering.TenantUsageSummary{
			{
				TenantID:     "tenant-1",
				SessionCount: 2,
				InputTokens:  100,
				OutputTokens: 50,
				CostUSD:      0.42,
			},
		},
	}
	h := NewAdminHandler(&mockStoreForUsage{}, nil, WithUsageStore(usage))

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/usage/tenants?limit=10&offset=3", nil)
	rec := httptest.NewRecorder()
	h.listTenantUsage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if usage.query.Limit != 10 || usage.query.Offset != 3 {
		t.Fatalf("query = %+v, want limit=10 offset=3", usage.query)
	}

	var resp struct {
		Tenants []TenantUsageSummary `json:"tenants"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Tenants) != 1 {
		t.Fatalf("tenant rows = %d, want 1", len(resp.Tenants))
	}
	if resp.Tenants[0].TotalTokens != 150 {
		t.Fatalf("total_tokens = %d, want 150", resp.Tenants[0].TotalTokens)
	}
}

func TestAdminListTenantUsage_RequiresAggregator(t *testing.T) {
	h := NewAdminHandler(&mockStoreForUsage{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/usage/tenants", nil)
	rec := httptest.NewRecorder()
	h.listTenantUsage(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
