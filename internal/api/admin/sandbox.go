package admin

import (
	"encoding/json"
	"net/http"

	"github.com/Colin4k1024/hermesx/internal/store"
)

func (h *AdminHandler) setSandboxPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		http.Error(w, "tenant id required", http.StatusBadRequest)
		return
	}

	var policy store.SandboxPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Fetch the tenant to confirm it exists.
	tenant, err := h.store.Tenants().Get(r.Context(), tenantID)
	if err != nil || tenant == nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}

	// Update sandbox policy.
	tenant.SandboxPolicy = &policy
	if err := h.store.Tenants().Update(r.Context(), tenant); err != nil {
		h.logger.Error("failed to update sandbox policy", "tenant_id", tenantID, "error", err)
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}

	h.logger.Info("sandbox policy set", "tenant_id", tenantID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tenant_id":      tenantID,
		"sandbox_policy": policy,
	})
}

func (h *AdminHandler) getSandboxPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		http.Error(w, "tenant id required", http.StatusBadRequest)
		return
	}

	tenant, err := h.store.Tenants().Get(r.Context(), tenantID)
	if err != nil || tenant == nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if tenant.SandboxPolicy == nil {
		json.NewEncoder(w).Encode(map[string]any{
			"tenant_id":      tenantID,
			"sandbox_policy": nil,
			"using_default":  true,
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"tenant_id":      tenantID,
		"sandbox_policy": tenant.SandboxPolicy,
		"using_default":  false,
	})
}

func (h *AdminHandler) deleteSandboxPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		http.Error(w, "tenant id required", http.StatusBadRequest)
		return
	}

	tenant, err := h.store.Tenants().Get(r.Context(), tenantID)
	if err != nil || tenant == nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}

	// Reset to default (nil).
	tenant.SandboxPolicy = nil
	if err := h.store.Tenants().Update(r.Context(), tenant); err != nil {
		h.logger.Error("failed to reset sandbox policy", "tenant_id", tenantID, "error", err)
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}

	h.logger.Info("sandbox policy reset to default", "tenant_id", tenantID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tenant_id":     tenantID,
		"using_default": true,
		"message":       "sandbox policy reset to system default",
	})
}
