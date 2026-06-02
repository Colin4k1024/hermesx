package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/channel"
	"github.com/Colin4k1024/hermesx/internal/store"
)

const channelSessionTTL = 8 * time.Hour

type ChannelAuthConfig struct {
	HashSecret   string
	PublicURL    string
	CookieSecure bool
	Challenges   *channel.ChallengeStore
	Providers    *channel.ProviderRegistry
}

type ChannelAuthHandler struct {
	store        store.Store
	apps         store.ChannelAppStore
	identities   store.ChannelIdentityStore
	sessions     store.BrowserSessionStore
	challenges   *channel.ChallengeStore
	providers    *channel.ProviderRegistry
	hashSecret   string
	publicURL    string
	cookieSecure bool
}

func NewChannelAuthHandler(s store.Store, cfg ChannelAuthConfig) (*ChannelAuthHandler, bool) {
	provider, ok := s.(store.ChannelStoreProvider)
	if !ok || cfg.HashSecret == "" {
		return nil, false
	}
	challenges := cfg.Challenges
	if challenges == nil {
		challenges = channel.NewChallengeStore(10 * time.Minute)
	}
	if cfg.Providers == nil {
		return nil, false
	}
	return &ChannelAuthHandler{
		store:        s,
		apps:         provider.ChannelApps(),
		identities:   provider.ChannelIdentities(),
		sessions:     provider.BrowserSessions(),
		challenges:   challenges,
		providers:    cfg.Providers,
		hashSecret:   cfg.HashSecret,
		publicURL:    strings.TrimRight(cfg.PublicURL, "/"),
		cookieSecure: cfg.CookieSecure,
	}, true
}

