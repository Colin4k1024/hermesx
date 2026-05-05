package middleware

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedisLimiter(t *testing.T) (*RedisRateLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return NewRedisRateLimiter(client), mr
}

func TestRedisRateLimiter_Allow(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		limit     int
		calls     int
		wantLast  bool
		wantRemGE int
	}{
		{
			name:      "single request under limit",
			key:       "rl:t1",
			limit:     10,
			calls:     1,
			wantLast:  true,
			wantRemGE: 9,
		},
		{
			name:      "exactly at limit still allowed",
			key:       "rl:t2",
			limit:     5,
			calls:     5,
			wantLast:  true,
			wantRemGE: 0,
		},
		{
			name:      "over limit denied",
			key:       "rl:t3",
			limit:     3,
			calls:     4,
			wantLast:  false,
			wantRemGE: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rl, _ := newTestRedisLimiter(t)
			var allowed bool
			var remaining int
			var err error

			for i := 0; i < tc.calls; i++ {
				allowed, remaining, err = rl.Allow(tc.key, tc.limit)
				if err != nil {
					t.Fatalf("Allow() returned unexpected error: %v", err)
				}
			}

			if allowed != tc.wantLast {
				t.Errorf("allowed = %v, want %v after %d calls", allowed, tc.wantLast, tc.calls)
			}
			if remaining < tc.wantRemGE {
				t.Errorf("remaining = %d, want >= %d", remaining, tc.wantRemGE)
			}
		})
	}
}

func TestRedisRateLimiter_WindowExpiry(t *testing.T) {
	rl, mr := newTestRedisLimiter(t)
	key := "rl:expire"
	limit := 2

	for i := 0; i < 2; i++ {
		_, _, err := rl.Allow(key, limit)
		if err != nil {
			t.Fatalf("Allow() error: %v", err)
		}
	}

	// Third request should be denied.
	allowed, _, err := rl.Allow(key, limit)
	if err != nil {
		t.Fatalf("Allow() error: %v", err)
	}
	if allowed {
		t.Fatal("expected denied after exceeding limit")
	}

	// Fast-forward time past window.
	mr.FastForward(61 * time.Second)

	// After window expires, request should be allowed again.
	allowed, remaining, err := rl.Allow(key, limit)
	if err != nil {
		t.Fatalf("Allow() error: %v", err)
	}
	if !allowed {
		t.Error("expected allowed after window expiry")
	}
	if remaining < 1 {
		t.Errorf("remaining = %d, want >= 1", remaining)
	}
}

func TestRedisRateLimiter_InterfaceCompliance(t *testing.T) {
	rl, _ := newTestRedisLimiter(t)
	var _ RateLimiter = rl
}
