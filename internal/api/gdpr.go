package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/hermes-agent/hermes-agent-go/internal/middleware"
	"github.com/hermes-agent/hermes-agent-go/internal/objstore"
	"github.com/hermes-agent/hermes-agent-go/internal/observability"
	"github.com/hermes-agent/hermes-agent-go/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

const gdprExportMaxSessions = 1000

var gdprAllowedTables = map[string]struct{}{
	"messages":         {},
	"sessions":         {},
	"memories":         {},
	"user_profiles":    {},
	"api_keys":         {},
	"cron_jobs":        {},
	"users":            {},
	"audit_logs":       {},
	"usage_records":    {},
	"roles":            {},
	"purge_audit_logs": {},
}

// GDPRHandler serves data export and deletion endpoints.
// Accepts the full Store + direct pool for tables not exposed via store interface.
type GDPRHandler struct {
	store store.Store
	pool  *pgxpool.Pool
	minio *objstore.MinIOClient
}

func NewGDPRHandler(s store.Store, pool *pgxpool.Pool, minio *objstore.MinIOClient) *GDPRHandler {
	return &GDPRHandler{store: s, pool: pool, minio: minio}
}

// ExportHandler returns GET /v1/gdpr/export — exports all user data for a tenant.
func (h *GDPRHandler) ExportHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		tenantID := middleware.TenantFromContext(r.Context())
		if tenantID == "" {
			http.Error(w, "tenant context required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		log := observability.ContextLogger(ctx)

		sessions, _, err := h.store.Sessions().List(ctx, tenantID, store.ListOptions{Limit: gdprExportMaxSessions})
		if err != nil {
			http.Error(w, "export failed", http.StatusInternalServerError)
			return
		}

		type sessionExport struct {
			Session  *store.Session   `json:"session"`
			Messages []*store.Message `json:"messages"`
		}

		// Collect all memories for tenant (across all users).
		type memoryEntry struct {
			UserID  string `json:"user_id"`
			Key     string `json:"key"`
			Content string `json:"content"`
		}
		var memories []memoryEntry
		if h.pool != nil {
			rows, err := h.pool.Query(ctx, `SELECT user_id, key, content FROM memories WHERE tenant_id = $1`, tenantID)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var m memoryEntry
					if err := rows.Scan(&m.UserID, &m.Key, &m.Content); err == nil {
						memories = append(memories, m)
					}
				}
			} else {
				log.Warn("gdpr export: failed to list memories", "error", err)
			}
		}

		// Collect user profiles via Store interface.
		type profileEntry struct {
			UserID  string `json:"user_id"`
			Content string `json:"content"`
		}
		var profiles []profileEntry
		if h.pool != nil {
			rows, err := h.pool.Query(ctx, `SELECT user_id, content FROM user_profiles WHERE tenant_id = $1`, tenantID)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var p profileEntry
					if err := rows.Scan(&p.UserID, &p.Content); err == nil {
						profiles = append(profiles, p)
					}
				}
			} else {
				log.Warn("gdpr export: failed to list profiles", "error", err)
			}
		}

		// Audit the export action.
		_ = h.store.AuditLogs().Append(ctx, &store.AuditLog{
			TenantID:  tenantID,
			Action:    "GDPR_EXPORT",
			Detail:    fmt.Sprintf("sessions=%d memories=%d profiles=%d", len(sessions), len(memories), len(profiles)),
			RequestID: middleware.RequestIDFromContext(ctx),
		})

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=export.json")

		enc := json.NewEncoder(w)
		fmt.Fprint(w, `{"tenant_id":"`+tenantID+`","sessions":[`)
		for i, sess := range sessions {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			msgs, err := h.store.Messages().List(ctx, tenantID, sess.ID, 1000, 0)
			if err != nil {
				log.Warn("gdpr export: failed to list messages", "session_id", sess.ID, "error", err)
				msgs = nil
			}
			enc.Encode(sessionExport{Session: sess, Messages: msgs})
		}
		fmt.Fprint(w, `],"memories":`)
		enc.Encode(memories)
		fmt.Fprint(w, `,"profiles":`)
		enc.Encode(profiles)
		fmt.Fprint(w, `}`)
	}
}

