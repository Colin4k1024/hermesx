package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/auth"
)

func TestSSEConnectionTracker_IncrDecr(t *testing.T) {
	tracker := NewSSEConnectionTracker(3)

	key := "tenant-1:user-1"

	// Initial count should be 0.
	if got := tracker.ActiveSSEConnections(key); got != 0 {
		t.Errorf("initial count = %d, want 0", got)
	}

	// Increment.
	tracker.IncrSSEConnections(key)
	if got := tracker.ActiveSSEConnections(key); got != 1 {
		t.Errorf("after first incr = %d, want 1", got)
	}

	tracker.IncrSSEConnections(key)
	tracker.IncrSSEConnections(key)
	if got := tracker.ActiveSSEConnections(key); got != 3 {
		t.Errorf("after three incr = %d, want 3", got)
	}

	// Decrement.
	tracker.DecrSSEConnections(key)
	if got := tracker.ActiveSSEConnections(key); got != 2 {
		t.Errorf("after one decr = %d, want 2", got)
	}

	// Decrement to zero.
	tracker.DecrSSEConnections(key)
	tracker.DecrSSEConnections(key)
	if got := tracker.ActiveSSEConnections(key); got != 0 {
		t.Errorf("after full decr = %d, want 0", got)
	}

	// Decrement below zero should return 0 (no negative counts).
	got := tracker.DecrSSEConnections(key)
	if got < 0 {
		t.Errorf("decrement below zero = %d, want >= 0", got)
	}
}

func TestSSEConnectionTracker_IndependentKeys(t *testing.T) {
	tracker := NewSSEConnectionTracker(5)

	tracker.IncrSSEConnections("t1:u1")
	tracker.IncrSSEConnections("t1:u1")
	tracker.IncrSSEConnections("t2:u2")

	if got := tracker.ActiveSSEConnections("t1:u1"); got != 2 {
		t.Errorf("t1:u1 count = %d, want 2", got)
	}
	if got := tracker.ActiveSSEConnections("t2:u2"); got != 1 {
		t.Errorf("t2:u2 count = %d, want 1", got)
	}
}

func TestSSEConnectionTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewSSEConnectionTracker(100)
	key := "tenant-1:user-1"

	var wg sync.WaitGroup
	n := 50
	wg.Add(n * 2)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			tracker.IncrSSEConnections(key)
		}()
		go func() {
			defer wg.Done()
			tracker.IncrSSEConnections(key)
		}()
	}
	wg.Wait()

	if got := tracker.ActiveSSEConnections(key); got != n*2 {
		t.Errorf("concurrent incr count = %d, want %d", got, n*2)
	}

	// Now decrement all concurrently.
	wg.Add(n * 2)
	for i := 0; i < n*2; i++ {
		go func() {
			defer wg.Done()
			tracker.DecrSSEConnections(key)
		}()
	}
	wg.Wait()

	if got := tracker.ActiveSSEConnections(key); got != 0 {
		t.Errorf("after concurrent decr = %d, want 0", got)
	}
}

func TestSSEConnectionTracker_DefaultMax(t *testing.T) {
	tracker := NewSSEConnectionTracker(0) // should default to DefaultMaxSSEStreamsPerUser
	if tracker.MaxSSEStreamsPerUser != DefaultMaxSSEStreamsPerUser {
		t.Errorf("default max = %d, want %d", tracker.MaxSSEStreamsPerUser, DefaultMaxSSEStreamsPerUser)
	}
}

func TestSSEConnectionTracker_MiddlewareRejectsAtLimit(t *testing.T) {
	tracker := NewSSEConnectionTracker(2) // limit of 2

	cfg := RateLimitConfig{
		Limiter:    &mockLimiter{allowed: true, remaining: 100},
		DefaultRPM: 60,
		SSETracker: tracker,
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RateLimitMiddleware(cfg)
	handler := mw(inner)

	ac := &auth.AuthContext{
		Identity: "user-1",
		TenantID: "tenant-1",
		Roles:    []string{"user"},
	}

	// Fill up the connection limit.
	tracker.IncrSSEConnections("tenant-1:user-1")
	tracker.IncrSSEConnections("tenant-1:user-1")

	// Third SSE request should be rejected.
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Accept", "text/event-stream")
	req = req.WithContext(auth.WithContext(req.Context(), ac))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("SSE over limit: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("X-SSE-Limit") != "2" {
		t.Errorf("X-SSE-Limit = %q, want %q", rec.Header().Get("X-SSE-Limit"), "2")
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After header on 429 SSE rejection")
	}

	// Cleanup.
	tracker.DecrSSEConnections("tenant-1:user-1")
	tracker.DecrSSEConnections("tenant-1:user-1")
}

func TestSSEConnectionTracker_MiddlewarePassesWhenUnderLimit(t *testing.T) {
	tracker := NewSSEConnectionTracker(5) // limit of 5

	cfg := RateLimitConfig{
		Limiter:    &mockLimiter{allowed: true, remaining: 100},
		DefaultRPM: 60,
		SSETracker: tracker,
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RateLimitMiddleware(cfg)
	handler := mw(inner)

	ac := &auth.AuthContext{
		Identity: "user-1",
		TenantID: "tenant-1",
		Roles:    []string{"user"},
	}

	// Pre-occupy 4 out of 5 slots.
	for i := 0; i < 4; i++ {
		tracker.IncrSSEConnections("tenant-1:user-1")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	req.Header.Set("Accept", "text/event-stream")
	req = req.WithContext(auth.WithContext(req.Context(), ac))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("SSE under limit: status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Cleanup.
	for i := 0; i < 4; i++ {
		tracker.DecrSSEConnections("tenant-1:user-1")
	}
}

func TestSSEConnectionTracker_MiddlewareNoSSEHeaderBypassesCheck(t *testing.T) {
	tracker := NewSSEConnectionTracker(1) // very tight limit

	cfg := RateLimitConfig{
		Limiter:    &mockLimiter{allowed: true, remaining: 100},
		DefaultRPM: 60,
		SSETracker: tracker,
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RateLimitMiddleware(cfg)
	handler := mw(inner)

	ac := &auth.AuthContext{
		Identity: "user-1",
		TenantID: "tenant-1",
		Roles:    []string{"user"},
	}

	// Fill the limit.
	tracker.IncrSSEConnections("tenant-1:user-1")

	// Non-SSE request should not be affected.
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req = req.WithContext(auth.WithContext(req.Context(), ac))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("non-SSE request with full SSE limit: status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Cleanup.
	tracker.DecrSSEConnections("tenant-1:user-1")
}
