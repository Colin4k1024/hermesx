package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/hermes-agent/hermes-agent-go/internal/store"
)

// listAuditLogs serves GET /admin/v1/audit-logs with cross-tenant query capabilities.
// Query params: tenant_id, from, to, action, limit, offset.
// Requires admin scope (enforced by parent router).
func (h *AdminHandler) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tenantID := q.Get("tenant_id")
	action := q.Get("action")

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	opts := store.AuditListOptions{
		Action: action,
		Limit:  limit,
		Offset: offset,
	}

	if fromStr := q.Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			opts.From = &t
		} else {
			http.Error(w, "invalid 'from' parameter: use RFC3339 format", http.StatusBadRequest)
			return
		}
	}

	if toStr := q.Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			opts.To = &t
		} else {
			http.Error(w, "invalid 'to' parameter: use RFC3339 format", http.StatusBadRequest)
			return
		}
	}

	logs, total, err := h.store.AuditLogs().List(r.Context(), tenantID, opts)
	if err != nil {
		h.logger.Error("admin audit-logs list failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"audit_logs": logs,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}
