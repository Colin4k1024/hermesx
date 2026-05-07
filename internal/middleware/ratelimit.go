package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/observability"
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
}

// RateLimitMiddleware applies per-tenant rate limiting.
// For authenticated requests with a UserID and a configured DualLimiter,
// it atomically checks both tenant and user limits.
// Falls back to local in-memory limiter if the distributed limiter errors.
func RateLimitMiddleware(cfg RateLimitConfig) Middleware {
	local := newLocalLimiter()
	localDual := NewLocalDualLimiter()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := auth.FromContext(r.Context())
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

// localLimiter is a simple in-memory sliding window fallback with bounded size.
type localLimiter struct {
	mu      sync.Mutex
	buckets *lru.Cache[string, *bucket]
}

type bucket struct {
	count    int
	limit    int
	windowAt time.Time
}

const maxRateLimitBuckets = 10000

func newLocalLimiter() *localLimiter {
	cache, _ := lru.New[string, *bucket](maxRateLimitBuckets)
	return &localLimiter{buckets: cache}
}

func (l *localLimiter) allow(key string, limit int) (bool, int) {
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
