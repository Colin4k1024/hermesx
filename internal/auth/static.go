package auth

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
)

// StaticTokenExtractor validates Bearer tokens against a fixed secret.
// Used for single-tenant / dev mode backward compatibility.
type StaticTokenExtractor struct {
	token    string
	tenantID string
}

func NewStaticTokenExtractor(token, tenantID string) *StaticTokenExtractor {
	return &StaticTokenExtractor{token: token, tenantID: tenantID}
}

func (s *StaticTokenExtractor) Extract(r *http.Request) (*AuthContext, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return nil, nil
	}

	provided := strings.TrimPrefix(auth, "Bearer ")
	if subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) != 1 {
		return nil, fmt.Errorf("invalid static token")
	}

	return &AuthContext{
		Identity:   "static-user",
		TenantID:   s.tenantID,
		Roles:      []string{"admin"},
		AuthMethod: "static_token",
	}, nil
}