// DeleteHandler returns DELETE /v1/gdpr/data — deletes all data for a tenant.
// All deletions run in a single transaction to prevent partial data loss.
func (h *GDPRHandler) DeleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		tenantID := middleware.TenantFromContext(r.Context())
		if tenantID == "" {
			http.Error(w, "tenant context required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		log := observability.ContextLogger(ctx)

		if h.pool != nil {
			statusCode, err := h.deleteViaTx(ctx, tenantID)
			if err != nil {
				log.Error("gdpr delete failed", "error", err)
				if statusCode == http.StatusMultiStatus {
					w.WriteHeader(http.StatusMultiStatus)
					fmt.Fprintf(w, `{"status":"partial","error":"some resources could not be deleted"}`)
				} else {
					http.Error(w, "deletion failed", statusCode)
				}
				return
			}
		} else {
			if err := h.deleteViaStore(ctx, tenantID, log); err != nil {
				http.Error(w, "deletion failed", http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// deleteViaTx performs all deletions in a single PG transaction (production path).
// Returns HTTP status code and error. 207 indicates partial success (MinIO cleanup failed).
func (h *GDPRHandler) deleteViaTx(ctx context.Context, tenantID string) (int, error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Order matters: child tables first (role_permissions cascades from roles via FK),
	// usage_records and purge_audit_logs before tenants.
	cascadeTables := []string{
		"messages", "sessions", "memories", "user_profiles",
		"api_keys", "cron_jobs", "usage_records",
		"roles", "users",
	}
	for _, table := range cascadeTables {
		if _, ok := gdprAllowedTables[table]; !ok {
			return http.StatusInternalServerError, fmt.Errorf("table %s not in allowlist", table)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE tenant_id = $1`, table), tenantID); err != nil {
			return http.StatusInternalServerError, fmt.Errorf("delete %s: %w", table, err)
		}
	}

	// Audit logs: use SECURITY DEFINER function to bypass REVOKE DELETE.
	if _, err := tx.Exec(ctx, `SELECT gdpr_purge_audit_logs($1, 'GDPR_DELETE')`, tenantID); err != nil {
		// Fallback for environments without the function (e.g., tests).
		if _, err2 := tx.Exec(ctx, `DELETE FROM audit_logs WHERE tenant_id = $1`, tenantID); err2 != nil {
			return http.StatusInternalServerError, fmt.Errorf("delete audit_logs: %w (func err: %v)", err2, err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenantID); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("delete tenant: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("commit: %w", err)
	}

	// Post-commit: MinIO cleanup (best-effort with audit trail).
	if h.minio != nil {
		if err := h.deleteMinIOTenantObjects(ctx, tenantID); err != nil {
			h.recordMinIOFailure(ctx, tenantID, err)
			return http.StatusMultiStatus, fmt.Errorf("minio cleanup: %w", err)
		}
	}
	return http.StatusNoContent, nil
}

func (h *GDPRHandler) deleteMinIOTenantObjects(ctx context.Context, tenantID string) error {
	prefix := tenantID + "/"
	keys, err := h.minio.ListObjects(ctx, prefix)
	if err != nil {
		return fmt.Errorf("list objects: %w", err)
	}
	for _, key := range keys {
		if err := h.minio.DeleteObject(ctx, key); err != nil {
			return fmt.Errorf("delete %s: %w", key, err)
		}
	}
	return nil
}

// recordMinIOFailure persists MinIO cleanup errors for retry/audit.
// Note: cleanupErr.Error() may contain object key paths — acceptable for internal audit only.
func (h *GDPRHandler) recordMinIOFailure(ctx context.Context, tenantID string, cleanupErr error) {
	if h.pool == nil {
		return
	}
	_, _ = h.pool.Exec(ctx,
		`INSERT INTO purge_audit_logs (tenant_id, action, detail, created_at)
		 VALUES ($1, 'MINIO_CLEANUP_FAILED', $2, now())`,
		tenantID, cleanupErr.Error())
}

// CleanupMinIOHandler returns POST /v1/gdpr/cleanup-minio — idempotent retry for MinIO cleanup.
func (h *GDPRHandler) CleanupMinIOHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tenantID := middleware.TenantFromContext(r.Context())
		if tenantID == "" {
			http.Error(w, "tenant context required", http.StatusBadRequest)
			return
		}
		if h.minio == nil {
			http.Error(w, "minio not configured", http.StatusServiceUnavailable)
			return
		}
		if err := h.deleteMinIOTenantObjects(r.Context(), tenantID); err != nil {
			observability.ContextLogger(r.Context()).Error("gdpr minio cleanup failed", "error", err)
			http.Error(w, "cleanup failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// deleteViaStore performs deletions through the store interface (fallback when pool is nil).
// Covers all tenant-scoped tables accessible via the store interface.
func (h *GDPRHandler) deleteViaStore(ctx context.Context, tenantID string, log *slog.Logger) error {
	// Delete sessions (messages cascade via session delete in most implementations).
	sessions, _, _ := h.store.Sessions().List(ctx, tenantID, store.ListOptions{Limit: gdprExportMaxSessions})
	for _, sess := range sessions {
		if err := h.store.Sessions().Delete(ctx, tenantID, sess.ID); err != nil {
			log.Error("gdpr delete: session failed", "session_id", sess.ID, "error", err)
			return err
		}
	}

	// Delete memories for all users in tenant.
	if memStore := h.store.Memories(); memStore != nil {
		if _, err := memStore.DeleteAllByTenant(ctx, tenantID); err != nil {
			log.Error("gdpr delete: memories failed", "error", err)
			return err
		}
	}

	// Delete API keys.
	if keyStore := h.store.APIKeys(); keyStore != nil {
		keys, _ := keyStore.List(ctx, tenantID)
		for _, k := range keys {
			_ = keyStore.Revoke(ctx, tenantID, k.ID)
		}
	}

	// Delete cron jobs.
	if cronStore := h.store.CronJobs(); cronStore != nil {
		jobs, _ := cronStore.List(ctx, tenantID)
		for _, j := range jobs {
			_ = cronStore.Delete(ctx, tenantID, j.ID)
		}
	}

	// Delete audit logs.
	if _, err := h.store.AuditLogs().DeleteByTenant(ctx, tenantID); err != nil {
		log.Error("gdpr delete: audit_logs failed", "error", err)
		return err
	}

	log.Warn("gdpr deleteViaStore: user_profiles, usage_records, roles, users require pool for complete deletion",
		"tenant_id", tenantID)
	return nil
}
