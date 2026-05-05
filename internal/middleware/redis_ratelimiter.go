package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisRateLimiter implements RateLimiter using a ZSET sliding window.
type RedisRateLimiter struct {
	client *redis.Client
	window time.Duration
}

// NewRedisRateLimiter creates a ZSET-based sliding window rate limiter.
func NewRedisRateLimiter(client *redis.Client) *RedisRateLimiter {
	return &RedisRateLimiter{client: client, window: 60 * time.Second}
}

// rateLimitScript atomically checks and enforces the sliding window.
// It removes expired entries, checks count, adds only if under limit,
// and returns [allowed (0/1), remaining].
var rateLimitScript = redis.NewScript(`
local key = KEYS[1]
local window_start = ARGV[1]
local now_score = ARGV[2]
local member = ARGV[3]
local limit = tonumber(ARGV[4])
local ttl = tonumber(ARGV[5])

redis.call('ZREMRANGEBYSCORE', key, '0', window_start)
local count = redis.call('ZCARD', key)

if count < limit then
    redis.call('ZADD', key, now_score, member)
    redis.call('EXPIRE', key, ttl)
    return {1, limit - count - 1}
end

redis.call('EXPIRE', key, ttl)
return {0, 0}
`)

// Allow checks if the request is within the rate limit using a Lua-based atomic ZSET sliding window.
func (r *RedisRateLimiter) Allow(key string, limit int) (bool, int, error) {
	ctx := context.Background()
	now := time.Now()
	nowNano := now.UnixNano()
	windowStart := now.Add(-r.window).UnixNano()
	ttlSeconds := int(r.window.Seconds()) + 1

	member := uniqueMember(now)

	result, err := rateLimitScript.Run(ctx, r.client, []string{key},
		strconv.FormatInt(windowStart, 10),
		strconv.FormatInt(nowNano, 10),
		member,
		limit,
		ttlSeconds,
	).Int64Slice()
	if err != nil {
		return false, 0, err
	}

	allowed := result[0] == 1
	remaining := int(result[1])
	return allowed, remaining, nil
}

func uniqueMember(now time.Time) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d:%s", now.UnixNano(), hex.EncodeToString(b))
}
