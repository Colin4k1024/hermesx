package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hermes-agent/hermes-agent-go/internal/auth"
)

type mockDualLimiter struct {
	allowed         bool
	tenantRemaining int
	userRemaining   int
	err             error
	lastTenantKey   string
	lastUserKey     string
	lastTenantLimit int
	lastUserLimit   int
}

func (m *mockDualLimiter) AllowDual(_ context.Context, tenantKey string, tenantLimit int, userKey string, userLimit int) (bool, int, int, error) {
	m.lastTenantKey = tenantKey
	m.lastUserKey = userKey
	m.lastTenantLimit = tenantLimit
	m.lastUserLimit = userLimit
	return m.allowed, m.tenantRemaining, m.userRemaining, m.err
}

func TestDualLayerMiddleware(t *testing.T) {
	tests := []struct {
		name            string
		dual            *mockDualLimiter
		authCtx         *auth.AuthContext
		defaultRPM      int
		userRPM         int
		tenantLimitFn   func(string) int
		userLimitFn     func(string, string) int
		wantStatus      int
		wantRemaining   string
		wantTenantKey   string
		wantUserKey     string
		wantTenantLimit int
		wantUserLimit   int
	}{
		{
			name:            "both layers allow",
			dual:            &mockDualLimiter{allowed: true, tenantRemaining: 90, userRemaining: 45},
			authCtx:         &auth.AuthContext{Identity: "u1", UserID: "u1", TenantID: "t1", Roles: []string{"user"}},
			defaultRPM:      100,
			userRPM:         50,
			wantStatus:      http.StatusOK,
			wantRemaining:   "45",
			wantTenantKey:   "rl:{t1}",
			wantUserKey:     "rl:{t1}:user:u1",
			wantTenantLimit: 100,
			wantUserLimit:   50,
		},
		{
			name:          "tenant limit exceeded",
			dual:          &mockDualLimiter{allowed: false, tenantRemaining: 0, userRemaining: 40},
			authCtx:       &auth.AuthContext{Identity: "u1", UserID: "u1", TenantID: "t1", Roles: []string{"user"}},
			defaultRPM:    100,
			userRPM:       50,
			wantStatus:    http.StatusTooManyRequests,
			wantRemaining: "0",
		},
		{
			name:          "user limit exceeded",
			dual:          &mockDualLimiter{allowed: false, tenantRemaining: 80, userRemaining: 0},
			authCtx:       &auth.AuthContext{Identity: "u1", UserID: "u1", TenantID: "t1", Roles: []string{"user"}},
			defaultRPM:    100,
			userRPM:       50,
			wantStatus:    http.StatusTooManyRequests,
			wantRemaining: "0",
		},
		{
			name:          "remaining is min of both layers",
			dual:          &mockDualLimiter{allowed: true, tenantRemaining: 5, userRemaining: 30},
			authCtx:       &auth.AuthContext{Identity: "u1", UserID: "u1", TenantID: "t1", Roles: []string{"user"}},
			defaultRPM:    100,
			userRPM:       50,
			wantStatus:    http.StatusOK,
			wantRemaining: "5",
		},
		{
			name:       "TenantLimitFn override",
			dual:       &mockDualLimiter{allowed: true, tenantRemaining: 990, userRemaining: 45},
			authCtx:    &auth.AuthContext{Identity: "u1", UserID: "u1", TenantID: "premium", Roles: []string{"user"}},
			defaultRPM: 100,
			userRPM:    50,
			tenantLimitFn: func(tid string) int {
				if tid == "premium" {
					return 1000
				}
				return 0
			},
			wantStatus:      http.StatusOK,
			wantRemaining:   "45",
			wantTenantLimit: 1000,
			wantUserLimit:   50,
		},
		{
			name:          "UserLimitFn override",
			dual:          &mockDualLimiter{allowed: true, tenantRemaining: 90, userRemaining: 190},
			authCtx:       &auth.AuthContext{Identity: "u1", UserID: "u1", TenantID: "t1", Roles: []string{"admin"}},
			defaultRPM:    100,
			userRPM:       50,
			userLimitFn:   func(tid, uid string) int { return 200 },
			wantStatus:    http.StatusOK,
			wantRemaining: "90",
			wantUserLimit: 200,
		},
		{
			name:          "dual limiter error falls back to local",
			dual:          &mockDualLimiter{err: errors.New("redis down")},
			authCtx:       &auth.AuthContext{Identity: "u1", UserID: "u1", TenantID: "t1", Roles: []string{"user"}},
			defaultRPM:    100,
			userRPM:       50,
			wantStatus:    http.StatusOK,
			wantRemaining: "49",
		},
		{
			name:       "no UserID falls back to single-layer path",
			dual:       &mockDualLimiter{allowed: true, tenantRemaining: 90, userRemaining: 45},
			authCtx:    &auth.AuthContext{Identity: "u1", UserID: "", TenantID: "t1", Roles: []string{"user"}},
			defaultRPM: 100,
			wantStatus: http.StatusOK,
		},
		{
			name:          "UserRPM defaults to DefaultRPM when zero",
			dual:          &mockDualLimiter{allowed: true, tenantRemaining: 90, userRemaining: 90},
			authCtx:       &auth.AuthContext{Identity: "u1", UserID: "u1", TenantID: "t1", Roles: []string{"user"}},
			defaultRPM:    100,
			userRPM:       0,
			wantStatus:    http.StatusOK,
			wantUserLimit: 100,
		},
		{
			name:          "hash tag in keys for Redis Cluster",
			dual:          &mockDualLimiter{allowed: true, tenantRemaining: 90, userRemaining: 45},
			authCtx:       &auth.AuthContext{Identity: "u1", UserID: "u1", TenantID: "org-abc", Roles: []string{"user"}},
			defaultRPM:    100,
			userRPM:       50,
			wantStatus:    http.StatusOK,
			wantTenantKey: "rl:{org-abc}",
			wantUserKey:   "rl:{org-abc}:user:u1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := RateLimitConfig{
				Limiter:       &mockLimiter{allowed: true, remaining: 99},
				DualLimiter:   tt.dual,
				DefaultRPM:    tt.defaultRPM,
				UserRPM:       tt.userRPM,
				TenantLimitFn: tt.tenantLimitFn,
				UserLimitFn:   tt.userLimitFn,
			}

			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			mw := RateLimitMiddleware(cfg)
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

			if tt.wantRemaining != "" && rec.Header().Get("X-RateLimit-Remaining") != tt.wantRemaining {
				t.Errorf("X-RateLimit-Remaining = %q, want %q", rec.Header().Get("X-RateLimit-Remaining"), tt.wantRemaining)
			}

			if tt.wantTenantKey != "" && tt.dual.lastTenantKey != tt.wantTenantKey {
				t.Errorf("tenantKey = %q, want %q", tt.dual.lastTenantKey, tt.wantTenantKey)
			}
			if tt.wantUserKey != "" && tt.dual.lastUserKey != tt.wantUserKey {
				t.Errorf("userKey = %q, want %q", tt.dual.lastUserKey, tt.wantUserKey)
			}
			if tt.wantTenantLimit > 0 && tt.dual.lastTenantLimit != tt.wantTenantLimit {
				t.Errorf("tenantLimit = %d, want %d", tt.dual.lastTenantLimit, tt.wantTenantLimit)
			}
			if tt.wantUserLimit > 0 && tt.dual.lastUserLimit != tt.wantUserLimit {
				t.Errorf("userLimit = %d, want %d", tt.dual.lastUserLimit, tt.wantUserLimit)
			}
		})
	}
}

