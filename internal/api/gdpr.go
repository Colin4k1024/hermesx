package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Colin4k1024/hermesx/internal/metering"
	"github.com/Colin4k1024/hermesx/internal/middleware"
	"github.com/Colin4k1024/hermesx/internal/objstore"
	"github.com/Colin4k1024/hermesx/internal/observability"
	"github.com/Colin4k1024/hermesx/internal/store"
)

const (
	gdprExportMaxSessions        = 1000
	gdprExportAlertEventPageSize = 500
)

// sessionExport pairs a session with its messages for GDPR export.
type sessionExport struct {
	Session  *store.Session   `json:"session"`
	Messages []*store.Message `json:"messages"`
}

// exportData is the top-level JSON structure for GDPR data export.
type exportData struct {
	TenantID            string                      `json:"tenant_id"`
	Sessions            []sessionExport             `json:"sessions"`
	Memories            []memoryExportEntry         `json:"memories"`
	Profiles            []profileExportEntry        `json:"profiles"`
	WorkflowDefinitions []*store.WorkflowDefinition `json:"workflow_definitions,omitempty"`
	WorkflowRuns        []*store.WorkflowRun        `json:"workflow_runs,omitempty"`
	AlertRules          []*metering.AlertRule       `json:"alert_rules,omitempty"`
	AlertEvents         []*metering.AlertEvent      `json:"alert_events,omitempty"`
}

type memoryExportEntry struct {
	UserID  string `json:"user_id"`
	Key     string `json:"key"`
	Content string `json:"content"`
}

type profileExportEntry struct {
	UserID  string `json:"user_id"`
	Content string `json:"content"`
}

// GDPRHandler serves data export and deletion endpoints.
// Uses store.Store for all operations — works with any backend (MySQL, PostgreSQL, SQLite).
type GDPRHandler struct {
	store       store.Store
	minio       objstore.ObjectStore
	alertRules  metering.AlertRuleStore  // optional; nil skips alert rule export
	alertEvents metering.AlertEventStore // optional; nil skips alert event export
}

func NewGDPRHandler(s store.Store, minio objstore.ObjectStore, alertRules metering.AlertRuleStore, alertEvents metering.AlertEventStore) *GDPRHandler {
	return &GDPRHandler{store: s, minio: minio, alertRules: alertRules, alertEvents: alertEvents}
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

		// Build session exports with messages.
		sessionExports := make([]sessionExport, 0, len(sessions))
		for _, sess := range sessions {
			msgs, err := h.store.Messages().List(ctx, tenantID, sess.ID, 1000, 0)
			if err != nil {
				log.Warn("gdpr export: failed to list messages", "session_id", sess.ID, "error", err)
				msgs = nil
			}
			sessionExports = append(sessionExports, sessionExport{Session: sess, Messages: msgs})
		}

		// Collect memories per user via store interface.
		var memories []memoryExportEntry
		if memStore := h.store.Memories(); memStore != nil {
			for uid := range userIDSet {
				entries, err := memStore.List(ctx, tenantID, uid)
				if err != nil {
					log.Warn("gdpr export: failed to list memories", "user_id", uid, "error", err)
					continue
				}
				for _, e := range entries {
					memories = append(memories, memoryExportEntry{UserID: uid, Key: e.Key, Content: e.Content})
				}
			}
		}

		// Collect user profiles via store interface.
		var profiles []profileExportEntry
		if profStore := h.store.UserProfiles(); profStore != nil {
			for uid := range userIDSet {
				content, err := profStore.Get(ctx, tenantID, uid)
				if err != nil || content == "" {
					continue
				}
				profiles = append(profiles, profileExportEntry{UserID: uid, Content: content})
			}
		}

		var workflowDefinitions []*store.WorkflowDefinition
		var workflowRuns []*store.WorkflowRun
		if wfStore := h.store.Workflows(); wfStore != nil {
			workflowDefinitions, _ = wfStore.ListDefinitions(ctx, tenantID)
			workflowRuns, _, _ = wfStore.ListRuns(ctx, tenantID, store.WorkflowRunListOptions{Limit: 1000})
		}

		// Collect alert rules and events for this tenant.
		var alertRules []*metering.AlertRule
		if h.alertRules != nil {
			alertRules, err = h.alertRules.List(ctx, tenantID)
			if err != nil {
				log.Warn("gdpr export: failed to list alert rules", "tenant_id", tenantID, "error", err)
			}
		}
		var alertEvents []*metering.AlertEvent
		if h.alertEvents != nil {
			alertEvents, err = h.listAllAlertEvents(ctx, tenantID)
			if err != nil {
				log.Warn("gdpr export: failed to list alert events", "tenant_id", tenantID, "error", err)
			}
		}

		// Audit the export action.
		_ = h.store.AuditLogs().Append(ctx, &store.AuditLog{
			TenantID:  tenantID,
			Action:    "GDPR_EXPORT",
			Detail:    fmt.Sprintf("sessions=%d memories=%d profiles=%d workflow_definitions=%d workflow_runs=%d alert_rules=%d alert_events=%d", len(sessions), len(memories), len(profiles), len(workflowDefinitions), len(workflowRuns), len(alertRules), len(alertEvents)),
			RequestID: middleware.RequestIDFromContext(ctx),
		})

		export := exportData{
			TenantID:            tenantID,
			Sessions:            sessionExports,
			Memories:            memories,
			Profiles:            profiles,
			WorkflowDefinitions: workflowDefinitions,
			WorkflowRuns:        workflowRuns,
			AlertRules:          alertRules,
			AlertEvents:         alertEvents,
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=export.json")
		if err := json.NewEncoder(w).Encode(export); err != nil {
			log.Error("gdpr export: failed to encode response", "error", err)
		}
	}
}

func (h *GDPRHandler) listAllAlertEvents(ctx context.Context, tenantID string) ([]*metering.AlertEvent, error) {
	var all []*metering.AlertEvent
	for offset := 0; ; offset += gdprExportAlertEventPageSize {
		page, err := h.alertEvents.ListByTenantPage(ctx, tenantID, gdprExportAlertEventPageSize, offset)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < gdprExportAlertEventPageSize {
			return all, nil
		}
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

		ctx := r.Context()

		// First check if the tenant is active (not soft-deleted).
		tenant, err := h.store.Tenants().Get(ctx, tenantID)
		if err == nil {
			_ = tenant
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"status": "active",
			})
			return
		}

		if !errors.Is(err, store.ErrNotFound) {
			// Unexpected error from Get — treat as server error.
			http.Error(w, "failed to check tenant status", http.StatusInternalServerError)
			return
		}

		// Tenant not found via Get (deleted_at IS NULL) — check if it's soft-deleted within the grace period.
		cutoff := time.Now().Add(-GDPRGracePeriod)
		deleted, err := h.store.Tenants().ListDeleted(ctx, cutoff)
		if err != nil {
			http.Error(w, "failed to check deletion status", http.StatusInternalServerError)
			return
		}

		for _, d := range deleted {
			if d.ID == tenantID {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{
					"status":  "pending_deletion",
					"message": "tenant is scheduled for deletion; use POST /v1/gdpr/restore to cancel",
				})
				return
			}
		}

		// Tenant not found in active or deleted-within-grace-period lists.
		http.Error(w, "tenant not found", http.StatusNotFound)
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
