package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/auth"
)

func TestRequireAnyScope(t *testing.T) {
	tests := []struct {
		name       string
		authCtx    *auth.AuthContext
		scopes     []string
		wantStatus int
	}{
		{
			name:       "missing auth returns unauthorized",
			authCtx:    nil,
			scopes:     []string{"billing:read"},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "matching domain scope returns ok",
			authCtx: &auth.AuthContext{
				Identity: "u1",
				TenantID: "t1",
				Scopes:   []string{"billing:read"},
			},
			scopes:     []string{"billing:read"},
			wantStatus: http.StatusOK,
		},
		{
			name: "admin scope is explicit break-glass",
			authCtx: &auth.AuthContext{
				Identity: "u2",
				TenantID: "t1",
				Scopes:   []string{"admin"},
			},
			scopes:     []string{"security:write"},
			wantStatus: http.StatusOK,
		},
		{
			name: "legacy empty scopes do not pass admin domain checks",
			authCtx: &auth.AuthContext{
				Identity: "u3",
				TenantID: "t1",
				Scopes:   nil,
			},
			scopes:     []string{"billing:read"},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "wrong scope returns forbidden",
			authCtx: &auth.AuthContext{
				Identity: "u4",
				TenantID: "t1",
				Scopes:   []string{"audit:read"},
			},
			scopes:     []string{"billing:read"},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RequireAnyScope(tt.scopes...)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/admin/v1/test", nil)
			if tt.authCtx != nil {
				req = req.WithContext(auth.WithContext(req.Context(), tt.authCtx))
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