func TestLocalDualLimiter_Basic(t *testing.T) {
	l := NewLocalDualLimiter()
	ctx := context.Background()

	allowed, tRem, uRem, err := l.AllowDual(ctx, "tenant:t1", 3, "user:u1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed || tRem != 2 || uRem != 1 {
		t.Errorf("first call: allowed=%v, tRem=%d, uRem=%d", allowed, tRem, uRem)
	}

	allowed, tRem, uRem, _ = l.AllowDual(ctx, "tenant:t1", 3, "user:u1", 2)
	if !allowed || tRem != 1 || uRem != 0 {
		t.Errorf("second call: allowed=%v, tRem=%d, uRem=%d", allowed, tRem, uRem)
	}

	allowed, _, uRem, _ = l.AllowDual(ctx, "tenant:t1", 3, "user:u1", 2)
	if allowed {
		t.Error("third call should be denied (user limit)")
	}
	if uRem != 0 {
		t.Errorf("user remaining should be 0, got %d", uRem)
	}
}

func TestLocalDualLimiter_TenantExhaustion(t *testing.T) {
	l := NewLocalDualLimiter()
	ctx := context.Background()

	l.AllowDual(ctx, "tenant:t1", 2, "user:u1", 10)
	l.AllowDual(ctx, "tenant:t1", 2, "user:u2", 10)

	allowed, tRem, _, _ := l.AllowDual(ctx, "tenant:t1", 2, "user:u3", 10)
	if allowed {
		t.Error("should be denied by tenant limit")
	}
	if tRem != 0 {
		t.Errorf("tenant remaining should be 0, got %d", tRem)
	}
}
