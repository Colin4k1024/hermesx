package admin

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/store"
)

const defaultTenantID = "00000000-0000-0000-0000-000000000001"

// BootstrapHandler handles the one-time admin bootstrap flow.
// GET  /admin/v1/bootstrap/status — public, no auth.
// POST /admin/v1/bootstrap        — ACP token auth only.
type BootstrapHandler struct {
	store    store.Store
	acpToken string
	logger   *slog.Logger
	mu       sync.Mutex // serializes check-then-create to prevent TOCTOU race
}

// NewBootstrapHandler creates a BootstrapHandler. The ACP token is read from
// the HERMES_ACP_TOKEN environment variable at construction time.
func NewBootstrapHandler(s store.Store, logger *slog.Logger) *BootstrapHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &BootstrapHandler{
		store:    s,
		acpToken: os.Getenv("HERMES_ACP_TOKEN"),
		logger:   logger,
	}
}

// hasAdminKey returns true if defaultTenantID has at least one non-revoked key
// with the "admin" role.
func (h *BootstrapHandler) hasAdminKey(r *http.Request) (bool, error) {
	keys, err := h.store.APIKeys().List(r.Context(), defaultTenantID)
	if err != nil {
		return false, err
	}
	for _, k := range keys {
		if k.RevokedAt != nil {
			continue
		}
		for _, role := range k.Roles {
			if role == "admin" {
				return true, nil
			}
		}
	}
	return false, nil
}

// Status handles GET /admin/v1/bootstrap/status (public).
func (h *BootstrapHandler) Status(w http.ResponseWriter, r *http.Request) {
	found, err := h.hasAdminKey(r)
	if err != nil {
		h.logger.Error("bootstrap status check failed", "error", err)
		http.Error(w, "status check failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"bootstrap_required": !found,
	})
}

type bootstrapRequest struct {
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Create handles POST /admin/v1/bootstrap (ACP token auth).
func (h *BootstrapHandler) Create(w http.ResponseWriter, r *http.Request) {
	// ACP token auth — constant-time comparison prevents timing attacks.
	authHeader := r.Header.Get("Authorization")
	expected := []byte("Bearer " + h.acpToken)
	actual := []byte(authHeader)
	if h.acpToken == "" || subtle.ConstantTimeCompare(actual, expected) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Serialize check-then-create to prevent TOCTOU race under concurrent requests.
	h.mu.Lock()
	defer h.mu.Unlock()

	// Guard: no second bootstrap (re-checked inside lock).
	found, err := h.hasAdminKey(r)
	if err != nil {
		h.logger.Error("bootstrap create: admin key check failed", "error", err)
		http.Error(w, "check failed", http.StatusInternalServerError)
		return
	}
	if found {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "bootstrap already completed"})
		return
	}

	var req bootstrapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		req.Name = "initial-admin-key"
	}

	rawKey := generateAdminRawKey()
	prefix := rawKey[:8]

	key := &store.APIKey{
		TenantID:  defaultTenantID,
		Name:      req.Name,
		KeyHash:   auth.HashKey(rawKey),
		Prefix:    prefix,
		Roles:     []string{"admin"},
		Scopes:    []string{"admin", "chat", "read"},
		ExpiresAt: req.ExpiresAt,
	}

	if creator, ok := h.store.(store.BootstrapAdminKeyCreator); ok {
		created, err := creator.CreateBootstrapAdminKey(r.Context(), key)
		if err != nil {
			h.logger.Error("bootstrap create api key failed", "error", err)
			http.Error(w, "create failed", http.StatusInternalServerError)
			return
		}
		if !created {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "bootstrap already completed"})
			return
		}
	} else {
		if err := h.store.APIKeys().Create(r.Context(), key); err != nil {
			h.logger.Error("bootstrap create api key failed", "error", err)
			http.Error(w, "create failed", http.StatusInternalServerError)
			return
		}
	}

	h.logger.Info("bootstrap admin key created", "key_id", key.ID, "name", key.Name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"id":         key.ID,
		"key":        rawKey,
		"name":       key.Name,
		"tenant_id":  key.TenantID,
		"roles":      key.Roles,
		"created_at": key.CreatedAt,
	})
}
