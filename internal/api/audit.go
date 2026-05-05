package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/hermes-agent/hermes-agent-go/internal/middleware"
	"github.com/hermes-agent/hermes-agent-go/internal/store"
)

// AuditHandler serves GET /v1/audit-logs for the current tenant.
type AuditHandler struct {
	store store.AuditLogStore
}

func NewAuditHandler(s store.AuditLogStore) *AuditHandler {
	return &AuditHandler{store: s}
}

func (h *AuditHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tenantID := middleware.TenantFromContext(r.Context())
	if tenantID == "" {
		http.Error(w, "tenant context required", http.StatusBadRequest)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	action := r.URL.Query().Get("action")

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	opts := store.AuditListOptions{
		Action: action,
		Limit:  limit,
		Offset: offset,
	}

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			opts.From = &t
		} else {
			http.Error(w, "invalid 'from' parameter: use RFC3339 format", http.StatusBadRequest)
			return
		}
	}

	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			opts.To = &t
		} else {
			http.Error(w, "invalid 'to' parameter: use RFC3339 format", http.StatusBadRequest)
			return
		}
	}

	logs, total, err := h.store.List(r.Context(), tenantID, opts)
	if err != nil {
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
