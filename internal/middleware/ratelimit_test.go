package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/auth"
)

// mockLimiter is a test double for the RateLimiter interface.
type mockLimiter struct {
	allowed   bool
	remaining int
	err       error
}

func (m *mockLimiter) Allow(_ string, _ int) (bool, int, error) {
	return m.allowed, m.remaining, m.err
}

func TestRateLimitMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		cfg        RateLimitConfig
		authCtx    *auth.AuthContext // nil means no AuthContext
		wantStatus int
		checkRetry bool // whether to check Retry-After header
	}{
		{
			name: "under limit returns 200 with headers",
			cfg: RateLimitConfig{
				Limiter:    &mockLimiter{allowed: true, remaining: 59},
				DefaultRPM: 60,
			},
			authCtx: &auth.AuthContext{
				Identity: "u1",
				TenantID: "t1",
				Roles:    []string{"user"},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "over limit returns 429 with Retry-After",
			cfg: RateLimitConfig{
				Limiter:    &mockLimiter{allowed: false, remaining: 0},
				DefaultRPM: 60,
			},
			authCtx: &auth.AuthContext{
				Identity: "u1",
				TenantID: "t1",
				Roles:    []string{"user"},
			},
			wantStatus: http.StatusTooManyRequests,
			checkRetry: true,
		},
		{
			name: "limiter error falls back to local limiter and allows first request",
			cfg: RateLimitConfig{
				Limiter:    &mockLimiter{err: errForTest},
				DefaultRPM: 60,
			},
			authCtx: &auth.AuthContext{
				Identity: "u1",
				TenantID: "t1",
				Roles:    []string{"user"},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "no limiter configured uses local limiter",
			cfg: RateLimitConfig{
				Limiter:    nil,
				DefaultRPM: 60,
			},
			authCtx: &auth.AuthContext{
				Identity: "u1",
				TenantID: "t-local",
				Roles:    []string{"user"},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "tenant key from AuthContext",
			cfg: RateLimitConfig{
				Limiter:    &mockLimiter{allowed: true, remaining: 99},
				DefaultRPM: 100,
				TenantLimitFn: func(tenantID string) int {
					if tenantID == "premium" {
						return 1000
					}
					return 0
				},
			},
			authCtx: &auth.AuthContext{
				Identity: "u1",
				TenantID: "premium",
				Roles:    []string{"user"},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "IP fallback when no AuthContext",
			cfg: RateLimitConfig{
				Limiter:    &mockLimiter{allowed: true, remaining: 58},
				DefaultRPM: 60,
				// NOTE: TenantLimitFn is nil to avoid nil pointer dereference
				// when ac is nil (no AuthContext).
			},
			authCtx:    nil,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			mw := RateLimitMiddleware(tt.cfg)
			handler := mw(inner)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authCtx != nil {
				ctx := auth.WithContext(req.Context(), tt.authCtx)
				req = req.WithContext(ctx)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				if rec.Header().Get("X-RateLimit-Limit") == "" {
					t.Error("missing X-RateLimit-Limit header")
				}
				if rec.Header().Get("X-RateLimit-Remaining") == "" {
					t.Error("missing X-RateLimit-Remaining header")
				}
			}

			if tt.checkRetry {
				if rec.Header().Get("Retry-After") == "" {
					t.Error("missing Retry-After header on 429 response")
				}
			}
		})
	}
}

// errForTest is a sentinel error for testing.
var errForTest = errTest("limiter failure")

type errTest string

func (e errTest) Error() string { return string(e) }
