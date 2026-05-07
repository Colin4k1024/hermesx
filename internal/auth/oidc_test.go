package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
)

func setupMockIdP(t *testing.T) (*httptest.Server, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	jwk := jose.JSONWebKey{Key: &key.PublicKey, KeyID: "test-key-1", Algorithm: "RS256", Use: "sig"}
	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}

	mux := http.NewServeMux()
	var issuer string

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                issuer,
			"jwks_uri":                              issuer + "/.well-known/jwks.json",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	})

	srv := httptest.NewServer(mux)
	issuer = srv.URL
	return srv, key
}

func signToken(t *testing.T, key *rsa.PrivateKey, issuer, audience, sub string, extra map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key-1"))
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	claims := josejwt.Claims{
		Issuer:    issuer,
		Subject:   sub,
		Audience:  josejwt.Audience{audience},
		IssuedAt:  josejwt.NewNumericDate(now),
		Expiry:    josejwt.NewNumericDate(now.Add(10 * time.Minute)),
		NotBefore: josejwt.NewNumericDate(now.Add(-1 * time.Minute)),
	}

	token, err := josejwt.Signed(signer).Claims(claims).Claims(extra).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestOIDCExtractor_ValidToken(t *testing.T) {
	srv, key := setupMockIdP(t)
	defer srv.Close()

	clientID := "hermes-test"
	extractor, err := NewOIDCExtractor(context.Background(), OIDCConfig{
		IssuerURL: srv.URL,
		ClientID:  clientID,
	})
	if err != nil {
		t.Fatal(err)
	}

	token := signToken(t, key, srv.URL, clientID, "user-123", map[string]any{
		"tenant_id": "tenant-abc",
		"roles":     []string{"admin", "user"},
		"acr":       "urn:mace:incommon:iap:silver",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	ac, err := extractor.Extract(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ac == nil {
		t.Fatal("expected AuthContext, got nil")
	}
	if ac.UserID != "user-123" {
		t.Errorf("UserID = %q, want user-123", ac.UserID)
	}
	if ac.TenantID != "tenant-abc" {
		t.Errorf("TenantID = %q, want tenant-abc", ac.TenantID)
	}
	if ac.AuthMethod != "oidc" {
		t.Errorf("AuthMethod = %q, want oidc", ac.AuthMethod)
	}
	if ac.ACRLevel != "urn:mace:incommon:iap:silver" {
		t.Errorf("ACRLevel = %q, want urn:mace:incommon:iap:silver", ac.ACRLevel)
	}
	if len(ac.Roles) != 2 || ac.Roles[0] != "admin" {
		t.Errorf("Roles = %v, want [admin user]", ac.Roles)
	}
}

func TestOIDCExtractor_NoBearer(t *testing.T) {
	srv, _ := setupMockIdP(t)
	defer srv.Close()

	extractor, _ := NewOIDCExtractor(context.Background(), OIDCConfig{
		IssuerURL: srv.URL,
		ClientID:  "hermes-test",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ac, err := extractor.Extract(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ac != nil {
		t.Error("expected nil for request without Bearer")
	}
}

func TestOIDCExtractor_InvalidToken(t *testing.T) {
	srv, _ := setupMockIdP(t)
	defer srv.Close()

	extractor, _ := NewOIDCExtractor(context.Background(), OIDCConfig{
		IssuerURL: srv.URL,
		ClientID:  "hermes-test",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid.jwt.token")
	ac, err := extractor.Extract(req)
	if err == nil {
		t.Fatal("expected error for invalid JWT token")
	}
	if ac != nil {
		t.Error("expected nil AuthContext for invalid token")
	}
}

func TestOIDCExtractor_CustomClaimMapper(t *testing.T) {
	srv, key := setupMockIdP(t)
	defer srv.Close()

	clientID := "hermes-test"
	extractor, err := NewOIDCExtractor(context.Background(), OIDCConfig{
		IssuerURL: srv.URL,
		ClientID:  clientID,
		ClaimMapper: &ClaimMapper{
			TenantClaim: "org_id",
			RolesClaim:  "permissions",
			ACRClaim:    "auth_level",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	token := signToken(t, key, srv.URL, clientID, "user-456", map[string]any{
		"org_id":      "org-xyz",
		"permissions": []string{"operator"},
		"auth_level":  "mfa",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	ac, err := extractor.Extract(req)
	if err != nil {
		t.Fatal(err)
	}
	if ac == nil {
		t.Fatal("expected AuthContext")
	}
	if ac.TenantID != "org-xyz" {
		t.Errorf("TenantID = %q, want org-xyz", ac.TenantID)
	}
	if ac.ACRLevel != "mfa" {
		t.Errorf("ACRLevel = %q, want mfa", ac.ACRLevel)
	}
	if len(ac.Roles) != 1 || ac.Roles[0] != "operator" {
		t.Errorf("Roles = %v, want [operator]", ac.Roles)
	}
}

func TestOIDCExtractor_DefaultRoles(t *testing.T) {
	srv, key := setupMockIdP(t)
	defer srv.Close()

	clientID := "hermes-test"
	extractor, _ := NewOIDCExtractor(context.Background(), OIDCConfig{
		IssuerURL: srv.URL,
		ClientID:  clientID,
	})

	token := signToken(t, key, srv.URL, clientID, "user-789", map[string]any{
		"tenant_id": "t1",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	ac, _ := extractor.Extract(req)
	if ac == nil {
		t.Fatal("expected AuthContext")
	}
	if len(ac.Roles) != 1 || ac.Roles[0] != "user" {
		t.Errorf("Roles = %v, want [user] (default)", ac.Roles)
	}
}

func TestOIDCExtractor_WrongAudience(t *testing.T) {
	srv, key := setupMockIdP(t)
	defer srv.Close()

	extractor, _ := NewOIDCExtractor(context.Background(), OIDCConfig{
		IssuerURL: srv.URL,
		ClientID:  "hermes-test",
	})

	token := signToken(t, key, srv.URL, "wrong-audience", "user-x", map[string]any{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	ac, err := extractor.Extract(req)
	if err == nil {
		t.Fatal("expected error for wrong audience token")
	}
	if ac != nil {
		t.Error("expected nil AuthContext for wrong audience")
	}
}

func TestOIDCExtractor_ExpiredToken(t *testing.T) {
	srv, key := setupMockIdP(t)
	defer srv.Close()

	clientID := "hermes-test"
	extractor, _ := NewOIDCExtractor(context.Background(), OIDCConfig{
		IssuerURL: srv.URL,
		ClientID:  clientID,
	})

	// Create an expired token
	signer, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key-1"))
	past := time.Now().Add(-1 * time.Hour)
	claims := josejwt.Claims{
		Issuer:   srv.URL,
		Subject:  "user-expired",
		Audience: josejwt.Audience{clientID},
		IssuedAt: josejwt.NewNumericDate(past),
		Expiry:   josejwt.NewNumericDate(past.Add(5 * time.Minute)),
	}
	token, _ := josejwt.Signed(signer).Claims(claims).Serialize()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	ac, _ := extractor.Extract(req)
	if ac != nil {
		t.Error("expected nil for expired token")
	}
}

func TestOIDCExtractor_MissingTenantClaim(t *testing.T) {
	srv, key := setupMockIdP(t)
	defer srv.Close()

	clientID := "hermes-test"
	extractor, _ := NewOIDCExtractor(context.Background(), OIDCConfig{
		IssuerURL: srv.URL,
		ClientID:  clientID,
	})

	// Token with no tenant_id claim.
	token := signToken(t, key, srv.URL, clientID, "user-no-tenant", map[string]any{
		"roles": []string{"user"},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	ac, err := extractor.Extract(req)
	if err == nil {
		t.Fatal("expected error for token missing tenant_id claim")
	}
	if ac != nil {
		t.Error("expected nil AuthContext for missing tenant claim")
	}
}
