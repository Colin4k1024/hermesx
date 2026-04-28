package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hermes-agent/hermes-agent-go/internal/auth"
)

// mockExtractor is a test double for auth.CredentialExtractor.
type mockExtractor struct {
	ac  *auth.AuthContext
	err error
}

func (m *mockExtractor) Extract(_ *http.Request) (*auth.AuthContext, error) {
	return m.ac, m.err
}

func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		extractor      *mockExtractor
		allowAnonymous bool
		wantStatus     int
		wantIdentity   string // expected identity in context; empty means no AuthContext
	}{
		{
			name: "valid auth populates context",
			extractor: &mockExtractor{
				ac: &auth.AuthContext{
					Identity: "user-1",
					TenantID: "t-1",
					Roles:    []string{"user"},
				},
			},
			allowAnonymous: false,
			wantStatus:     http.StatusOK,
			wantIdentity:   "user-1",
		},
		{
			name: "extractor returns error gives 401",
			extractor: &mockExtractor{
				err: errors.New("invalid token"),
			},
			allowAnonymous: false,
			wantStatus:     http.StatusUnauthorized,
		},
		{
			name:           "no credentials and allowAnonymous false gives 401",
			extractor:      &mockExtractor{},
			allowAnonymous: false,
			wantStatus:     http.StatusUnauthorized,
		},
		{
			name:           "no credentials and allowAnonymous true gives 200",
			extractor:      &mockExtractor{},
			allowAnonymous: true,
			wantStatus:     http.StatusOK,
			wantIdentity:   "", // no AuthContext set
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := auth.NewExtractorChain(tt.extractor)

			var capturedIdentity string
			var contextHasAuth bool
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ac, ok := auth.FromContext(r.Context())
				contextHasAuth = ok && ac != nil
				if contextHasAuth {
					capturedIdentity = ac.Identity
				}
				w.WriteHeader(http.StatusOK)
			})

			mw := AuthMiddleware(chain, tt.allowAnonymous)
			handler := mw(inner)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK && tt.wantIdentity != "" {
				if !contextHasAuth {
					t.Fatal("expected AuthContext in context, got none")
				}
				if capturedIdentity != tt.wantIdentity {
					t.Errorf("identity = %q, want %q", capturedIdentity, tt.wantIdentity)
				}
			}

			if tt.wantStatus == http.StatusOK && tt.wantIdentity == "" && tt.allowAnonymous {
				if contextHasAuth {
					t.Error("expected no AuthContext for anonymous request, but got one")
				}
			}
		})
	}
}
