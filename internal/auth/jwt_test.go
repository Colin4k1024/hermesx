package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func signJWT(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	s, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestJWTExtractor(t *testing.T) {
	key := generateTestKey(t)
	wrongKey := generateTestKey(t)
	const issuer = "hermesx-test"

	extractor := NewJWTExtractor(&key.PublicKey, issuer)

	tests := []struct {
		name       string
		authHeader func(t *testing.T) string
		wantNil    bool
		wantID     string
		wantTenant string
		wantRoles  []string
		wantMethod string
	}{
		{
			name: "valid token with all claims",
			authHeader: func(t *testing.T) string {
				return "Bearer " + signJWT(t, key, jwt.MapClaims{
					"sub":       "user-42",
					"iss":       issuer,
					"exp":       jwt.NewNumericDate(time.Now().Add(time.Hour)),
					"tenant_id": "tenant-x",
					"roles":     []any{"admin", "operator"},
				})
			},
			wantNil:    false,
			wantID:     "user-42",
			wantTenant: "tenant-x",
			wantRoles:  []string{"admin", "operator"},
			wantMethod: "jwt",
		},
		{
			name: "expired token",
			authHeader: func(t *testing.T) string {
				return "Bearer " + signJWT(t, key, jwt.MapClaims{
					"sub": "user-42",
					"iss": issuer,
					"exp": jwt.NewNumericDate(time.Now().Add(-time.Hour)),
				})
			},
			wantNil: true,
		},
		{
			name: "wrong issuer",
			authHeader: func(t *testing.T) string {
				return "Bearer " + signJWT(t, key, jwt.MapClaims{
					"sub": "user-42",
					"iss": "wrong-issuer",
					"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
				})
			},
			wantNil: true,
		},
		{
			name: "missing exp claim",
			authHeader: func(t *testing.T) string {
				return "Bearer " + signJWT(t, key, jwt.MapClaims{
					"sub": "user-42",
					"iss": issuer,
				})
			},
			wantNil: true,
		},
		{
			name: "no roles claim defaults to user",
			authHeader: func(t *testing.T) string {
				return "Bearer " + signJWT(t, key, jwt.MapClaims{
					"sub":       "user-42",
					"iss":       issuer,
					"exp":       jwt.NewNumericDate(time.Now().Add(time.Hour)),
					"tenant_id": "tenant-y",
				})
			},
			wantNil:    false,
			wantID:     "user-42",
			wantTenant: "tenant-y",
			wantRoles:  []string{"user"},
			wantMethod: "jwt",
		},
		{
			name: "wrong signing key",
			authHeader: func(t *testing.T) string {
				return "Bearer " + signJWT(t, wrongKey, jwt.MapClaims{
					"sub": "user-42",
					"iss": issuer,
					"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
				})
			},
			wantNil: true,
		},
		{
			name: "wrong signing method HMAC",
			authHeader: func(t *testing.T) string {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"sub": "user-42",
					"iss": issuer,
					"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
				})
				s, err := token.SignedString([]byte("hmac-secret"))
				if err != nil {
					t.Fatal(err)
				}
				return "Bearer " + s
			},
			wantNil: true,
		},
		{
			name: "missing authorization header",
			authHeader: func(_ *testing.T) string {
				return ""
			},
			wantNil: true,
		},
		{
			name: "malformed token string",
			authHeader: func(_ *testing.T) string {
				return "Bearer not.a.valid.jwt.token"
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			header := tt.authHeader(t)
			if header != "" {
				req.Header.Set("Authorization", header)
			}

			ac, err := extractor.Extract(req)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantNil {
				if ac != nil {
					t.Fatalf("expected nil AuthContext, got %+v", ac)
				}
				return
			}

			if ac == nil {
				t.Fatal("expected non-nil AuthContext, got nil")
			}
			if ac.Identity != tt.wantID {
				t.Errorf("Identity = %q, want %q", ac.Identity, tt.wantID)
			}
			if ac.TenantID != tt.wantTenant {
				t.Errorf("TenantID = %q, want %q", ac.TenantID, tt.wantTenant)
			}
			if ac.AuthMethod != tt.wantMethod {
				t.Errorf("AuthMethod = %q, want %q", ac.AuthMethod, tt.wantMethod)
			}
			if len(ac.Roles) != len(tt.wantRoles) {
				t.Errorf("Roles = %v, want %v", ac.Roles, tt.wantRoles)
			} else {
				for i, r := range ac.Roles {
					if r != tt.wantRoles[i] {
						t.Errorf("Roles[%d] = %q, want %q", i, r, tt.wantRoles[i])
					}
				}
			}
		})
	}
}
