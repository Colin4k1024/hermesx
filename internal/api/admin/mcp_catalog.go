package admin

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/mcpcatalog"
)

type mcpCatalogItemRequest struct {
	Name                string   `json:"name"`
	Version             string   `json:"version,omitempty"`
	Description         string   `json:"description,omitempty"`
	SourceURL           string   `json:"source_url"`
	TrustTier           string   `json:"trust_tier"`
	ReviewStatus        string   `json:"review_status"`
	Transport           string   `json:"transport"`
	Command             string   `json:"command,omitempty"`
	Args                []string `json:"args,omitempty"`
	URL                 string   `json:"url,omitempty"`
	RequiredCredentials []string `json:"required_credentials,omitempty"`
	Scopes              []string `json:"scopes,omitempty"`
	EgressDomains       []string `json:"egress_domains,omitempty"`
	SandboxRequired     bool     `json:"sandbox_required"`
	Reason              string   `json:"reason,omitempty"`
}

type mcpTenantPolicyRequest struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason,omitempty"`
}

func (h *AdminHandler) listMCPCatalogItems(w http.ResponseWriter, r *http.Request) {
	if h.mcpCatalog == nil {
		http.Error(w, "mcp catalog not configured", http.StatusServiceUnavailable)
		return
	}
	items, err := h.mcpCatalog.ListItems(r.Context())
	if err != nil {
		h.logger.Error("list mcp catalog failed", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"items": items, "count": len(items)})
}

func (h *AdminHandler) getMCPCatalogItem(w http.ResponseWriter, r *http.Request) {
	if h.mcpCatalog == nil {
		http.Error(w, "mcp catalog not configured", http.StatusServiceUnavailable)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "catalog item id required", http.StatusBadRequest)
		return
	}
	item, err := h.mcpCatalog.GetItem(r.Context(), id)
	if err != nil {
		if errors.Is(err, mcpcatalog.ErrNotFound) {
			http.Error(w, "catalog item not found", http.StatusNotFound)
			return
		}
		h.logger.Error("get mcp catalog item failed", "item_id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, item)
}

func (h *AdminHandler) upsertMCPCatalogItem(w http.ResponseWriter, r *http.Request) {
	if h.mcpCatalog == nil {
		http.Error(w, "mcp catalog not configured", http.StatusServiceUnavailable)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "catalog item id required", http.StatusBadRequest)
		return
	}

	var req mcpCatalogItemRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	before, _ := h.mcpCatalog.GetItem(r.Context(), id)
	item, err := h.mcpCatalog.UpsertItem(r.Context(), mcpcatalog.Item{
		ID:                  id,
		Name:                req.Name,
		Version:             req.Version,
		Description:         req.Description,
		SourceURL:           req.SourceURL,
		TrustTier:           req.TrustTier,
		ReviewStatus:        req.ReviewStatus,
		Transport:           req.Transport,
		Command:             req.Command,
		Args:                req.Args,
		URL:                 req.URL,
		RequiredCredentials: req.RequiredCredentials,
		Scopes:              req.Scopes,
		EgressDomains:       req.EgressDomains,
		SandboxRequired:     req.SandboxRequired,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.appendGovernanceAudit(r, "admin.mcp_catalog.item.upsert", map[string]any{
		"item_id": id,
		"before":  before,
		"after":   item,
		"reason":  req.Reason,
	})
	writeJSON(w, item)
}

func (h *AdminHandler) listMCPTenantPolicies(w http.ResponseWriter, r *http.Request) {
	if h.mcpCatalog == nil {
		http.Error(w, "mcp catalog not configured", http.StatusServiceUnavailable)
		return
	}
	tenantID := r.PathValue("id")
	if tenantID == "" {
		http.Error(w, "tenant id required", http.StatusBadRequest)
		return
	}
	policies, err := h.mcpCatalog.ListTenantPolicies(r.Context(), tenantID)
	if err != nil {
		h.logger.Error("list mcp tenant policies failed", "tenant_id", tenantID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"tenant_id": tenantID,
		"policies":  policies,
		"count":     len(policies),
	})
}

func (h *AdminHandler) setMCPTenantPolicy(w http.ResponseWriter, r *http.Request) {
	if h.mcpCatalog == nil {
		http.Error(w, "mcp catalog not configured", http.StatusServiceUnavailable)
		return
	}
	tenantID := r.PathValue("id")
	itemID := r.PathValue("itemID")
	if tenantID == "" || itemID == "" {
		http.Error(w, "tenant id and item id required", http.StatusBadRequest)
		return
	}

	var req mcpTenantPolicyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	updatedBy := ""
	if ac, ok := auth.FromContext(r.Context()); ok && ac != nil {
		updatedBy = ac.Identity
	}
	policy, err := h.mcpCatalog.SetTenantPolicy(r.Context(), mcpcatalog.TenantItemPolicy{
		TenantID:  tenantID,
		ItemID:    itemID,
		Enabled:   req.Enabled,
		Reason:    req.Reason,
		UpdatedBy: updatedBy,
	})
	if err != nil {
		if errors.Is(err, mcpcatalog.ErrNotFound) {
			http.Error(w, "catalog item not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.appendGovernanceAudit(r, "admin.mcp_catalog.tenant_policy.set", map[string]any{
		"tenant_id": tenantID,
		"item_id":   itemID,
		"policy":    policy,
		"reason":    req.Reason,
	})
	writeJSON(w, policy)
}
