package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Colin4k1024/hermesx/internal/middleware"
	"github.com/Colin4k1024/hermesx/internal/objstore"
	"github.com/Colin4k1024/hermesx/internal/observability"
	"github.com/Colin4k1024/hermesx/internal/store"
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
// Uses store.Store for all operations — works with any backend (MySQL, PostgreSQL, SQLite).
type GDPRHandler struct {
	store store.Store
	minio objstore.ObjectStore
}

func NewGDPRHandler(s store.Store, minio objstore.ObjectStore) *GDPRHandler {
	return &GDPRHandler{store: s, minio: minio}
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

		// Collect unique user IDs from sessions.
		userIDSet := make(map[string]struct{})
		for _, sess := range sessions {
			if sess.UserID != "" {
				userIDSet[sess.UserID] = struct{}{}
			}
		}

		// Collect memories per user via store interface.
		type memoryEntry struct {
			UserID  string `json:"user_id"`
			Key     string `json:"key"`
			Content string `json:"content"`
		}
		var memories []memoryEntry
		if memStore := h.store.Memories(); memStore != nil {
			for uid := range userIDSet {
				entries, err := memStore.List(ctx, tenantID, uid)
				if err != nil {
					log.Warn("gdpr export: failed to list memories", "user_id", uid, "error", err)
					continue
				}
				for _, e := range entries {
					memories = append(memories, memoryEntry{UserID: uid, Key: e.Key, Content: e.Content})
				}
			}
		}

		// Collect user profiles via store interface.
		type profileEntry struct {
			UserID  string `json:"user_id"`
			Content string `json:"content"`
		}
		var profiles []profileEntry
		if profStore := h.store.UserProfiles(); profStore != nil {
			for uid := range userIDSet {
				content, err := profStore.Get(ctx, tenantID, uid)
				if err != nil || content == "" {
					continue
				}
				profiles = append(profiles, profileEntry{UserID: uid, Content: content})
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
			type sessionExport struct {
				Session  *store.Session   `json:"session"`
				Messages []*store.Message `json:"messages"`
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

// DeleteHandler returns DELETE /v1/gdpr/data — deletes all data for a tenant via the store interface.
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

		if err := h.deleteViaStore(ctx, tenantID, log); err != nil {
			http.Error(w, "deletion failed", http.StatusInternalServerError)
			return
		}

		// Post-delete: clean up object store (best-effort).
		if h.minio != nil {
			if err := h.deleteMinIOTenantObjects(ctx, tenantID); err != nil {
				log.Warn("gdpr delete: minio cleanup failed", "tenant_id", tenantID, "error", err)
				// Record failure via audit log (best-effort).
				_ = h.store.AuditLogs().Append(ctx, &store.AuditLog{
					TenantID: tenantID,
					Action:   "MINIO_CLEANUP_FAILED",
					Detail:   err.Error(),
				})
				w.WriteHeader(http.StatusMultiStatus)
				fmt.Fprintf(w, `{"status":"partial","error":"some resources could not be deleted"}`)
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
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

// CleanupMinIOHandler returns POST /v1/gdpr/cleanup-minio — idempotent retry for MinIO/RustFS cleanup.
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

// deleteViaStore performs deletions through the store interface (works with MySQL, PostgreSQL, SQLite).
func (h *GDPRHandler) deleteViaStore(ctx context.Context, tenantID string, log *slog.Logger) error {
	// Delete sessions (messages typically cascade via FK or session delete).
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

	// Delete user profiles for all users in tenant.
	if profStore := h.store.UserProfiles(); profStore != nil {
		if _, err := profStore.DeleteAllByTenant(ctx, tenantID); err != nil {
			log.Error("gdpr delete: user_profiles failed", "error", err)
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

	return nil
}
