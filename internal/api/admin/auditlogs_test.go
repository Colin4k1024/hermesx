package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

// mockAuditStore implements store.AuditLogStore for testing.
type mockAuditStore struct {
	logs []*store.AuditLog
}

func (m *mockAuditStore) Append(_ context.Context, log *store.AuditLog) error {
	m.logs = append(m.logs, log)
	return nil
}

func (m *mockAuditStore) DeleteByTenant(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func (m *mockAuditStore) List(_ context.Context, tenantID string, opts store.AuditListOptions) ([]*store.AuditLog, int, error) {
	var filtered []*store.AuditLog
	for _, l := range m.logs {
		if tenantID != "" && l.TenantID != tenantID {
			continue
		}
		if opts.Action != "" && l.Action != opts.Action {
			continue
		}
		if opts.From != nil && l.CreatedAt.Before(*opts.From) {
			continue
		}
		if opts.To != nil && !l.CreatedAt.Before(*opts.To) {
			continue
		}
		filtered = append(filtered, l)
	}

	total := len(filtered)

	if opts.Offset > 0 && opts.Offset < len(filtered) {
		filtered = filtered[opts.Offset:]
	} else if opts.Offset >= len(filtered) {
		filtered = nil
	}
	if opts.Limit > 0 && opts.Limit < len(filtered) {
		filtered = filtered[:opts.Limit]
	}

	return filtered, total, nil
}

func TestAdminListAuditLogs(t *testing.T) {
	now := time.Now().UTC()
	earlier := now.Add(-24 * time.Hour)
	later := now.Add(24 * time.Hour)

	tests := []struct {
		name       string
		query      string
		seedData   []*store.AuditLog
		wantStatus int
		wantTotal  int
	}{
		{
			name:  "no filters returns all",
			query: "/admin/v1/audit-logs",
			seedData: []*store.AuditLog{
				{ID: 1, TenantID: "t1", Action: "POST /v1/sessions", CreatedAt: now},
				{ID: 2, TenantID: "t2", Action: "DELETE /v1/sessions/x", CreatedAt: now},
			},
			wantStatus: http.StatusOK,
			wantTotal:  2,
		},
		{
			name:  "filter by tenant_id",
			query: "/admin/v1/audit-logs?tenant_id=t1",
			seedData: []*store.AuditLog{
				{ID: 1, TenantID: "t1", Action: "POST /v1/sessions", CreatedAt: now},
				{ID: 2, TenantID: "t2", Action: "DELETE /v1/sessions/x", CreatedAt: now},
			},
			wantStatus: http.StatusOK,
			wantTotal:  1,
		},
		{
			name:  "filter by action",
			query: "/admin/v1/audit-logs?action=POST+/v1/sessions",
			seedData: []*store.AuditLog{
				{ID: 1, TenantID: "t1", Action: "POST /v1/sessions", CreatedAt: now},
				{ID: 2, TenantID: "t1", Action: "DELETE /v1/sessions/x", CreatedAt: now},
			},
			wantStatus: http.StatusOK,
			wantTotal:  1,
		},
		{
			name:  "filter by time range",
			query: "/admin/v1/audit-logs?from=" + now.Add(-1*time.Hour).Format(time.RFC3339) + "&to=" + now.Add(1*time.Hour).Format(time.RFC3339),
			seedData: []*store.AuditLog{
				{ID: 1, TenantID: "t1", Action: "a", CreatedAt: earlier},
				{ID: 2, TenantID: "t1", Action: "b", CreatedAt: now},
				{ID: 3, TenantID: "t1", Action: "c", CreatedAt: later},
			},
			wantStatus: http.StatusOK,
			wantTotal:  1,
		},
		{
			name:       "invalid from param returns 400",
			query:      "/admin/v1/audit-logs?from=not-a-date",
			seedData:   nil,
			wantStatus: http.StatusBadRequest,
			wantTotal:  -1,
		},
		{
			name:       "invalid to param returns 400",
			query:      "/admin/v1/audit-logs?to=bad",
			seedData:   nil,
			wantStatus: http.StatusBadRequest,
			wantTotal:  -1,
		},
		{
			name:  "pagination limit",
			query: "/admin/v1/audit-logs?limit=1&offset=1",
			seedData: []*store.AuditLog{
				{ID: 1, TenantID: "t1", Action: "a", CreatedAt: now},
				{ID: 2, TenantID: "t1", Action: "b", CreatedAt: now},
				{ID: 3, TenantID: "t1", Action: "c", CreatedAt: now},
			},
			wantStatus: http.StatusOK,
			wantTotal:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockAuditStore{logs: tt.seedData}
			mockStore := &mockStoreForAudit{auditStore: ms}
			h := NewAdminHandler(mockStore, nil)

			req := httptest.NewRequest(http.MethodGet, tt.query, nil)
			rec := httptest.NewRecorder()

			h.listAuditLogs(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantTotal >= 0 {
				var resp map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				total := int(resp["total"].(float64))
				if total != tt.wantTotal {
					t.Errorf("total = %d, want %d", total, tt.wantTotal)
				}
			}
		})
	}
}

// mockStoreForAudit wraps just the AuditLogs method for admin handler tests.
type mockStoreForAudit struct {
	store.Store
	auditStore store.AuditLogStore
}

func (m *mockStoreForAudit) AuditLogs() store.AuditLogStore {
	return m.auditStore
}
