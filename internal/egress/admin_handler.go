package egress

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

type AdminHandler struct {
	store      RuleAdminStore
	policy     EgressPolicy
	auditStore store.AuditLogStore
}

type AdminOption func(*AdminHandler)

func WithAuditStore(s store.AuditLogStore) AdminOption {
	return func(h *AdminHandler) { h.auditStore = s }
}

func NewAdminHandler(store RuleAdminStore, policy EgressPolicy, opts ...AdminOption) *AdminHandler {
	h := &AdminHandler{store: store, policy: policy}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// RegisterRoutes registers admin egress endpoints.
// IMPORTANT: caller must wrap mux with admin auth middleware (e.g. RequireScope("admin"))
// before exposing these routes. See internal/api/admin/handler.go for the pattern.
func (h *AdminHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/egress/rules", h.listRules)
	mux.HandleFunc("POST /admin/egress/rules", h.createRule)
	mux.HandleFunc("DELETE /admin/egress/rules/", h.deleteRule)
}

// RegisterV1Routes registers admin egress endpoints under the /admin/v1/ prefix
// used by the centralised admin handler (internal/api/admin). The mux must
// already be wrapped with RequireScope("admin") by the caller.
func (h *AdminHandler) RegisterV1Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/v1/egress/allowlist", h.listRules)
	mux.HandleFunc("POST /admin/v1/egress/allowlist", h.createRule)
	mux.HandleFunc("DELETE /admin/v1/egress/allowlist/{id}", h.deleteRuleV1)
	mux.HandleFunc("GET /admin/v1/egress/blocked-log", h.blockedLog)
}

// deleteRuleV1 handles DELETE /admin/v1/egress/allowlist/{id} using the Go 1.22
// path value syntax instead of manual prefix-trimming.
func (h *AdminHandler) deleteRuleV1(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "rule id is required", http.StatusBadRequest)
		return
	}

	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		http.Error(w, "tenant_id is required", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteRule(r.Context(), id, tenantID); err != nil {
		slog.Error("delete egress rule failed", "error", err)
		http.Error(w, "operation failed", http.StatusInternalServerError)
		return
	}

	if h.policy != nil {
		h.policy.Reload(r.Context())
	}

	w.WriteHeader(http.StatusNoContent)
}

// blockedLog returns recent denied egress decisions persisted as audit logs.
func (h *AdminHandler) blockedLog(w http.ResponseWriter, r *http.Request) {
	if h.auditStore == nil {
		http.Error(w, "egress audit store not configured", http.StatusServiceUnavailable)
		return
	}

	tenantID := r.URL.Query().Get("tenant_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	opts := store.AuditListOptions{
		Action: EgressDeniedAuditAction,
		Limit:  limit,
		Offset: offset,
	}
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		from, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			http.Error(w, "invalid 'from' parameter: use RFC3339 format", http.StatusBadRequest)
			return
		}
		opts.From = &from
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		to, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			http.Error(w, "invalid 'to' parameter: use RFC3339 format", http.StatusBadRequest)
			return
		}
		opts.To = &to
	}

	events, total, err := h.auditStore.List(r.Context(), tenantID, opts)
	if err != nil {
		slog.Error("list egress blocked log failed", "error", err)
		http.Error(w, "operation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"blocked_events": events,
		"total":          total,
		"limit":          limit,
		"offset":         offset,
	})
}

func (h *AdminHandler) listRules(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	rules, err := h.store.ListRules(r.Context(), tenantID)
	if err != nil {
		slog.Error("list egress rules failed", "error", err)
		http.Error(w, "operation failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}

type createRuleRequest struct {
	TenantID    string `json:"tenant_id"`
	HostPattern string `json:"host_pattern"`
	PathPrefix  string `json:"path_prefix"`
	Action      Action `json:"action"`
	Priority    int    `json:"priority"`
}

func (h *AdminHandler) createRule(w http.ResponseWriter, r *http.Request) {
	var req createRuleRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.TenantID == "" || req.HostPattern == "" {
		http.Error(w, "tenant_id and host_pattern are required", http.StatusBadRequest)
		return
	}
	if req.Action == "" {
		req.Action = ActionAllow
	}
	if req.Action != ActionAllow && req.Action != ActionDeny {
		http.Error(w, "action must be 'allow' or 'deny'", http.StatusBadRequest)
		return
	}
	if req.PathPrefix == "" {
		req.PathPrefix = "/"
	}

	rule := EgressRule{
		TenantID:    req.TenantID,
		HostPattern: req.HostPattern,
		PathPrefix:  req.PathPrefix,
		Action:      req.Action,
		Priority:    req.Priority,
	}

	id, err := h.store.CreateRule(r.Context(), rule)
	if err != nil {
		slog.Error("create egress rule failed", "error", err)
		http.Error(w, "operation failed", http.StatusInternalServerError)
		return
	}

	if h.policy != nil {
		h.policy.Reload(r.Context())
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (h *AdminHandler) deleteRule(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/admin/egress/rules/")
	if id == "" {
		http.Error(w, "rule id is required", http.StatusBadRequest)
		return
	}

	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		http.Error(w, "tenant_id is required", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteRule(r.Context(), id, tenantID); err != nil {
		slog.Error("delete egress rule failed", "error", err)
		http.Error(w, "operation failed", http.StatusInternalServerError)
		return
	}

	if h.policy != nil {
		h.policy.Reload(r.Context())
	}

	w.WriteHeader(http.StatusNoContent)
}
