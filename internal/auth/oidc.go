package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// OIDCConfig configures the OIDC extractor.
type OIDCConfig struct {
	IssuerURL string
	ClientID  string
	// ClaimMapper overrides default claim paths for tenant_id, roles, acr.
	ClaimMapper *ClaimMapper
}

// ClaimMapper maps IdP-specific claim names to internal fields.
type ClaimMapper struct {
	TenantClaim string // default: "tenant_id"
	RolesClaim  string // default: "roles"
	ACRClaim    string // default: "acr"
}

func (cm *ClaimMapper) tenantClaim() string {
	if cm != nil && cm.TenantClaim != "" {
		return cm.TenantClaim
	}
	return "tenant_id"
}

func (cm *ClaimMapper) rolesClaim() string {
	if cm != nil && cm.RolesClaim != "" {
		return cm.RolesClaim
	}
	return "roles"
}

func (cm *ClaimMapper) acrClaim() string {
	if cm != nil && cm.ACRClaim != "" {
		return cm.ACRClaim
	}
	return "acr"
}

// OIDCExtractor validates OIDC ID tokens using JWKS rotation.
type OIDCExtractor struct {
	verifier    *oidc.IDTokenVerifier
	claimMapper *ClaimMapper
}

// NewOIDCExtractor creates an OIDC extractor. ctx is used for JWKS discovery.
func NewOIDCExtractor(ctx context.Context, cfg OIDCConfig) (*OIDCExtractor, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery for %s: %w", cfg.IssuerURL, err)
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.ClientID,
	})

	return &OIDCExtractor{
		verifier:    verifier,
		claimMapper: cfg.ClaimMapper,
	}, nil
}

// NewOIDCExtractorWithVerifier allows injecting a verifier (for testing).
func NewOIDCExtractorWithVerifier(verifier *oidc.IDTokenVerifier, mapper *ClaimMapper) *OIDCExtractor {
	return &OIDCExtractor{verifier: verifier, claimMapper: mapper}
}

func (o *OIDCExtractor) Extract(r *http.Request) (*AuthContext, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return nil, nil
	}
	tokenStr := strings.TrimPrefix(auth, "Bearer ")

	// Only attempt OIDC verification for JWT-shaped tokens (three dot-separated parts).
	if strings.Count(tokenStr, ".") != 2 {
		return nil, nil
	}

	idToken, err := o.verifier.Verify(r.Context(), tokenStr)
	if err != nil {
		return nil, fmt.Errorf("oidc token verification failed: %w", err)
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oidc token claims decode failed: %w", err)
	}

	sub := idToken.Subject
	tenantID, _ := claims[o.claimMapper.tenantClaim()].(string)
	if tenantID == "" {
		return nil, fmt.Errorf("oidc token missing required %q claim", o.claimMapper.tenantClaim())
	}
	acr, _ := claims[o.claimMapper.acrClaim()].(string)

	var roles []string
	switch v := claims[o.claimMapper.rolesClaim()].(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				roles = append(roles, s)
			}
		}
	case string:
		roles = strings.Split(v, ",")
	}
	if len(roles) == 0 {
		roles = []string{"user"}
	}

	return &AuthContext{
		Identity:   sub,
		UserID:     sub,
		TenantID:   tenantID,
		Roles:      roles,
		AuthMethod: "oidc",
		ACRLevel:   acr,
	}, nil
}
