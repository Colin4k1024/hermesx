package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hermes-agent/hermes-agent-go/internal/store"
)

// APIKeyExtractor validates Bearer tokens against hashed API keys in the store.
type APIKeyExtractor struct {
	keys store.APIKeyStore
}

func NewAPIKeyExtractor(keys store.APIKeyStore) *APIKeyExtractor {
	return &APIKeyExtractor{keys: keys}
}

func (a *APIKeyExtractor) Extract(r *http.Request) (*AuthContext, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return nil, nil
	}

	provided := strings.TrimPrefix(auth, "Bearer ")
	hash := hashKey(provided)

	key, err := a.keys.GetByHash(r.Context(), hash)
	if err != nil {
		return nil, nil // not found = not this extractor's concern
	}
	if key == nil {
		return nil, nil
	}
	if key.RevokedAt != nil {
		return nil, fmt.Errorf("api key revoked")
	}
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("api key expired")
	}

	return &AuthContext{
		Identity:   key.ID,
		TenantID:   key.TenantID,
		Roles:      key.Roles,
		Scopes:     key.Scopes,
		AuthMethod: "api_key",
	}, nil
}

// HashKey produces SHA-256 hex digest of a raw API key.
func HashKey(raw string) string { return hashKey(raw) }

func hashKey(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
