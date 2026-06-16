package egress

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

type adminAuditStore struct {
	logs []*store.AuditLog
}

func (s *adminAuditStore) Append(_ context.Context, log *store.AuditLog) error {
	s.logs = append(s.logs, log)
	return nil
}

func (s *adminAuditStore) DeleteByTenant(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func (s *adminAuditStore) ArchiveOlderThan(_ context.Context, _ time.Time, _ int) ([]*store.AuditLog, error) {
	return nil, nil
}

func (s *adminAuditStore) ArchiveCount(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func (s *adminAuditStore) List(_ context.Context, tenantID string, opts store.AuditListOptions) ([]*store.AuditLog, int, error) {
	var filtered []*store.AuditLog
	for _, log := range s.logs {
		if tenantID != "" && log.TenantID != tenantID {
			continue
		}
		if opts.Action != "" && log.Action != opts.Action {
			continue
		}
		filtered = append(filtered, log)
	}
	total := len(filtered)
	if opts.Offset >= len(filtered) {
		filtered = nil
	} else if opts.Offset > 0 {
		filtered = filtered[opts.Offset:]
	}
	if opts.Limit > 0 && len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
	}
	return filtered, total, nil
}

func TestBlockedLogReturnsPersistedDeniedEgressEvents(t *testing.T) {
	auditStore := &adminAuditStore{logs: []*store.AuditLog{
		{ID: 1, TenantID: "tenant-1", Action: EgressDeniedAuditAction, Detail: `{"host":"blocked.example.com"}`},
		{ID: 2, TenantID: "tenant-1", Action: "POST /v1/chat/completions"},
		{ID: 3, TenantID: "tenant-2", Action: EgressDeniedAuditAction},
	}}
	handler := NewAdminHandler(nil, nil, WithAuditStore(auditStore))

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/egress/blocked-log?tenant_id=tenant-1&limit=1&offset=-10", nil)
	rec := httptest.NewRecorder()

	handler.blockedLog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		BlockedEvents []store.AuditLog `json:"blocked_events"`
		Total         int              `json:"total"`
		Limit         int              `json:"limit"`
		Offset        int              `json:"offset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("total = %d, want 1", resp.Total)
	}
	if resp.Limit != 1 || resp.Offset != 0 {
		t.Fatalf("pagination = limit %d offset %d, want limit 1 offset 0", resp.Limit, resp.Offset)
	}
	if len(resp.BlockedEvents) != 1 || resp.BlockedEvents[0].ID != 1 {
		t.Fatalf("blocked_events = %+v, want only id=1", resp.BlockedEvents)
	}
}
