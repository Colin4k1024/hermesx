package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Colin4k1024/hermesx/internal/middleware"
	"github.com/Colin4k1024/hermesx/internal/objstore"
	"github.com/Colin4k1024/hermesx/internal/observability"
	"github.com/Colin4k1024/hermesx/internal/store"
)

const gdprExportMaxSessions = 1000

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

		var workflowDefinitions []*store.WorkflowDefinition
		var workflowRuns []*store.WorkflowRun
		if wfStore := h.store.Workflows(); wfStore != nil {
			workflowDefinitions, _ = wfStore.ListDefinitions(ctx, tenantID)
			workflowRuns, _, _ = wfStore.ListRuns(ctx, tenantID, store.WorkflowRunListOptions{Limit: 1000})
		}

		// Audit the export action.
		_ = h.store.AuditLogs().Append(ctx, &store.AuditLog{
			TenantID:  tenantID,
			Action:    "GDPR_EXPORT",
			Detail:    fmt.Sprintf("sessions=%d memories=%d profiles=%d workflow_definitions=%d workflow_runs=%d", len(sessions), len(memories), len(profiles), len(workflowDefinitions), len(workflowRuns)),
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
		fmt.Fprint(w, `,"workflow_definitions":`)
		enc.Encode(workflowDefinitions)
		fmt.Fprint(w, `,"workflow_runs":`)
		enc.Encode(workflowRuns)
		fmt.Fprint(w, `}`)
	}
}

// GDPRGracePeriod is the retention window before soft-deleted data is permanently purged.
const GDPRGracePeriod = 30 * 24 * time.Hour

// DeleteHandler returns DELETE /v1/gdpr/data — initiates soft-delete with a 30-day grace period.
// Data is not immediately removed; the TenantCleanupJob will purge it after the grace window.
// During the grace period, the tenant can be restored via POST /v1/gdpr/restore.
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

		if err := h.store.Tenants().Delete(ctx, tenantID); err != nil {
			http.Error(w, "deletion request failed", http.StatusInternalServerError)
			return
		}

		_ = h.store.AuditLogs().Append(ctx, &store.AuditLog{
			TenantID:  tenantID,
			Action:    "GDPR_DELETE_REQUESTED",
			Detail:    fmt.Sprintf("grace_period=%s", GDPRGracePeriod),
			RequestID: middleware.RequestIDFromContext(ctx),
		})

		purgeAt := time.Now().Add(GDPRGracePeriod)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]any{
			"status":       "scheduled",
			"grace_period": GDPRGracePeriod.String(),
			"purge_after":  purgeAt.Format(time.RFC3339),
			"restore_url":  "/v1/gdpr/restore",
		})
	}
}

// RestoreHandler returns POST /v1/gdpr/restore — cancels a pending deletion during the grace period.
func (h *GDPRHandler) RestoreHandler() http.HandlerFunc {
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

		ctx := r.Context()

		if err := h.store.Tenants().Restore(ctx, tenantID); err != nil {
			http.Error(w, "restore failed: tenant not found or grace period expired", http.StatusGone)
			return
		}

		_ = h.store.AuditLogs().Append(ctx, &store.AuditLog{
			TenantID:  tenantID,
			Action:    "GDPR_DELETE_CANCELLED",
			Detail:    "tenant restored during grace period",
			RequestID: middleware.RequestIDFromContext(ctx),
		})

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"restored"}`)
	}
}

// DeletionStatusHandler returns GET /v1/gdpr/status — reports the deletion state and remaining grace time.
func (h *GDPRHandler) DeletionStatusHandler() http.HandlerFunc {
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

		tenant, err := h.store.Tenants().Get(r.Context(), tenantID)
		if err != nil {
			// Tenant not found via normal Get (filters deleted_at IS NULL) — check deleted state.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"status":  "pending_deletion",
				"message": "tenant is scheduled for deletion; use POST /v1/gdpr/restore to cancel",
			})
			return
		}

		_ = tenant
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "active",
		})
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
