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

// UniversalClient returns the underlying Redis client (satisfies redis.UniversalClient).
func (c *Client) UniversalClient() redis.UniversalClient { return c.rdb }

// GoRedisClient returns the underlying *redis.Client for direct use.
func (c *Client) GoRedisClient() *redis.Client { return c.rdb }

// --- Session Lock (Distributed) ---

// Lock Key Design:
//
// The lock key hierarchy supports multi-task parallelism within a session:
//
//   Session-level lock (existing): hermes:lock:{tenant_id}:{session_id}
//     - Prevents the same session from being processed concurrently by multiple pods
//     - Used by AgentFactory.Run for the conversation turn lock
//
//   Task-level lock (new): hermes:lock:{tenant_id}:{session_id}:{tool_name}
//     - Allows concurrent sessions for the same tenant to each acquire tool locks
//     - Prevents the same tool from being invoked concurrently within a session
//     - Enables parallel tool execution across different sessions
//
// Migration note: The old key format "lock:session:{tenant}:{session}" is preserved
// for backward compatibility. New code should use the TaskLock functions for
// finer-grained parallelism.

// sessionLockKey returns the Redis key for a session-level lock.
// Format: hermes:lock:{tenant_id}:{session_id}
func sessionLockKey(tenantID, sessionID string) string {
	return fmt.Sprintf("hermes:lock:%s:%s", tenantID, sessionID)
}

// taskLockKey returns the Redis key for a task/tool-level lock.
// Format: hermes:lock:{tenant_id}:{session_id}:{tool_name}
func taskLockKey(tenantID, sessionID, toolName string) string {
	return fmt.Sprintf("hermes:lock:%s:%s:%s", tenantID, sessionID, toolName)
}

// legacySessionLockKey returns the old-format Redis key for a session-level lock.
// Format: lock:session:{tenant_id}:{session_id}
// Used for backward compatibility during rolling deploys.
func legacySessionLockKey(tenantID, sessionID string) string {
	return fmt.Sprintf("lock:session:%s:%s", tenantID, sessionID)
}

// AcquireSessionLock attempts to acquire a distributed lock for a session.
// Returns the lock token (for owner-verified release) and whether acquisition succeeded.
// It also attempts to clean up any old-format lock key for backward compatibility
// during rolling deploys where old pods may still use the legacy key format.
func (c *Client) AcquireSessionLock(ctx context.Context, tenantID, sessionID string, ttl time.Duration) (string, bool, error) {
	ctx, span := redisTracer.Start(ctx, "redis.AcquireSessionLock",
		trace.WithAttributes(
			attribute.String("tenant.id", tenantID),
			attribute.String("session.id", sessionID),
		),
	)
	defer span.End()

	key := sessionLockKey(tenantID, sessionID)
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	ok, err := c.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	// During rolling deploy, old pods may still use the legacy key format.
	// If we acquired the new-format lock, best-effort delete any legacy key
	// to prevent old pods from also acquiring it.
	if ok {
		legacyKey := legacySessionLockKey(tenantID, sessionID)
		if delErr := c.rdb.Del(ctx, legacyKey).Err(); delErr != nil {
			slog.Debug("redis: failed to clean up legacy lock key (non-fatal)",
				"key", legacyKey, "error", delErr)
		}
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

	key := sessionLockKey(tenantID, sessionID)
	_, err := releaseScript.Run(ctx, c.rdb, []string{key}, token).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// ExtendSessionLock extends the TTL of a session lock only if the caller holds it.
func (c *Client) ExtendSessionLock(ctx context.Context, tenantID, sessionID, token string, ttl time.Duration) error {
	key := sessionLockKey(tenantID, sessionID)
	// Verify ownership before extending
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil || val != token {
		return fmt.Errorf("lock not owned by this token")
	}
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// --- Task Lock (Distributed, session+tool granularity) ---

// AcquireTaskLock attempts to acquire a distributed lock for a specific tool
// within a session. This allows concurrent sessions for the same tenant to
// proceed independently, while preventing the same tool from running concurrently
// within a single session.
//
// Key format: hermes:lock:{tenant_id}:{session_id}:{tool_name}
func (c *Client) AcquireTaskLock(ctx context.Context, tenantID, sessionID, toolName string, ttl time.Duration) (string, bool, error) {
	ctx, span := redisTracer.Start(ctx, "redis.AcquireTaskLock",
		trace.WithAttributes(
			attribute.String("tenant.id", tenantID),
			attribute.String("session.id", sessionID),
			attribute.String("tool.name", toolName),
		),
	)
	defer span.End()

	key := taskLockKey(tenantID, sessionID, toolName)
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	ok, err := c.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.SetAttributes(attribute.Bool("lock.acquired", ok))
	return token, ok, err
}

// ReleaseTaskLock releases a task lock only if the caller holds it.
func (c *Client) ReleaseTaskLock(ctx context.Context, tenantID, sessionID, toolName, token string) error {
	ctx, span := redisTracer.Start(ctx, "redis.ReleaseTaskLock",
		trace.WithAttributes(
			attribute.String("tenant.id", tenantID),
			attribute.String("session.id", sessionID),
			attribute.String("tool.name", toolName),
		),
	)
	defer span.End()

	key := taskLockKey(tenantID, sessionID, toolName)
	_, err := releaseScript.Run(ctx, c.rdb, []string{key}, token).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
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
