package egress

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/Colin4k1024/hermesx/internal/store"
)

const EgressDeniedAuditAction = "egress.denied"

type auditLogStore interface {
	Append(ctx context.Context, log *store.AuditLog) error
}

type StoreAuditLogger struct {
	store auditLogStore
	next  AuditLogger
}

func NewStoreAuditLogger(store auditLogStore, next AuditLogger) *StoreAuditLogger {
	if next == nil {
		next = &defaultAuditLogger{}
	}
	return &StoreAuditLogger{store: store, next: next}
}

func (l *StoreAuditLogger) Log(ctx context.Context, tenantID string, host string, allowed bool, reason string) {
	if l.next != nil {
		l.next.Log(ctx, tenantID, host, allowed, reason)
	}
	if l.store == nil || allowed || tenantID == "" {
		return
	}

	detail, err := json.Marshal(map[string]any{
		"host":    host,
		"path":    PathFromContext(ctx),
		"reason":  reason,
		"allowed": false,
	})
	if err != nil {
		slog.Warn("egress audit detail marshal failed", "tenant_id", tenantID, "host", host, "reason", reason, "error", err)
		return
	}

	if err := l.store.Append(ctx, &store.AuditLog{
		TenantID:  tenantID,
		Action:    EgressDeniedAuditAction,
		Detail:    string(detail),
		ErrorCode: reason,
	}); err != nil {
		slog.Warn("egress denied audit persist failed", "tenant_id", tenantID, "host", host, "reason", reason, "error", err)
	}
}