func (h *ChannelAuthHandler) Start(w http.ResponseWriter, r *http.Request) {
	platform := r.PathValue("platform")
	appKey := strings.TrimSpace(r.URL.Query().Get("app_key"))
	challengeID := strings.TrimSpace(r.URL.Query().Get("challenge"))
	returnTo := sanitizeReturnTo(r.URL.Query().Get("return_to"))
	if platform == "" || appKey == "" || challengeID == "" {
		h.audit(r, "", "", "CHANNEL_AUTH_FAILED", "missing platform, app_key, or challenge", http.StatusBadRequest)
		http.Error(w, "platform, app_key, and challenge are required", http.StatusBadRequest)
		return
	}

	challengeValue, err := h.challenges.Peek(challengeID)
	if err != nil || challengeValue.Platform != platform || challengeValue.AppKey != appKey {
		h.audit(r, "", "", "CHANNEL_AUTH_FAILED", "challenge expired or mismatched", http.StatusBadRequest)
		http.Error(w, "challenge expired or mismatched", http.StatusBadRequest)
		return
	}
	if returnTo == "" {
		returnTo = challengeValue.ReturnTo
	}

	app, err := h.apps.GetByPlatformAppKey(r.Context(), platform, appKey)
	if err != nil || app == nil || !app.Enabled {
		h.audit(r, "", "", "CHANNEL_AUTH_FAILED", "channel app disabled or not found", http.StatusNotFound)
		http.Error(w, "channel app not found", http.StatusNotFound)
		return
	}
	if _, err := h.store.Tenants().Get(r.Context(), app.TenantID); err != nil {
		h.audit(r, app.TenantID, "", "CHANNEL_AUTH_FAILED", "tenant inactive or not found", http.StatusNotFound)
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}
	provider, ok := h.providers.Get(platform)
	if !ok {
		h.audit(r, app.TenantID, "", "CHANNEL_AUTH_FAILED", "channel provider not configured", http.StatusServiceUnavailable)
		http.Error(w, "channel provider not configured", http.StatusServiceUnavailable)
		return
	}

	state, err := h.challenges.CreateState(challengeID)
	if err != nil {
		h.audit(r, app.TenantID, "", "CHANNEL_AUTH_FAILED", err.Error(), http.StatusBadRequest)
		http.Error(w, "challenge expired", http.StatusBadRequest)
		return
	}
	redirectURI := h.callbackURL(r, platform)
	authURL, err := provider.AuthCodeURL(app, redirectURI, state.ID)
	if err != nil {
		h.audit(r, app.TenantID, "", "CHANNEL_AUTH_FAILED", err.Error(), http.StatusInternalServerError)
		http.Error(w, "failed to create provider auth url", http.StatusInternalServerError)
		return
	}

	h.audit(r, app.TenantID, "", "CHANNEL_LOGIN_STARTED", "platform="+platform, http.StatusFound)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *ChannelAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	platform := r.PathValue("platform")
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	stateID := strings.TrimSpace(r.URL.Query().Get("state"))
	if platform == "" || code == "" || stateID == "" {
		h.audit(r, "", "", "CHANNEL_AUTH_FAILED", "missing platform, code, or state", http.StatusBadRequest)
		http.Error(w, "platform, code, and state are required", http.StatusBadRequest)
		return
	}

	challengeValue, err := h.challenges.TakeState(stateID)
	if err != nil || challengeValue.Platform != platform {
		h.audit(r, "", "", "CHANNEL_AUTH_FAILED", "state expired or mismatched", http.StatusBadRequest)
		http.Error(w, "state expired or mismatched", http.StatusBadRequest)
		return
	}
	app, err := h.apps.GetByPlatformAppKey(r.Context(), platform, challengeValue.AppKey)
	if err != nil || app == nil || !app.Enabled {
		h.audit(r, "", "", "CHANNEL_AUTH_FAILED", "channel app disabled or not found", http.StatusNotFound)
		http.Error(w, "channel app not found", http.StatusNotFound)
		return
	}
	if _, err := h.store.Tenants().Get(r.Context(), app.TenantID); err != nil {
		h.audit(r, app.TenantID, "", "CHANNEL_AUTH_FAILED", "tenant inactive or not found", http.StatusNotFound)
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}
	provider, ok := h.providers.Get(platform)
	if !ok {
		h.audit(r, app.TenantID, "", "CHANNEL_AUTH_FAILED", "channel provider not configured", http.StatusServiceUnavailable)
		http.Error(w, "channel provider not configured", http.StatusServiceUnavailable)
		return
	}
	principal, err := provider.ExchangeCode(r.Context(), app, code)
	if err != nil {
		h.audit(r, app.TenantID, "", "CHANNEL_AUTH_FAILED", err.Error(), http.StatusUnauthorized)
		http.Error(w, "provider authentication failed", http.StatusUnauthorized)
		return
	}
	providerHash, err := channel.HashProviderUser(h.hashSecret, platform, app.AppKey, principal.ProviderUserID)
	if err != nil || providerHash != challengeValue.ExpectedUserHash {
		h.audit(r, app.TenantID, "", "CHANNEL_AUTH_FAILED", "provider user mismatch", http.StatusForbidden)
		http.Error(w, "provider user mismatch", http.StatusForbidden)
		return
	}

	user, err := h.store.Users().GetOrCreate(r.Context(), app.TenantID, "channel:"+platform+":"+providerHash, principal.DisplayName)
	if err != nil {
		h.audit(r, app.TenantID, "", "CHANNEL_AUTH_FAILED", "failed to create user", http.StatusInternalServerError)
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}
	identity := &store.ChannelIdentity{
		TenantID:            app.TenantID,
		ChannelAppID:        app.ID,
		Platform:            platform,
		ProviderUserHash:    providerHash,
		ProviderDisplayName: principal.DisplayName,
		UserID:              user.ID,
	}
	if err := h.identities.Upsert(r.Context(), identity); err != nil {
		h.audit(r, app.TenantID, user.ID, "CHANNEL_AUTH_FAILED", "failed to bind identity", http.StatusInternalServerError)
		http.Error(w, "failed to bind identity", http.StatusInternalServerError)
		return
	}

	if err := h.createBrowserSession(w, r, app.TenantID, user.ID); err != nil {
		h.audit(r, app.TenantID, user.ID, "CHANNEL_AUTH_FAILED", err.Error(), http.StatusInternalServerError)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	h.audit(r, app.TenantID, user.ID, "CHANNEL_BINDING_CREATED", "platform="+platform, http.StatusOK)
	h.audit(r, app.TenantID, user.ID, "CHANNEL_LOGIN_SUCCEEDED", "platform="+platform, http.StatusFound)
	http.Redirect(w, r, sanitizeReturnTo(challengeValue.ReturnTo), http.StatusFound)
}

func (h *ChannelAuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil || ac.SessionID == "" {
		http.Error(w, "channel session required", http.StatusUnauthorized)
		return
	}
	if err := h.sessions.Revoke(r.Context(), ac.SessionID); err != nil {
		http.Error(w, "logout failed", http.StatusInternalServerError)
		return
	}
	clearCookie(w, auth.ChannelSessionCookie, true, h.cookieSecure)
	clearCookie(w, auth.ChannelCSRFCookie, false, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

func (h *ChannelAuthHandler) ServeBindingsHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listUserBindings(w, r)
	case http.MethodDelete:
		id := strings.TrimPrefix(r.URL.Path, "/v1/channel-bindings/")
		if id == "" || id == r.URL.Path {
			http.Error(w, "binding id required", http.StatusBadRequest)
			return
		}
		h.deleteUserBinding(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ChannelAuthHandler) listUserBindings(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil || ac.UserID == "" {
		http.Error(w, "user identity required", http.StatusBadRequest)
		return
	}
	items, err := h.identities.ListByUser(r.Context(), ac.TenantID, ac.UserID)
	if err != nil {
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"bindings": items, "count": len(items)})
}

func (h *ChannelAuthHandler) deleteUserBinding(w http.ResponseWriter, r *http.Request, id string) {
	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil || ac.UserID == "" {
		http.Error(w, "user identity required", http.StatusBadRequest)
		return
	}
	identity, err := h.identities.GetByID(r.Context(), ac.TenantID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "binding not found", http.StatusNotFound)
			return
		}
		http.Error(w, "lookup failed", http.StatusInternalServerError)
		return
	}
	if identity.UserID != ac.UserID && !ac.HasRole("admin") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := h.identities.Revoke(r.Context(), ac.TenantID, id); err != nil {
		http.Error(w, "revoke failed", http.StatusInternalServerError)
		return
	}
	h.audit(r, ac.TenantID, ac.UserID, "CHANNEL_BINDING_REVOKED", "binding_id="+id, http.StatusNoContent)
	w.WriteHeader(http.StatusNoContent)
}

