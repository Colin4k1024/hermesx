package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/store"
)

type createAdminKeyRequest struct {
	Name   string   `json:"name"`
	Roles  []string `json:"roles,omitempty"`
	Scopes []string `json:"scopes,omitempty"`
}

type createAdminKeyResponse struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	Prefix    string    `json:"prefix"`
	RawKey    string    `json:"key"` // one-time; never stored
	Roles     []string  `json:"roles"`
	Scopes    []string  `json:"scopes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *AdminHandler) createAPIKey(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		http.Error(w, "tenant id required", http.StatusBadRequest)
		return
	}

	// Verify tenant exists.
	tenant, err := h.store.Tenants().Get(r.Context(), tenantID)
	if err != nil || tenant == nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}

	var req createAdminKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	roles := req.Roles
	if len(roles) == 0 {
		roles = []string{"user"}
	}

	rawKey := generateAdminRawKey()
	prefix := rawKey[:8]

	key := &store.APIKey{
		TenantID: tenantID,
		Name:     req.Name,
		KeyHash:  auth.HashKey(rawKey),
		Prefix:   prefix,
		Roles:    roles,
		Scopes:   req.Scopes,
	}

	if err := h.store.APIKeys().Create(r.Context(), key); err != nil {
		h.logger.Error("failed to create api key", "tenant_id", tenantID, "error", err)
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	h.logger.Info("admin api key created", "tenant_id", tenantID, "key_id", key.ID, "name", req.Name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createAdminKeyResponse{
		ID:        key.ID,
		TenantID:  tenantID,
		Name:      key.Name,
		Prefix:    prefix,
		RawKey:    rawKey,
		Roles:     roles,
		Scopes:    req.Scopes,
		CreatedAt: key.CreatedAt,
	})
}

type apiKeyItem struct {
	ID        string     `json:"id"`
	TenantID  string     `json:"tenant_id"`
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	Roles     []string   `json:"roles"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func (h *AdminHandler) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	if tenantID == "" {
		http.Error(w, "tenant id required", http.StatusBadRequest)
		return
	}

	keys, err := h.store.APIKeys().List(r.Context(), tenantID)
	if err != nil {
		h.logger.Error("failed to list api keys", "tenant_id", tenantID, "error", err)
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}

	items := make([]apiKeyItem, 0, len(keys))
	for _, k := range keys {
		items = append(items, apiKeyItem{
			ID:        k.ID,
			TenantID:  k.TenantID,
			Name:      k.Name,
			Prefix:    k.Prefix,
			Roles:     k.Roles,
			Scopes:    k.Scopes,
			ExpiresAt: k.ExpiresAt,
			RevokedAt: k.RevokedAt,
			CreatedAt: k.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"api_keys": items,
		"total":    len(items),
	})
}

