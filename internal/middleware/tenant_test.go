package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hermes-agent/hermes-agent-go/internal/auth"
)

func TestTenantMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		authCtx    *auth.AuthContext
		wantStatus int
		wantTenant string
	}{
		{
			name: "derives tenant from AuthContext",
			authCtx: &auth.AuthContext{
				Identity: "user-1",
				TenantID: "acme-corp",
				Roles:    []string{"user"},
			},
			wantStatus: http.StatusOK,
			wantTenant: "acme-corp",
		},
		{
			name:       "defaults to default when no AuthContext",
			authCtx:    nil,
			wantStatus: http.StatusOK,
			wantTenant: "default",
		},
		{
			name: "defaults to default when TenantID is empty",
			authCtx: &auth.AuthContext{
				Identity: "user-2",
				TenantID: "",
				Roles:    []string{"user"},
			},
			wantStatus: http.StatusOK,
			wantTenant: "default",
		},
		{
			name: "invalid tenant ID with spaces returns 400",
			authCtx: &auth.AuthContext{
				Identity: "user-3",
				TenantID: "bad tenant!",
				Roles:    []string{"user"},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "tenant ID too long returns 400",
			authCtx: &auth.AuthContext{
				Identity: "user-4",
				TenantID: strings.Repeat("a", 65),
				Roles:    []string{"user"},
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedTenant string
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedTenant = TenantFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authCtx != nil {
				ctx := auth.WithContext(req.Context(), tt.authCtx)
				req = req.WithContext(ctx)
			}

			rec := httptest.NewRecorder()
			TenantMiddleware(inner).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK && capturedTenant != tt.wantTenant {
				t.Errorf("tenant = %q, want %q", capturedTenant, tt.wantTenant)
			}
		})
	}
}

func TestTenantFromContext_WithTenant_RoundTrip(t *testing.T) {
	ctx := context.Background()

	if got := TenantFromContext(ctx); got != "" {
		t.Errorf("TenantFromContext on empty context = %q, want empty", got)
	}

	ctx = WithTenant(ctx, "my-tenant")
	if got := TenantFromContext(ctx); got != "my-tenant" {
		t.Errorf("TenantFromContext = %q, want %q", got, "my-tenant")
	}
}