func (h *ChannelAuthHandler) createBrowserSession(w http.ResponseWriter, r *http.Request, tenantID, userID string) error {
	rawSession, err := channel.RandomToken("hs_", 32)
	if err != nil {
		return err
	}
	rawCSRF, err := channel.RandomToken("csrf_", 32)
	if err != nil {
		return err
	}
	session := &store.BrowserSession{
		TenantID:      tenantID,
		UserID:        userID,
		TokenHash:     auth.HashKey(rawSession),
		CSRFTokenHash: auth.HashKey(rawCSRF),
		UserAgent:     r.UserAgent(),
		SourceIP:      r.RemoteAddr,
		ExpiresAt:     time.Now().Add(channelSessionTTL),
	}
	if err := h.sessions.Create(r.Context(), session); err != nil {
		return err
	}
	setCookie(w, auth.ChannelSessionCookie, rawSession, true, h.cookieSecure, channelSessionTTL)
	setCookie(w, auth.ChannelCSRFCookie, rawCSRF, false, h.cookieSecure, channelSessionTTL)
	return nil
}

func (h *ChannelAuthHandler) callbackURL(r *http.Request, platform string) string {
	base := h.publicURL
	if base == "" {
		scheme := r.Header.Get("X-Forwarded-Proto")
		if scheme == "" {
			scheme = "http"
			if r.TLS != nil {
				scheme = "https"
			}
		}
		host := r.Header.Get("X-Forwarded-Host")
		if host == "" {
			host = r.Host
		}
		base = scheme + "://" + host
	}
	return base + "/auth/channel/" + url.PathEscape(platform) + "/callback"
}

func (h *ChannelAuthHandler) audit(r *http.Request, tenantID, userID, action, detail string, status int) {
	if h.store == nil || h.store.AuditLogs() == nil {
		return
	}
	_ = h.store.AuditLogs().Append(r.Context(), &store.AuditLog{
		TenantID:   tenantID,
		UserID:     userID,
		Action:     action,
		Detail:     detail,
		RequestID:  "",
		StatusCode: status,
		SourceIP:   r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	})
}

func sanitizeReturnTo(v string) string {
	if v == "" {
		return "/"
	}
	if strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//") && !strings.Contains(v, "\n") && !strings.Contains(v, "\r") {
		return v
	}
	return "/"
}

func setCookie(w http.ResponseWriter, name, value string, httpOnly, secure bool, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: httpOnly,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(w http.ResponseWriter, name string, httpOnly, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: httpOnly,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func channelNotConfigured(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "channel login is not configured", http.StatusServiceUnavailable)
}
