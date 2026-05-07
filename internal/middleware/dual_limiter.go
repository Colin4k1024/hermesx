package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/redis/go-redis/v9"
)

// DualLayerLimiter checks tenant and user/key rate limits atomically.
type DualLayerLimiter interface {
	AllowDual(ctx context.Context, tenantKey string, tenantLimit int, userKey string, userLimit int) (allowed bool, tenantRemaining int, userRemaining int, err error)
}

// RedisDualLimiter implements DualLayerLimiter using a single Lua script
// that atomically checks two ZSET sliding windows.
type RedisDualLimiter struct {
	client *redis.Client
	window time.Duration
}

func NewRedisDualLimiter(client *redis.Client) *RedisDualLimiter {
	return &RedisDualLimiter{client: client, window: 60 * time.Second}
}

// dualLimitScript atomically checks two ZSET sliding windows in one round-trip.
// KEYS[1] = tenant key, KEYS[2] = user key
// Returns [allowed(0/1), tenant_remaining, user_remaining]
var dualLimitScript = redis.NewScript(`
local tenant_key = KEYS[1]
local user_key = KEYS[2]
local window_start = ARGV[1]
local now_score = ARGV[2]
local member = ARGV[3]
local tenant_limit = tonumber(ARGV[4])
local user_limit = tonumber(ARGV[5])
local ttl = tonumber(ARGV[6])

redis.call('ZREMRANGEBYSCORE', tenant_key, '0', window_start)
redis.call('ZREMRANGEBYSCORE', user_key, '0', window_start)

local tenant_count = redis.call('ZCARD', tenant_key)
local user_count = redis.call('ZCARD', user_key)

if tenant_count >= tenant_limit then
    redis.call('EXPIRE', tenant_key, ttl)
    redis.call('EXPIRE', user_key, ttl)
    return {0, 0, user_limit - user_count}
end

if user_count >= user_limit then
    redis.call('EXPIRE', tenant_key, ttl)
    redis.call('EXPIRE', user_key, ttl)
    return {0, tenant_limit - tenant_count, 0}
end

redis.call('ZADD', tenant_key, now_score, member)
redis.call('ZADD', user_key, now_score, member)
redis.call('EXPIRE', tenant_key, ttl)
redis.call('EXPIRE', user_key, ttl)

return {1, tenant_limit - tenant_count - 1, user_limit - user_count - 1}
`)

func (r *RedisDualLimiter) AllowDual(ctx context.Context, tenantKey string, tenantLimit int, userKey string, userLimit int) (bool, int, int, error) {
	now := time.Now()
	nowNano := now.UnixNano()
	windowStart := now.Add(-r.window).UnixNano()
	ttlSeconds := int(r.window.Seconds()) + 1

	member := dualUniqueMember(now)

	result, err := dualLimitScript.Run(ctx, r.client, []string{tenantKey, userKey},
		strconv.FormatInt(windowStart, 10),
		strconv.FormatInt(nowNano, 10),
		member,
		tenantLimit,
		userLimit,
		ttlSeconds,
	).Int64Slice()
	if err != nil {
		return false, 0, 0, err
	}

	allowed := result[0] == 1
	tenantRemaining := int(result[1])
	userRemaining := int(result[2])
	return allowed, tenantRemaining, userRemaining, nil
}

func dualUniqueMember(now time.Time) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d:%s", now.UnixNano(), hex.EncodeToString(b))
}

// LocalDualLimiter provides an in-memory fallback for dual-layer rate limiting.
type LocalDualLimiter struct {
	mu      sync.Mutex
	buckets *lru.Cache[string, *bucket]
}

const maxDualLimitBuckets = 20000

func NewLocalDualLimiter() *LocalDualLimiter {
	cache, _ := lru.New[string, *bucket](maxDualLimitBuckets)
	return &LocalDualLimiter{buckets: cache}
}

func (l *LocalDualLimiter) AllowDual(_ context.Context, tenantKey string, tenantLimit int, userKey string, userLimit int) (bool, int, int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	tenantAllowed, tenantRemaining := l.check(tenantKey, tenantLimit, now)
	if !tenantAllowed {
		_, userRemaining := l.peek(userKey, userLimit, now)
		return false, 0, userRemaining, nil
	}

	userAllowed, userRemaining := l.check(userKey, userLimit, now)
	if !userAllowed {
		l.rollback(tenantKey)
		return false, tenantRemaining + 1, 0, nil
	}

	return true, tenantRemaining, userRemaining, nil
}

func (l *LocalDualLimiter) check(key string, limit int, now time.Time) (bool, int) {
	b, ok := l.buckets.Get(key)
	if !ok || now.Sub(b.windowAt) >= time.Minute {
		l.buckets.Add(key, &bucket{count: 1, limit: limit, windowAt: now})
		return true, limit - 1
	}
	if b.count >= limit {
		return false, 0
	}
	b.count++
	return true, limit - b.count
}

func (l *LocalDualLimiter) peek(key string, limit int, now time.Time) (bool, int) {
	b, ok := l.buckets.Get(key)
	if !ok || now.Sub(b.windowAt) >= time.Minute {
		return true, limit
	}
	if b.count >= limit {
		return false, 0
	}
	return true, limit - b.count
}

func (l *LocalDualLimiter) rollback(key string) {
	b, ok := l.buckets.Get(key)
	if ok && b.count > 0 {
		b.count--
	}
}
