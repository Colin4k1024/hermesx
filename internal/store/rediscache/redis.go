package rediscache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var redisTracer = otel.Tracer("hermes-redis")

// Client wraps a Redis connection for distributed state.
type Client struct {
	rdb *redis.Client
}

// New creates a Redis client from a URL.
func New(ctx context.Context, redisURL string) (*Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis parse url: %w", err)
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	slog.Info("Redis connected", "addr", opts.Addr)
	return &Client{rdb: rdb}, nil
}

// Close closes the Redis connection.
func (c *Client) Close() error { return c.rdb.Close() }

// --- Session Lock (Distributed) ---

// AcquireSessionLock attempts to acquire a distributed lock for a session.
// Returns the lock token (for owner-verified release) and whether acquisition succeeded.
func (c *Client) AcquireSessionLock(ctx context.Context, tenantID, sessionID string, ttl time.Duration) (string, bool, error) {
	ctx, span := redisTracer.Start(ctx, "redis.AcquireSessionLock",
		trace.WithAttributes(
			attribute.String("tenant.id", tenantID),
			attribute.String("session.id", sessionID),
		),
	)
	defer span.End()

	key := fmt.Sprintf("lock:session:%s:%s", tenantID, sessionID)
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	ok, err := c.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.SetAttributes(attribute.Bool("lock.acquired", ok))
	return token, ok, err
}

// releaseScript is a Lua script that only deletes the lock if the caller owns it.
var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
else
	return 0
end
`)

// ReleaseSessionLock releases a session lock only if the caller holds it (owner-verified).
func (c *Client) ReleaseSessionLock(ctx context.Context, tenantID, sessionID, token string) error {
	ctx, span := redisTracer.Start(ctx, "redis.ReleaseSessionLock",
		trace.WithAttributes(
			attribute.String("tenant.id", tenantID),
			attribute.String("session.id", sessionID),
		),
	)
	defer span.End()

	key := fmt.Sprintf("lock:session:%s:%s", tenantID, sessionID)
	_, err := releaseScript.Run(ctx, c.rdb, []string{key}, token).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// ExtendSessionLock extends the TTL of a session lock only if the caller holds it.
func (c *Client) ExtendSessionLock(ctx context.Context, tenantID, sessionID, token string, ttl time.Duration) error {
	key := fmt.Sprintf("lock:session:%s:%s", tenantID, sessionID)
	// Verify ownership before extending
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil || val != token {
		return fmt.Errorf("lock not owned by this token")
	}
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// --- Rate Limiting ---

// CheckRateLimit increments and checks a rate limit counter. Returns (allowed, current count).
func (c *Client) CheckRateLimit(ctx context.Context, tenantID, userID string, window time.Duration, maxRequests int) (bool, int64, error) {
	ctx, span := redisTracer.Start(ctx, "redis.CheckRateLimit",
		trace.WithAttributes(
			attribute.String("tenant.id", tenantID),
			attribute.String("user.id", userID),
		),
	)
	defer span.End()

	key := fmt.Sprintf("ratelimit:%s:%s:%.0f", tenantID, userID, window.Seconds())

	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	_, err := pipe.Exec(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, 0, err // fail closed
	}

	count := incr.Val()
	allowed := count <= int64(maxRequests)
	span.SetAttributes(attribute.Bool("ratelimit.allowed", allowed))
	return allowed, count, nil
}

// Ping checks Redis connectivity (used by health probes).
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Allow implements the middleware.RateLimiter interface using a 1-minute sliding window.
func (c *Client) Allow(key string, limit int) (bool, int, error) {
	ctx, span := redisTracer.Start(context.Background(), "redis.Allow",
		trace.WithAttributes(
			attribute.String("ratelimit.key", key),
			attribute.Int("ratelimit.limit", limit),
		),
	)
	defer span.End()

	allowed, count, err := c.CheckRateLimit(ctx, key, "", 1*time.Minute, limit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return allowed, int(int64(limit) - count), err
}

// --- Context Cache ---

// SetContextCache caches agent context summary for a session.
func (c *Client) SetContextCache(ctx context.Context, tenantID, sessionID, summary string, ttl time.Duration) error {
	ctx, span := redisTracer.Start(ctx, "redis.SetContextCache",
		trace.WithAttributes(
			attribute.String("tenant.id", tenantID),
			attribute.String("session.id", sessionID),
		),
	)
	defer span.End()

	key := fmt.Sprintf("agent:cache:%s:%s", tenantID, sessionID)
	err := c.rdb.Set(ctx, key, summary, ttl).Err()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// GetContextCache retrieves cached context summary.
func (c *Client) GetContextCache(ctx context.Context, tenantID, sessionID string) (string, error) {
	ctx, span := redisTracer.Start(ctx, "redis.GetContextCache",
		trace.WithAttributes(
			attribute.String("tenant.id", tenantID),
			attribute.String("session.id", sessionID),
		),
	)
	defer span.End()

	key := fmt.Sprintf("agent:cache:%s:%s", tenantID, sessionID)
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return val, err
}

// --- Pairing Cache ---

// SetPairingApproved caches a user's pairing approval.
func (c *Client) SetPairingApproved(ctx context.Context, tenantID, platform, userID string, ttl time.Duration) error {
	key := fmt.Sprintf("pairing:%s:%s:%s", tenantID, platform, userID)
	return c.rdb.Set(ctx, key, "approved", ttl).Err()
}

// IsPairingApproved checks the pairing cache.
func (c *Client) IsPairingApproved(ctx context.Context, tenantID, platform, userID string) (bool, error) {
	key := fmt.Sprintf("pairing:%s:%s:%s", tenantID, platform, userID)
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	return val == "approved", err
}

// --- Runtime Status ---

// SetInstanceStatus reports this instance's status to Redis.
func (c *Client) SetInstanceStatus(ctx context.Context, instanceID string, fields map[string]any, ttl time.Duration) error {
	key := fmt.Sprintf("status:gateway:%s", instanceID)
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, key, fields)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}
