package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedisDualLimiter(t *testing.T) (*RedisDualLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return NewRedisDualLimiter(client), mr
}

func TestRedisDualLimiter_BothAllow(t *testing.T) {
	rl, _ := newTestRedisDualLimiter(t)
	ctx := context.Background()

	allowed, tRem, uRem, err := rl.AllowDual(ctx, "rl:{t1}", 10, "rl:{t1}:user:u1", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Error("expected allowed")
	}
	if tRem != 9 {
		t.Errorf("tenantRemaining = %d, want 9", tRem)
	}
	if uRem != 4 {
		t.Errorf("userRemaining = %d, want 4", uRem)
	}
}

func TestRedisDualLimiter_UserExhausted(t *testing.T) {
	rl, _ := newTestRedisDualLimiter(t)
	ctx := context.Background()

	for range 3 {
		rl.AllowDual(ctx, "rl:{t1}", 10, "rl:{t1}:user:u1", 3)
	}

	allowed, tRem, uRem, err := rl.AllowDual(ctx, "rl:{t1}", 10, "rl:{t1}:user:u1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Error("should be denied by user limit")
	}
	if uRem != 0 {
		t.Errorf("userRemaining = %d, want 0", uRem)
	}
	if tRem <= 0 {
		t.Errorf("tenantRemaining = %d, want > 0", tRem)
	}
}

func TestRedisDualLimiter_TenantExhausted(t *testing.T) {
	rl, _ := newTestRedisDualLimiter(t)
	ctx := context.Background()

	for i := range 3 {
		rl.AllowDual(ctx, "rl:{t1}", 3, "rl:{t1}:user:u"+string(rune('1'+i)), 10)
	}

	allowed, tRem, _, err := rl.AllowDual(ctx, "rl:{t1}", 3, "rl:{t1}:user:u9", 10)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Error("should be denied by tenant limit")
	}
	if tRem != 0 {
		t.Errorf("tenantRemaining = %d, want 0", tRem)
	}
}

func TestRedisDualLimiter_WindowExpiry(t *testing.T) {
	rl, mr := newTestRedisDualLimiter(t)
	ctx := context.Background()

	for range 2 {
		rl.AllowDual(ctx, "rl:{t1}", 2, "rl:{t1}:user:u1", 5)
	}

	allowed, _, _, _ := rl.AllowDual(ctx, "rl:{t1}", 2, "rl:{t1}:user:u1", 5)
	if allowed {
		t.Error("should be denied")
	}

	mr.FastForward(61 * time.Second)

	allowed, tRem, uRem, err := rl.AllowDual(ctx, "rl:{t1}", 2, "rl:{t1}:user:u1", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Error("expected allowed after window expiry")
	}
	if tRem != 1 {
		t.Errorf("tenantRemaining = %d, want 1", tRem)
	}
	if uRem != 4 {
		t.Errorf("userRemaining = %d, want 4", uRem)
	}
}

func TestRedisDualLimiter_InterfaceCompliance(t *testing.T) {
	rl, _ := newTestRedisDualLimiter(t)
	var _ DualLayerLimiter = rl
}
