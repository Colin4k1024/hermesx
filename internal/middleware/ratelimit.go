package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/observability"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var rateLimitRejectedTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "hermesx_rate_limit_rejected_total",
		Help: "Total requests rejected by rate limiting.",
	},
	[]string{"tenant_id"},
)

var sseRejectedTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "hermesx_sse_connections_rejected_total",
		Help: "Total SSE connection requests rejected due to per-user connection limit.",
	},
	[]string{"tenant_id"},
)

// RateLimiter checks whether a request should be allowed.
type RateLimiter interface {
	Allow(key string, limit int) (bool, int, error) // allowed, remaining, error
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	Limiter       RateLimiter
	DualLimiter   DualLayerLimiter // optional: atomic tenant+user limiting
	DefaultRPM    int
	UserRPM       int                               // per-user limit (defaults to DefaultRPM if 0)
	TenantLimitFn func(tenantID string) int         // optional: per-tenant override
	UserLimitFn   func(tenantID, userID string) int // optional: per-user override

	// SSETracker tracks active SSE connections per user for connection limiting.
	// When nil, SSE connection limiting is disabled.
	SSETracker *SSEConnectionTracker
}

// RateLimitMiddleware applies per-tenant rate limiting.
// For authenticated requests with a UserID and a configured DualLimiter,
// it atomically checks both tenant and user limits.
// Falls back to in-memory limiter if the distributed limiter errors.
func RateLimitMiddleware(cfg RateLimitConfig) Middleware {
	local := newMemoryLimiter()
	localDual := NewLocalDualLimiter()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := auth.FromContext(r.Context())

			// Check SSE connection limit before regular rate limiting.
			if cfg.SSETracker != nil && r.Header.Get("Accept") == "text/event-stream" && ok && ac != nil {
				userKey := ac.TenantID + ":" + ac.Identity
				if cfg.SSETracker.MaxSSEStreamsPerUser > 0 {
					if current := cfg.SSETracker.ActiveSSEConnections(userKey); current >= cfg.SSETracker.MaxSSEStreamsPerUser {
						sseRejectedTotal.WithLabelValues(ac.TenantID).Inc()
						w.Header().Set("X-SSE-Limit", strconv.Itoa(cfg.SSETracker.MaxSSEStreamsPerUser))
						w.Header().Set("X-SSE-Current", strconv.Itoa(current))
						w.Header().Set("Retry-After", "30")
						http.Error(w, "too many concurrent SSE streams", http.StatusTooManyRequests)
						return
					}
				}
			}

			if ok && ac != nil && ac.UserID != "" && cfg.DualLimiter != nil {
				dualPath(cfg, localDual, ac, w, r, next)
				return
			}

			var key string
			if ok && ac != nil {
				key = "rl:" + ac.TenantID
			} else {
				ip := r.RemoteAddr
				if idx := strings.LastIndex(ip, ":"); idx != -1 {
					ip = ip[:idx]
				}
				key = "rl:anon:" + ip
			}
			limit := cfg.DefaultRPM
			if cfg.TenantLimitFn != nil && ac != nil {
				if tl := cfg.TenantLimitFn(ac.TenantID); tl > 0 {
					limit = tl
				}
			}

			var allowed bool
			var remaining int

			if cfg.Limiter != nil {
				var err error
				allowed, remaining, err = cfg.Limiter.Allow(key, limit)
				if err != nil {
					observability.ContextLogger(r.Context()).Warn("distributed rate limiter failed, falling back to local", "error", err)
					allowed, remaining = local.allow(key, limit)
				}
			} else {
				allowed, remaining = local.allow(key, limit)
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				tenantLabel := "anonymous"
				if ac != nil && ac.TenantID != "" {
					tenantLabel = ac.TenantID
				}
				rateLimitRejectedTotal.WithLabelValues(tenantLabel).Inc()
				w.Header().Set("Retry-After", "60")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func dualPath(cfg RateLimitConfig, localDual *LocalDualLimiter, ac *auth.AuthContext, w http.ResponseWriter, r *http.Request, next http.Handler) {
	tenantKey := "rl:{" + ac.TenantID + "}"
	userKey := "rl:{" + ac.TenantID + "}:user:" + ac.UserID

	tenantLimit := cfg.DefaultRPM
	if cfg.TenantLimitFn != nil {
		if tl := cfg.TenantLimitFn(ac.TenantID); tl > 0 {
			tenantLimit = tl
		}
	}

	userLimit := cfg.UserRPM
	if userLimit == 0 {
		userLimit = cfg.DefaultRPM
	}
	if cfg.UserLimitFn != nil {
		if ul := cfg.UserLimitFn(ac.TenantID, ac.UserID); ul > 0 {
			userLimit = ul
		}
	}

	allowed, tenantRemaining, userRemaining, err := cfg.DualLimiter.AllowDual(r.Context(), tenantKey, tenantLimit, userKey, userLimit)
	if err != nil {
		observability.ContextLogger(r.Context()).Warn("dual rate limiter failed, falling back to local", "error", err)
		allowed, tenantRemaining, userRemaining, _ = localDual.AllowDual(r.Context(), tenantKey, tenantLimit, userKey, userLimit)
	}

	remaining := min(tenantRemaining, userRemaining)

	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(tenantLimit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

	if !allowed {
		rateLimitRejectedTotal.WithLabelValues(ac.TenantID).Inc()
		w.Header().Set("Retry-After", "60")
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	next.ServeHTTP(w, r)
}

// memoryLimiter is a simple in-memory sliding window fallback with bounded size.
// Used as fallback when distributed limiter (Redis) is unavailable.
type memoryLimiter struct {
	mu      sync.Mutex
	buckets *lru.Cache[string, *bucket]
}

type bucket struct {
	count    int
	limit    int
	windowAt time.Time
}

const maxRateLimitBuckets = 10000

func newMemoryLimiter() *memoryLimiter {
	cache, _ := lru.New[string, *bucket](maxRateLimitBuckets)
	return &memoryLimiter{buckets: cache}
}

func (l *memoryLimiter) allow(key string, limit int) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
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

// DefaultMaxSSEStreamsPerUser is the default maximum concurrent SSE streams per user.
const DefaultMaxSSEStreamsPerUser = 5

// sseConnCount is a simple per-key connection counter using atomic operations.
type sseConnCount struct {
	count atomic.Int32
}

// SSEConnectionTracker tracks active SSE connections per user.
// Uses sync.Map of atomic counters for lock-free concurrent access.
// Safe for concurrent use from multiple goroutines.
type SSEConnectionTracker struct {
	// MaxSSEStreamsPerUser is the maximum concurrent SSE streams allowed per user key.
	// A value <= 0 disables SSE connection limiting.
	MaxSSEStreamsPerUser int
	counts              sync.Map // map[string]*sseConnCount
}

// NewSSEConnectionTracker creates a new tracker with the given per-user limit.
// If maxPerUser <= 0, DefaultMaxSSEStreamsPerUser is used.
func NewSSEConnectionTracker(maxPerUser int) *SSEConnectionTracker {
	if maxPerUser <= 0 {
		maxPerUser = DefaultMaxSSEStreamsPerUser
	}
	return &SSEConnectionTracker{
		MaxSSEStreamsPerUser: maxPerUser,
	}
}

func (t *SSEConnectionTracker) getOrCreate(key string) *sseConnCount {
	val, loaded := t.counts.LoadOrStore(key, &sseConnCount{})
	if loaded {
		return val.(*sseConnCount)
	}
	return val.(*sseConnCount)
}

// IncrSSEConnections atomically increments the connection count for the given key.
// Returns the new count after increment.
func (t *SSEConnectionTracker) IncrSSEConnections(key string) int {
	c := t.getOrCreate(key)
	return int(c.count.Add(1))
}

// DecrSSEConnections atomically decrements the connection count for the given key.
// Entries are never removed from the map to avoid TOCTOU races between
// concurrent Incr and Decr operations. The sync.Map handles memory efficiently.
// Returns the new count after decrement.
func (t *SSEConnectionTracker) DecrSSEConnections(key string) int {
	c := t.getOrCreate(key)
	newCount := c.count.Add(-1)
	if newCount < 0 {
		// Guard against underflow from mismatched Incr/Decr calls.
		c.count.CompareAndSwap(newCount, 0)
		return 0
	}
	return int(newCount)
}

// ActiveSSEConnections returns the current active connection count for the given key.
func (t *SSEConnectionTracker) ActiveSSEConnections(key string) int {
	val, ok := t.counts.Load(key)
	if !ok {
		return 0
	}
	return int(val.(*sseConnCount).count.Load())
}