func (h *AdminHandler) rotateAPIKey(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	kid := r.PathValue("kid")
	if tenantID == "" || kid == "" {
		http.Error(w, "tenant id and key id required", http.StatusBadRequest)
		return
	}

	// Verify tenant exists.
	tenant, err := h.store.Tenants().Get(r.Context(), tenantID)
	if err != nil || tenant == nil {
		http.Error(w, "tenant not found", http.StatusNotFound)
		return
	}

	// Get existing key to inherit its name, roles, and scopes.
	oldKey, err := h.store.APIKeys().GetByID(r.Context(), tenantID, kid)
	if err != nil || oldKey == nil {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}
	if oldKey.RevokedAt != nil {
		http.Error(w, "cannot rotate a revoked key", http.StatusConflict)
		return
	}

	// Atomic rotation: create new + revoke old in a single transaction.
	rawKey := generateAdminRawKey()
	prefix := rawKey[:8]

	newKey := &store.APIKey{
		TenantID: tenantID,
		Name:     oldKey.Name,
		KeyHash:  auth.HashKey(rawKey),
		Prefix:   prefix,
		Roles:    oldKey.Roles,
		Scopes:   oldKey.Scopes,
	}

	if h.pool != nil {
		if err := h.rotateKeyAtomic(r.Context(), newKey, tenantID, kid); err != nil {
			h.logger.Error("failed to rotate key atomically", "tenant_id", tenantID, "error", err)
			http.Error(w, "rotation failed", http.StatusInternalServerError)
			return
		}
	} else {
		if err := h.store.APIKeys().Create(r.Context(), newKey); err != nil {
			h.logger.Error("failed to create rotated key", "tenant_id", tenantID, "error", err)
			http.Error(w, "rotation failed: create new key", http.StatusInternalServerError)
			return
		}
		if err := h.store.APIKeys().Revoke(r.Context(), tenantID, kid); err != nil {
			h.logger.Error("failed to revoke old key during rotation", "tenant_id", tenantID, "old_key_id", kid, "error", err)
			http.Error(w, "rotation failed: revoke old key", http.StatusInternalServerError)
			return
		}
	}

	h.logger.Info("api key rotated", "tenant_id", tenantID, "old_key_id", kid, "new_key_id", newKey.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"new_key": createAdminKeyResponse{
			ID:        newKey.ID,
			TenantID:  tenantID,
			Name:      newKey.Name,
			Prefix:    prefix,
			RawKey:    rawKey,
			Roles:     newKey.Roles,
			Scopes:    newKey.Scopes,
			CreatedAt: newKey.CreatedAt,
		},
		"revoked_key_id": kid,
		"message":        "Key rotated. Save the new key now; it will not be shown again.",
	})
}

func (h *AdminHandler) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	kid := r.PathValue("kid")
	if tenantID == "" || kid == "" {
		http.Error(w, "tenant id and key id required", http.StatusBadRequest)
		return
	}

	// Verify key belongs to tenant.
	existing, err := h.store.APIKeys().GetByID(r.Context(), tenantID, kid)
	if err != nil || existing == nil {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}
	if existing.RevokedAt != nil {
		http.Error(w, "key already revoked", http.StatusConflict)
		return
	}

	if err := h.store.APIKeys().Revoke(r.Context(), tenantID, kid); err != nil {
		h.logger.Error("failed to revoke api key", "tenant_id", tenantID, "key_id", kid, "error", err)
		http.Error(w, "revoke failed", http.StatusInternalServerError)
		return
	}

	h.logger.Info("api key revoked", "tenant_id", tenantID, "key_id", kid)
	w.WriteHeader(http.StatusNoContent)
}

// rotateKeyAtomic creates a new key and revokes the old one in a single transaction.
func (h *AdminHandler) rotateKeyAtomic(ctx context.Context, newKey *store.APIKey, tenantID, oldKeyID string) error {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", tenantID); err != nil {
		return fmt.Errorf("set tenant context: %w", err)
	}

	now := time.Now()
	var newID string
	err = tx.QueryRow(ctx,
		`INSERT INTO api_keys (tenant_id, name, key_hash, prefix, roles, scopes, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		newKey.TenantID, newKey.Name, newKey.KeyHash, newKey.Prefix,
		newKey.Roles, newKey.Scopes, now,
	).Scan(&newID)
	if err != nil {
		return fmt.Errorf("insert new key: %w", err)
	}
	newKey.ID = newID
	newKey.CreatedAt = now

	_, err = tx.Exec(ctx,
		`UPDATE api_keys SET revoked_at = $1 WHERE tenant_id = $2 AND id = $3`,
		now, tenantID, oldKeyID,
	)
	if err != nil {
		return fmt.Errorf("revoke old key: %w", err)
	}

	return tx.Commit(ctx)
}

// generateAdminRawKey produces a 32-byte random key with "hk_" prefix encoded as hex.
func generateAdminRawKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return "hk_" + hex.EncodeToString(b)
}
