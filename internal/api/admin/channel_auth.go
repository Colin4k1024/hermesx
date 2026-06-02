package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/store"
)

type channelAppRequest struct {
	TenantID         string `json:"tenant_id"`
	Platform         string `json:"platform"`
	AppKey           string `json:"app_key"`
	AppSecretRef     string `json:"app_secret_ref,omitempty"`
	OAuthSecretRef   string `json:"oauth_secret_ref,omitempty"`
	WebhookSecretRef string `json:"webhook_secret_ref,omitempty"`
	Enabled          *bool  `json:"enabled,omitempty"`
}

func (h *AdminHandler) channelStores() (store.ChannelAppStore, store.ChannelIdentityStore, store.BrowserSessionStore, bool) {
	provider, ok := h.store.(store.ChannelStoreProvider)
	if !ok {
		return nil, nil, nil, false
	}
	return provider.ChannelApps(), provider.ChannelIdentities(), provider.BrowserSessions(), true
}

func (h *AdminHandler) listChannelApps(w http.ResponseWriter, r *http.Request) {
	apps, _, _, ok := h.channelStores()
	if !ok {
		jsonError(w, "channel store not configured", http.StatusServiceUnavailable)
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	if tenantID == "" {
		jsonError(w, "tenant_id query parameter is required", http.StatusBadRequest)
		return
	}
	limit, offset := parseChannelListWindow(r)
	items, total, err := apps.List(r.Context(), tenantID, store.ListOptions{Limit: limit, Offset: offset})
	if err != nil {
		h.logger.Error("list channel apps failed", "error", err, "tenant_id", tenantID)
		jsonError(w, "failed to list channel apps", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
}

func (h *AdminHandler) createChannelApp(w http.ResponseWriter, r *http.Request) {
	apps, _, _, ok := h.channelStores()
	if !ok {
		jsonError(w, "channel store not configured", http.StatusServiceUnavailable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req channelAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.Platform = strings.TrimSpace(req.Platform)
	req.AppKey = strings.TrimSpace(req.AppKey)
	if req.TenantID == "" || req.Platform == "" || req.AppKey == "" {
		jsonError(w, "tenant_id, platform, and app_key are required", http.StatusBadRequest)
		return
	}
	if _, err := h.store.Tenants().Get(r.Context(), req.TenantID); err != nil {
		jsonError(w, "tenant not found", http.StatusNotFound)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	app := &store.ChannelApp{
		TenantID:         req.TenantID,
		Platform:         req.Platform,
		AppKey:           req.AppKey,
		AppSecretRef:     strings.TrimSpace(req.AppSecretRef),
		OAuthSecretRef:   strings.TrimSpace(req.OAuthSecretRef),
		WebhookSecretRef: strings.TrimSpace(req.WebhookSecretRef),
		Enabled:          enabled,
	}
	if err := apps.Create(r.Context(), app); err != nil {
		h.logger.Error("create channel app failed", "error", err, "tenant_id", req.TenantID, "platform", req.Platform)
		jsonError(w, "failed to create channel app", http.StatusInternalServerError)
		return
	}
	h.auditChannelAdmin(r, req.TenantID, "CHANNEL_APP_CREATED", "app_id="+app.ID+",platform="+app.Platform)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(app)
}

func (h *AdminHandler) updateChannelApp(w http.ResponseWriter, r *http.Request) {
	apps, _, _, ok := h.channelStores()
	if !ok {
		jsonError(w, "channel store not configured", http.StatusServiceUnavailable)
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	id := strings.TrimSpace(r.PathValue("id"))
	if tenantID == "" || id == "" {
		jsonError(w, "tenant_id query parameter and id path parameter are required", http.StatusBadRequest)
		return
	}
	existing, err := apps.GetByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			jsonError(w, "channel app not found", http.StatusNotFound)
			return
		}
		jsonError(w, "failed to load channel app", http.StatusInternalServerError)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req channelAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Platform) != "" {
		existing.Platform = strings.TrimSpace(req.Platform)
	}
	if strings.TrimSpace(req.AppKey) != "" {
		existing.AppKey = strings.TrimSpace(req.AppKey)
	}
	if req.AppSecretRef != "" {
		existing.AppSecretRef = strings.TrimSpace(req.AppSecretRef)
	}
	if req.OAuthSecretRef != "" {
		existing.OAuthSecretRef = strings.TrimSpace(req.OAuthSecretRef)
	}
	if req.WebhookSecretRef != "" {
		existing.WebhookSecretRef = strings.TrimSpace(req.WebhookSecretRef)
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := apps.Update(r.Context(), existing); err != nil {
		h.logger.Error("update channel app failed", "error", err, "tenant_id", tenantID, "app_id", id)
		jsonError(w, "failed to update channel app", http.StatusInternalServerError)
		return
	}
	h.auditChannelAdmin(r, tenantID, "CHANNEL_APP_UPDATED", "app_id="+id)
	writeJSON(w, existing)
}

func (h *AdminHandler) deleteChannelApp(w http.ResponseWriter, r *http.Request) {
	apps, _, _, ok := h.channelStores()
	if !ok {
		jsonError(w, "channel store not configured", http.StatusServiceUnavailable)
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	id := strings.TrimSpace(r.PathValue("id"))
	if tenantID == "" || id == "" {
		jsonError(w, "tenant_id query parameter and id path parameter are required", http.StatusBadRequest)
		return
	}
	if err := apps.Delete(r.Context(), tenantID, id); err != nil {
		h.logger.Error("delete channel app failed", "error", err, "tenant_id", tenantID, "app_id", id)
		jsonError(w, "failed to delete channel app", http.StatusInternalServerError)
		return
	}
	h.auditChannelAdmin(r, tenantID, "CHANNEL_APP_DELETED", "app_id="+id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandler) listChannelBindings(w http.ResponseWriter, r *http.Request) {
	_, identities, _, ok := h.channelStores()
	if !ok {
		jsonError(w, "channel store not configured", http.StatusServiceUnavailable)
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	if tenantID == "" {
		jsonError(w, "tenant_id query parameter is required", http.StatusBadRequest)
		return
	}
	limit, offset := parseChannelListWindow(r)
	items, total, err := identities.List(r.Context(), tenantID, store.ListOptions{Limit: limit, Offset: offset})
	if err != nil {
		h.logger.Error("list channel bindings failed", "error", err, "tenant_id", tenantID)
		jsonError(w, "failed to list channel bindings", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
}

func (h *AdminHandler) revokeChannelBinding(w http.ResponseWriter, r *http.Request) {
	_, identities, sessions, ok := h.channelStores()
	if !ok {
		jsonError(w, "channel store not configured", http.StatusServiceUnavailable)
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	id := strings.TrimSpace(r.PathValue("id"))
	if tenantID == "" || id == "" {
		jsonError(w, "tenant_id query parameter and id path parameter are required", http.StatusBadRequest)
		return
	}
	identity, err := identities.GetByID(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			jsonError(w, "channel binding not found", http.StatusNotFound)
			return
		}
		jsonError(w, "failed to load channel binding", http.StatusInternalServerError)
		return
	}
	if err := identities.Revoke(r.Context(), tenantID, id); err != nil {
		jsonError(w, "failed to revoke channel binding", http.StatusInternalServerError)
		return
	}
	_ = sessions.RevokeByUser(r.Context(), tenantID, identity.UserID)
	h.auditChannelAdmin(r, tenantID, "CHANNEL_BINDING_REVOKED", "binding_id="+id+",user_id="+identity.UserID)
	w.WriteHeader(http.StatusNoContent)
}

func parseChannelListWindow(r *http.Request) (int, int) {
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

func (h *AdminHandler) auditChannelAdmin(r *http.Request, tenantID, action, detail string) {
	if h.store == nil || h.store.AuditLogs() == nil {
		return
	}
	entry := &store.AuditLog{
		TenantID:   tenantID,
		Action:     action,
		Detail:     detail,
		StatusCode: http.StatusOK,
		SourceIP:   r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	}
	if ac, ok := auth.FromContext(r.Context()); ok && ac != nil {
		entry.UserID = ac.Identity
	}
	if err := h.store.AuditLogs().Append(r.Context(), entry); err != nil {
		h.logger.Warn("channel admin audit append failed", "action", action, "error", err)
	}
}
