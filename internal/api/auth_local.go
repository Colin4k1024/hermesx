package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/channel"
	"github.com/Colin4k1024/hermesx/internal/skills"
	"github.com/Colin4k1024/hermesx/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// LocalAuthHandler handles username/password self-registration and login.
type LocalAuthHandler struct {
	store        store.Store
	sessions     store.BrowserSessionStore
	provisioner  *skills.Provisioner
	hashSecret   string
	cookieSecure bool
}

// NewLocalAuthHandler creates a handler for local (username/password) authentication.
func NewLocalAuthHandler(s store.Store, sessions store.BrowserSessionStore, provisioner *skills.Provisioner, hashSecret string, cookieSecure bool) *LocalAuthHandler {
	return &LocalAuthHandler{
		store:        s,
		sessions:     sessions,
		provisioner:  provisioner,
		hashSecret:   hashSecret,
		cookieSecure: cookieSecure,
	}
}

type registerReq struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	DisplayName   string `json:"display_name"`
	TenantID      string `json:"tenant_id,omitempty"`
	NewTenantName string `json:"new_tenant_name,omitempty"`
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TenantID string `json:"tenant_id"`
}

type authResp struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
}

// Register handles POST /auth/register — creates a new user with username/password.
func (h *LocalAuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registerReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.TenantID = strings.TrimSpace(req.TenantID)
	req.NewTenantName = strings.TrimSpace(req.NewTenantName)

	// Validate input.
	if len(req.Username) < 3 || len(req.Username) > 64 {
		http.Error(w, "username must be 3-64 characters", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 || len(req.Password) > 128 {
		http.Error(w, "password must be 8-128 characters", http.StatusBadRequest)
		return
	}
	if req.TenantID == "" && req.NewTenantName == "" {
		http.Error(w, "tenant_id or new_tenant_name is required", http.StatusBadRequest)
		return
	}
	if req.TenantID != "" && req.NewTenantName != "" {
		http.Error(w, "specify either tenant_id or new_tenant_name, not both", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	tenantID := req.TenantID

	// Create new tenant if requested.
	if req.NewTenantName != "" {
		if len(req.NewTenantName) < 2 || len(req.NewTenantName) > 128 {
			http.Error(w, "tenant name must be 2-128 characters", http.StatusBadRequest)
			return
		}
		newTenant := &store.Tenant{
			Name:   req.NewTenantName,
			Plan:   "free",
			MaxSessions: 100,
			RateLimitRPM: 60,
		}
		if err := h.store.Tenants().Create(ctx, newTenant); err != nil {
			slog.Error("create tenant failed", "error", err)
			http.Error(w, "failed to create tenant", http.StatusInternalServerError)
			return
		}
		tenantID = newTenant.ID

		// Provision tenant soul + bundled skills.
		if h.provisioner != nil {
			go func() {
				bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				if err := h.provisioner.Provision(bgCtx, tenantID); err != nil {
					slog.Error("tenant provisioning failed", "tenant", tenantID, "error", err)
				}
			}()
		}
	} else {
		// Verify tenant exists.
		if _, err := h.store.Tenants().Get(ctx, tenantID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.Error(w, "tenant not found", http.StatusNotFound)
				return
			}
			http.Error(w, "failed to verify tenant", http.StatusInternalServerError)
			return
		}
	}

	// Check username uniqueness in tenant.
	if _, _, err := h.store.Users().GetByUsername(ctx, tenantID, req.Username); err == nil {
		http.Error(w, "username already taken in this tenant", http.StatusConflict)
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		slog.Error("check username failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Hash password.
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		slog.Error("bcrypt failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Create user.
	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Username
	}
	user := &store.User{
		TenantID:    tenantID,
		Username:    req.Username,
		DisplayName: displayName,
	}
	if err := h.store.Users().CreateWithPassword(ctx, user, string(hash)); err != nil {
		slog.Error("create user failed", "error", err)
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	// Provision user skills in background.
	if h.provisioner != nil {
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := h.provisioner.ProvisionUserSkills(bgCtx, tenantID, user.ID); err != nil {
				slog.Error("user skill provisioning failed", "tenant", tenantID, "user", user.ID, "error", err)
			}
		}()
	}

	// Create browser session.
	if err := h.createBrowserSession(w, r, tenantID, user.ID); err != nil {
		slog.Error("create session failed", "error", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	h.audit(r, tenantID, user.ID, "LOCAL_REGISTER", fmt.Sprintf("username=%s", req.Username), http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(authResp{UserID: user.ID, TenantID: tenantID})
}

// Login handles POST /auth/login — authenticates with username/password.
func (h *LocalAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.TenantID = strings.TrimSpace(req.TenantID)

	if req.Username == "" || req.Password == "" || req.TenantID == "" {
		http.Error(w, "username, password, and tenant_id are required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Look up user.
	user, passwordHash, err := h.store.Users().GetByUsername(ctx, req.TenantID, req.Username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.audit(r, req.TenantID, "", "LOCAL_LOGIN_FAILED", "user not found", http.StatusUnauthorized)
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		slog.Error("get user by username failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Verify password.
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		h.audit(r, req.TenantID, user.ID, "LOCAL_LOGIN_FAILED", "wrong password", http.StatusUnauthorized)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Create browser session.
	if err := h.createBrowserSession(w, r, req.TenantID, user.ID); err != nil {
		slog.Error("create session failed", "error", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	h.audit(r, req.TenantID, user.ID, "LOCAL_LOGIN", fmt.Sprintf("username=%s", req.Username), http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(authResp{UserID: user.ID, TenantID: req.TenantID})
}

// ListTenants handles GET /auth/tenants — returns available tenants for registration.
func (h *LocalAuthHandler) ListTenants(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	tenants, _, err := h.store.Tenants().List(ctx, store.ListOptions{Limit: 100})
	if err != nil {
		slog.Error("list tenants failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	type tenantItem struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	items := make([]tenantItem, 0, len(tenants))
	for _, t := range tenants {
		items = append(items, tenantItem{ID: t.ID, Name: t.Name})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"tenants": items})
}

// createBrowserSession creates a new browser session and sets hx_session + hx_csrf cookies.
func (h *LocalAuthHandler) createBrowserSession(w http.ResponseWriter, r *http.Request, tenantID, userID string) error {
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

func (h *LocalAuthHandler) audit(r *http.Request, tenantID, userID, action, detail string, status int) {
	if h.store == nil || h.store.AuditLogs() == nil {
		return
	}
	_ = h.store.AuditLogs().Append(r.Context(), &store.AuditLog{
		TenantID:   tenantID,
		UserID:     userID,
		Action:     action,
		Detail:     detail,
		StatusCode: status,
		SourceIP:   r.RemoteAddr,
		UserAgent:  r.UserAgent(),
	})
}
