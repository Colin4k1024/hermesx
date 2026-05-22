package admin

import (
	"errors"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/evolution"
	"github.com/Colin4k1024/hermesx/internal/store"
)

type sharingPolicyRequest struct {
	Mode   string `json:"mode"`
	Reason string `json:"reason,omitempty"`
}

type tenantSharingPolicyRequest struct {
	ConsumeShared    bool     `json:"consume_shared"`
	ContributionMode string   `json:"contribution_mode"`
	Labels           []string `json:"labels,omitempty"`
	Reason           string   `json:"reason,omitempty"`
}

type sharedKnowledgeRevokeRequest struct {
	TaskClass    string `json:"task_class,omitempty"`
	SourceTenant string `json:"source_tenant,omitempty"`
	Source       string `json:"source,omitempty"`
	From         string `json:"from,omitempty"`
	To           string `json:"to,omitempty"`
	ConfirmAll   bool   `json:"confirm_all,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

type policyRollbackRequest struct {
	Version int64  `json:"version"`
	Reason  string `json:"reason,omitempty"`
}

func (h *AdminHandler) getEvolutionSharingPolicy(w http.ResponseWriter, _ *http.Request) {
	if h.evolutionStore == nil {
		http.Error(w, "evolution store not configured", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, h.evolutionStore.SharingPolicySnapshot())
}

func parseHistoryWindow(r *http.Request) (int, int) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (h *AdminHandler) listEvolutionSharingPolicyHistory(w http.ResponseWriter, r *http.Request) {
	if h.evolutionStore == nil {
		http.Error(w, "evolution store not configured", http.StatusServiceUnavailable)
		return
	}
	limit, offset := parseHistoryWindow(r)
	entries, err := h.evolutionStore.ListSharingPolicyHistory(limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"entries": entries, "limit": limit, "offset": offset})
}

func (h *AdminHandler) updateEvolutionSharingPolicy(w http.ResponseWriter, r *http.Request) {
	if h.evolutionStore == nil {
		http.Error(w, "evolution store not configured", http.StatusServiceUnavailable)
		return
	}
	var req sharingPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Mode != evolution.SharingDisabled && req.Mode != evolution.SharingAnonymous && req.Mode != evolution.SharingTrusted {
		http.Error(w, "invalid sharing mode", http.StatusBadRequest)
		return
	}
	before := h.evolutionStore.SharingPolicySnapshot()
	after, err := h.evolutionStore.SetSharingMode(req.Mode, req.Reason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.appendGovernanceAudit(r, "admin.evolution.sharing_policy.update", map[string]any{
		"before": before,
		"after":  after,
		"reason": req.Reason,
	})
	writeJSON(w, after)
}

func (h *AdminHandler) rollbackEvolutionSharingPolicy(w http.ResponseWriter, r *http.Request) {
	if h.evolutionStore == nil {
		http.Error(w, "evolution store not configured", http.StatusServiceUnavailable)
		return
	}
	var req policyRollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Version <= 0 {
		http.Error(w, "version must be > 0", http.StatusBadRequest)
		return
	}
	before := h.evolutionStore.SharingPolicySnapshot()
	if req.Version == before.Version {
		http.Error(w, "already at requested version", http.StatusBadRequest)
		return
	}
	after, err := h.evolutionStore.RollbackSharingPolicy(req.Version, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, evolution.ErrPolicyVersionNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	h.appendGovernanceAudit(r, "admin.evolution.sharing_policy.rollback", map[string]any{
		"before":         before,
		"after":          after,
		"target_version": req.Version,
		"reason":         req.Reason,
	})
	writeJSON(w, after)
}

func (h *AdminHandler) getEvolutionTenantSharingPolicy(w http.ResponseWriter, r *http.Request) {
	if h.evolutionStore == nil {
		http.Error(w, "evolution store not configured", http.StatusServiceUnavailable)
		return
	}
	tenantID := r.PathValue("id")
	if tenantID == "" {
		http.Error(w, "tenant id required", http.StatusBadRequest)
		return
	}
	writeJSON(w, h.evolutionStore.EffectiveTenantSharingPolicy(tenantID))
}

func (h *AdminHandler) listEvolutionTenantSharingPolicyHistory(w http.ResponseWriter, r *http.Request) {
	if h.evolutionStore == nil {
		http.Error(w, "evolution store not configured", http.StatusServiceUnavailable)
		return
	}
	tenantID := r.PathValue("id")
	if tenantID == "" {
		http.Error(w, "tenant id required", http.StatusBadRequest)
		return
	}
	limit, offset := parseHistoryWindow(r)
	entries, err := h.evolutionStore.ListTenantSharingPolicyHistory(tenantID, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"entries": entries, "limit": limit, "offset": offset})
}

func (h *AdminHandler) updateEvolutionTenantSharingPolicy(w http.ResponseWriter, r *http.Request) {
	if h.evolutionStore == nil {
		http.Error(w, "evolution store not configured", http.StatusServiceUnavailable)
		return
	}
	tenantID := r.PathValue("id")
	if tenantID == "" {
		http.Error(w, "tenant id required", http.StatusBadRequest)
		return
	}
	var req tenantSharingPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.ContributionMode != evolution.SharingDisabled && req.ContributionMode != evolution.SharingAnonymous && req.ContributionMode != evolution.SharingTrusted {
		http.Error(w, "invalid contribution mode", http.StatusBadRequest)
		return
	}
	before := h.evolutionStore.EffectiveTenantSharingPolicy(tenantID)
	after, err := h.evolutionStore.SetTenantSharingPolicy(evolution.TenantSharingPolicy{
		TenantID:         tenantID,
		ConsumeShared:    req.ConsumeShared,
		ContributionMode: req.ContributionMode,
		Labels:           req.Labels,
	}, req.Reason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.appendGovernanceAudit(r, "admin.evolution.tenant_sharing_policy.update", map[string]any{
		"tenant_id": tenantID,
		"before":    before,
		"after":     after,
		"reason":    req.Reason,
	})
	writeJSON(w, after)
}

func (h *AdminHandler) rollbackEvolutionTenantSharingPolicy(w http.ResponseWriter, r *http.Request) {
	if h.evolutionStore == nil {
		http.Error(w, "evolution store not configured", http.StatusServiceUnavailable)
		return
	}
	tenantID := r.PathValue("id")
	if tenantID == "" {
		http.Error(w, "tenant id required", http.StatusBadRequest)
		return
	}
	var req policyRollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Version <= 0 {
		http.Error(w, "version must be > 0", http.StatusBadRequest)
		return
	}
	before := h.evolutionStore.EffectiveTenantSharingPolicy(tenantID)
	if req.Version == before.Version {
		http.Error(w, "already at requested version", http.StatusBadRequest)
		return
	}
	after, err := h.evolutionStore.RollbackTenantSharingPolicy(tenantID, req.Version, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, evolution.ErrPolicyVersionNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	h.appendGovernanceAudit(r, "admin.evolution.tenant_sharing_policy.rollback", map[string]any{
		"tenant_id":      tenantID,
		"before":         before,
		"after":          after,
		"target_version": req.Version,
		"reason":         req.Reason,
	})
	writeJSON(w, after)
}

func (h *AdminHandler) revokeEvolutionSharedKnowledge(w http.ResponseWriter, r *http.Request) {
	if h.evolutionStore == nil {
		http.Error(w, "evolution store not configured", http.StatusServiceUnavailable)
		return
	}
	var req sharedKnowledgeRevokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	criteria := evolution.SharedRevokeCriteria{
		TaskClass:    req.TaskClass,
		SourceTenant: req.SourceTenant,
		Source:       req.Source,
		ConfirmAll:   req.ConfirmAll,
	}
	var err error
	if req.From != "" {
		from, parseErr := time.Parse(time.RFC3339, req.From)
		if parseErr != nil {
			http.Error(w, "invalid 'from' parameter: use RFC3339 format", http.StatusBadRequest)
			return
		}
		criteria.From = &from
	}
	if req.To != "" {
		to, parseErr := time.Parse(time.RFC3339, req.To)
		if parseErr != nil {
			http.Error(w, "invalid 'to' parameter: use RFC3339 format", http.StatusBadRequest)
			return
		}
		criteria.To = &to
	}

	deleted, err := h.evolutionStore.RevokeShared(r.Context(), criteria)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.appendGovernanceAudit(r, "admin.evolution.shared_knowledge.revoke", map[string]any{
		"criteria": criteria,
		"deleted":  deleted,
		"reason":   req.Reason,
	})
	writeJSON(w, map[string]any{
		"deleted": deleted,
	})
}

func (h *AdminHandler) appendGovernanceAudit(r *http.Request, action string, detail map[string]any) {
	if h.store == nil || h.store.AuditLogs() == nil {
		return
	}
	detailBytes, _ := json.Marshal(detail)
	entry := &store.AuditLog{
		Action:    action,
		Detail:    string(detailBytes),
		RequestID: r.Header.Get("X-Request-ID"),
		SourceIP:  r.RemoteAddr,
		UserAgent: r.UserAgent(),
	}
	if ac, ok := auth.FromContext(r.Context()); ok && ac != nil {
		entry.TenantID = ac.TenantID
		entry.UserID = ac.Identity
	}
	if err := h.store.AuditLogs().Append(r.Context(), entry); err != nil {
		h.logger.Warn("admin governance audit append failed", "action", action, "error", err)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
