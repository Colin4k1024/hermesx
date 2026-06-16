package egress

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/store"
)

type captureAuditStore struct {
	logs []*store.AuditLog
}

func (s *captureAuditStore) Append(_ context.Context, log *store.AuditLog) error {
	s.logs = append(s.logs, log)
	return nil
}

func TestStoreAuditLoggerPersistsDeniedEvents(t *testing.T) {
	audits := &captureAuditStore{}
	logger := NewStoreAuditLogger(audits, nil)

	ctx := WithPath(WithTenant(context.Background(), "tenant-1"), "/v1/models")
	logger.Log(ctx, "tenant-1", "api.example.com", false, "not_allowed")
	logger.Log(ctx, "tenant-1", "api.example.com", true, "connected")
	logger.Log(context.Background(), "", "api.example.com", false, "dns_failure")

	if len(audits.logs) != 1 {
		t.Fatalf("persisted logs = %d, want 1", len(audits.logs))
	}
	got := audits.logs[0]
	if got.TenantID != "tenant-1" {
		t.Fatalf("tenant_id = %q, want tenant-1", got.TenantID)
	}
	if got.Action != EgressDeniedAuditAction {
		t.Fatalf("action = %q, want %q", got.Action, EgressDeniedAuditAction)
	}
	if got.ErrorCode != "not_allowed" {
		t.Fatalf("error_code = %q, want not_allowed", got.ErrorCode)
	}

	var detail map[string]any
	if err := json.Unmarshal([]byte(got.Detail), &detail); err != nil {
		t.Fatalf("detail is not JSON: %v", err)
	}
	if detail["host"] != "api.example.com" || detail["path"] != "/v1/models" || detail["reason"] != "not_allowed" {
		t.Fatalf("unexpected detail: %+v", detail)
	}
}
