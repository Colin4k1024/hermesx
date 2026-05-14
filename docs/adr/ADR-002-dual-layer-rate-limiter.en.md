# ADR-002: Dual-Layer Rate Limiter Interface

## Decision Info

| Field | Value |
|-------|-------|
| Number | ADR-002 |
| Title | Introduce DualLayerLimiter Interface to Replace Single-Layer RateLimiter |
| Status | Accepted |
| Date | 2026-05-07 |
| Owner | tech-lead |
| Related Requirement | US-09 (User-Level Rate Limiting), P2-S3 |

---

## Background and Constraints

### Current Problem

The existing `RateLimiter` interface signature is:

```go
type RateLimiter interface {
    Allow(key string, limit int) (bool, int, error)
}
```

This interface only supports single-key, single-limit checks. US-09 requires simultaneously checking both tenant-level and user/key-level rate limits, and both checks must be executed atomically — otherwise inconsistent states arise where tenant quota is exhausted but user quota appears available.

### Business Goals

- Enterprise tenants need fair-use guarantees: a single user must not exhaust the entire tenant's API quota
- Dual-layer rate limiting must be atomically determined: one Lua script checks two keys
- Return remaining quota for both layers; HTTP response headers show `min(tenant_remaining, user_remaining)`

### Constraints

- The existing `RateLimiter` interface already has 3 implementations (Redis, local fallback, test mock)
- The existing `RateLimitMiddleware` depends on the `Allow(key, limit)` signature
- Multi-key operations in Redis Cluster mode require hash tags to guarantee same slot
- Local fallback must provide degraded operation when Redis is unavailable

### Non-Goals

- Not changing the semantics of tenant-level rate limiting (keeping ZSET sliding window)
- Not supporting burst mode (deferred to v1.3.0)
- Not introducing external rate limiting services (e.g., envoy ratelimit gRPC)

---

## Alternatives

### Option A: Two Sequential Allow Calls (Non-Atomic)

Call `Allow()` twice: check tenant first, then user.

- **Applicable when**: Non-atomicity is acceptable
- **Pros**: Zero interface changes, simplest implementation
- **Risks**: Race condition — tenant allow + user deny consumes tenant quota but rejects the request; tenant deny + user allow fails to correctly decrement user counter
- **Rejected because**: Inaccurate rate limiting under high concurrency violates US-09 acceptance criteria

### Option B: Modify Existing Allow Signature for Multiple Keys

```go
Allow(keys []string, limits []int) (bool, []int, error)
```

- **Applicable when**: Willing to accept breaking changes for all callers
- **Pros**: Single interface, no interface fragmentation
- **Risks**: All existing callers must be adapted; local fallback complexity increases; single-key scenarios become verbose
- **Rejected because**: Too disruptive, and single-layer callers (anonymous traffic) don't need multi-key semantics

### Option C: New DualLayerLimiter Interface (Adopted)

```go
type DualLayerLimiter interface {
    AllowDual(tenantKey string, tenantLimit int, userKey string, userLimit int) (allowed bool, tenantRemaining int, userRemaining int, err error)
}
```

- **Applicable when**: Authenticated requests requiring atomic dual-layer checking
- **Pros**: Does not break existing interface; old and new can coexist; Lua script encapsulates atomic logic
- **Risks**: Two coexisting interfaces increase cognitive load; need to ensure middleware works correctly with/without DualLayerLimiter
- **Chosen because**: Backward compatibility + atomicity + clear semantics

---

## Decision Outcome

**Adopting Option C: introduce the `DualLayerLimiter` interface.**

### Detailed Design

```go
// DualLayerLimiter checks tenant and user/key rate limits atomically.
type DualLayerLimiter interface {
    AllowDual(tenantKey string, tenantLimit int, userKey string, userLimit int) (allowed bool, tenantRemaining int, userRemaining int, err error)
}
```

### Implementation Strategy

1. **Redis implementation** (`RedisDualLimiter`): single Lua script atomically checks two ZSET keys
   - Key format: `rl:{tenantID}` + `rl:{tenantID}:user:{userID}` or `rl:{tenantID}:key:{keyID}`
   - Hash tag `{tenantID}` guarantees same slot in Redis Cluster
   - Returns `[allowed(0/1), tenant_remaining, user_remaining]`

2. **Local fallback** (`LocalDualLimiter`): two independent LRU sliding windows
   - Not strictly atomic (no intra-process race), but provides best-effort degraded operation

3. **Middleware compatibility**:
   - Authenticated requests: if `DualLayerLimiter` available → use `AllowDual`
   - Anonymous requests / no UserID: fall back to old `RateLimiter.Allow` single-layer check
   - `X-RateLimit-Remaining` = `min(tenantRemaining, userRemaining)`

### Migration Path

```
Phase 2-S3:
  1. Add DualLayerLimiter interface + Redis implementation
  2. Add LocalDualLimiter fallback
  3. RateLimitMiddleware adds optional DualLayerLimiter field
  4. Authenticated requests use AllowDual, anonymous requests use Allow
  5. Old RateLimiter interface is not deleted or modified
```

### Impact Scope

- `internal/middleware/ratelimit.go` — add DualLayerLimiter path
- `internal/middleware/redis_ratelimiter.go` — add RedisDualLimiter
- `internal/middleware/ratelimit_test.go` — add dual-layer tests
- Prometheus metric: add `hermes_rate_limit_rejected_total{tenant_id, layer}` label

### Compatibility

- All existing `RateLimiter.Allow` callers are unaffected
- Anonymous request behavior unchanged
- New middleware configuration is optional — without DualLayerLimiter falls back to single-layer

### Failure / Fallback Strategy

- If the Lua script does not work in Redis Cluster mode: fall back to Option A (two calls + document non-atomicity)
- If local fallback accuracy is insufficient: accept as "best effort", does not block release

---

## Follow-up Actions

| Action | Owner | Completion Criteria |
|--------|-------|---------------------|
| Implement DualLayerLimiter + Redis Lua | backend-engineer | P2-S3 complete |
| Update RateLimitMiddleware | backend-engineer | Authenticated requests use dual-layer path |
| Table-driven test coverage | backend-engineer | ≥8 scenarios |
| Redis Cluster hash tag validation | backend-engineer | Redis Cluster mode test in CI |
