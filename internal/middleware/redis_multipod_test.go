package middleware

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisRateLimiter_MultiPodConcurrency(t *testing.T) {
	mr := miniredis.RunT(t)
	const limit = 20
	const pods = 4
	const requestsPerPod = 10

	var allowed atomic.Int64
	var denied atomic.Int64

	var wg sync.WaitGroup
	for p := range pods {
		_ = p
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			defer client.Close()
			rl := NewRedisRateLimiter(client)

			for range requestsPerPod {
				ok, _, err := rl.Allow("rl:shared-tenant", limit)
				if err != nil {
					t.Errorf("Allow error: %v", err)
					return
				}
				if ok {
					allowed.Add(1)
				} else {
					denied.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	totalAllowed := allowed.Load()
	totalDenied := denied.Load()
	totalRequests := int64(pods * requestsPerPod)

	if totalAllowed+totalDenied != totalRequests {
		t.Errorf("total = %d, want %d", totalAllowed+totalDenied, totalRequests)
	}
	if totalAllowed > int64(limit) {
		t.Errorf("allowed %d requests, but limit is %d — sliding window violated", totalAllowed, limit)
	}
	if totalAllowed < int64(limit) {
		t.Errorf("only allowed %d, expected exactly %d with shared state", totalAllowed, limit)
	}
	if totalDenied != totalRequests-int64(limit) {
		t.Errorf("denied = %d, want %d", totalDenied, totalRequests-int64(limit))
	}
}

func TestRedisDualLimiter_MultiPodConcurrency(t *testing.T) {
	mr := miniredis.RunT(t)
	const tenantLimit = 15
	const userLimit = 8
	const pods = 3
	const requestsPerPod = 10

	var allowed atomic.Int64
	var wg sync.WaitGroup

	for p := range pods {
		_ = p
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			defer client.Close()
			rl := NewRedisDualLimiter(client)
			ctx := context.Background()

			for range requestsPerPod {
				ok, _, _, err := rl.AllowDual(ctx, "rl:{t1}", tenantLimit, "rl:{t1}:user:u1", userLimit)
				if err != nil {
					t.Errorf("AllowDual error: %v", err)
					return
				}
				if ok {
					allowed.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	totalAllowed := allowed.Load()
	if totalAllowed > int64(userLimit) {
		t.Errorf("allowed %d requests, but user limit is %d", totalAllowed, userLimit)
	}
	if totalAllowed < int64(userLimit) {
		t.Errorf("only allowed %d, expected exactly %d with shared Redis state", totalAllowed, userLimit)
	}
}
