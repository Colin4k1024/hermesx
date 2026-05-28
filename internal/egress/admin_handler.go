package egress

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

type AdminHandler struct {
	store  RuleAdminStore
	policy EgressPolicy
}

func NewAdminHandler(store RuleAdminStore, policy EgressPolicy) *AdminHandler {
	return &AdminHandler{store: store, policy: policy}
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

// blockedLog returns recent blocked-egress events. Currently returns the
// in-memory allowlist policy state as a placeholder; a persistent blocked-log
// table can be wired in here once the schema migration lands.
func (h *AdminHandler) blockedLog(w http.ResponseWriter, r *http.Request) {
	if h.policy == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
		return
	}

	// Surface the current loaded rules so operators can see what is active.
	tenantID := r.URL.Query().Get("tenant_id")
	rules, err := h.store.ListRules(r.Context(), tenantID)
	if err != nil {
		slog.Error("list egress rules for blocked log failed", "error", err)
		http.Error(w, "operation failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"note":  "persistent blocked-log table pending migration; showing active rules",
		"rules": rules,
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
