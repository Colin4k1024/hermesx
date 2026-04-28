# SaaS Hardening Plan — Concurrency / Isolation / Observability

## Metadata

| Field | Value |
|-------|-------|
| Created | 2026-04-28 |
| Baseline | commit `4d8dab0` (post Phase 0-5 closeout) |
| Scope | 3 dimensions: Concurrency Safety, Data Isolation, Enterprise Tracing |
| Goal | Production-grade multi-tenant SaaS with enterprise observability |
| Status | Plan Only — not yet implemented |

---

## Current Assessment

| Dimension | Score | Verdict |
|-----------|-------|---------|
| Concurrency Safety | 2.5/5 | 2 CRITICAL + 3 HIGH |
| Data Isolation | 4.0/5 | 1 HIGH cross-tenant vulnerability |
| Enterprise Tracing | 1.5/5 | No distributed tracing, sparse structured logging |

---

## Phase 1: Concurrency Safety Fixes (CRITICAL + HIGH)

### 1.1 [CRITICAL] Config Reload Race — sync.Once Reset

**File**: `internal/config/config.go:145-234`

**Problem**: `Reload()` resets `sync.Once` via reassignment (`configOnce = sync.Once{}`), which is not atomic. Concurrent `Load()` and `Reload()` calls corrupt `globalConfig`.

**Fix**:
- Replace `sync.Once` + reassignment with `sync.RWMutex` guarding `globalConfig`
- `Load()` takes read lock; `Reload()` takes write lock
- Remove `configOnce` entirely

**Acceptance Criteria**:
- `go test -race` passes with concurrent Load/Reload goroutines
- No `sync.Once` reassignment in config package

### 1.2 [CRITICAL] Profile Override Race — sync.Once Reset

**File**: `internal/config/profiles.go:17-198`

**Problem**: `SetActiveProfile()` and `OverrideActiveProfile()` both reset `activeProfileOnce = sync.Once{}` and write `profileOverride` without synchronization.

**Fix**:
- Use a single `sync.RWMutex` shared with config (or a dedicated one for profile state)
- Protect `profileOverride`, `activeProfile` reads/writes under the lock
- Remove all `sync.Once` reassignments

**Acceptance Criteria**:
- `go test -race` passes with concurrent SetActiveProfile/OverrideActiveProfile/loadActiveProfile
- Zero `sync.Once` reassignment in profiles package

### 1.3 [HIGH] Memory Function Pointer — Unsynchronized Write

**File**: `internal/tools/memory.go:73-81`

**Problem**: `SetMemoryProviderNameFunc()` writes a function pointer without lock. Concurrent reads in `getMemoryProvider()` (which is mutex-protected) can still race with the write.

**Fix**:
- Protect `getMemoryProviderName` assignment with `activeProviderMu` lock
- Or use `atomic.Pointer[func() string]` (Go 1.19+)

**Acceptance Criteria**:
- `go test -race` passes

### 1.4 [HIGH] Rate Limiter Bucket Map — Unbounded Growth

**File**: `internal/middleware/ratelimit.go:80-112`

**Problem**: `localLimiter.buckets` map grows without bound. Each unique IP/tenant key allocates a bucket that is never deleted, only overwritten on window reset. DoS vector in multi-tenant deployment.

**Fix**:
- Add a periodic GC goroutine (e.g., every 5 minutes) that evicts buckets older than 2x window duration
- Or use a fixed-size LRU cache with TTL (e.g., `hashicorp/golang-lru` or manual eviction)
- Cap maximum bucket count as a safety valve

**Acceptance Criteria**:
- Bucket count stabilizes under sustained load with rotating keys
- Memory usage bounded after 10K unique keys

### 1.5 [HIGH] profileOverride Global — Unprotected Read/Write

**File**: `internal/config/profiles.go:188-193`

**Problem**: `profileOverride` string is written without lock in `OverrideActiveProfile()` and read without lock in `loadActiveProfile()`.

**Fix**: Covered by 1.2 — same mutex protects all profile state.

---

## Phase 2: Data Isolation Fixes (HIGH + MEDIUM)

### 2.1 [HIGH] API Key Lookup Without Tenant Filter

**File**: `internal/store/pg/apikey.go:26-47`

**Problem**: `GetByHash()` and `GetByID()` query API keys globally without `WHERE tenant_id = $N`. An attacker who guesses or brute-forces a key ID/hash can impersonate another tenant.

**Fix**:
- `GetByHash()`: Add `tenant_id` parameter is not practical here (the hash lookup is the authentication step itself — tenant is unknown before lookup). Instead, **this is acceptable by design** for authentication: the key's embedded `tenant_id` determines the caller's tenant. However, add a post-lookup validation: after the auth middleware resolves tenant from the key, all subsequent operations MUST use that resolved tenant_id, never a user-supplied one.
- `GetByID()`: Add `tenant_id` parameter to signature and SQL `WHERE tenant_id = $1 AND id = $2`. Callers (admin API) must pass the authenticated tenant.
- Update `store.APIKeyStore` interface accordingly.
- Update `internal/api/apikeys.go` handler to always pass `tenantID` from middleware context.

**Acceptance Criteria**:
- `GetByID(ctx, tenantID, id)` — interface enforces tenant scope
- Admin API key endpoints always scope by authenticated tenant
- Unit test: tenant A cannot retrieve tenant B's key by ID

### 2.2 [MEDIUM] API Key Revoke Without Tenant Filter

**File**: `internal/store/pg/apikey.go:74-80`

**Problem**: `Revoke(ctx, id)` updates any key by ID regardless of tenant.

**Fix**:
- Change signature to `Revoke(ctx, tenantID, id)` with `WHERE tenant_id = $1 AND id = $2`
- Update `store.APIKeyStore` interface
- Update handler to pass tenant from context

**Acceptance Criteria**:
- Tenant A cannot revoke tenant B's key
- Unit test covers cross-tenant revoke rejection

### 2.3 [MEDIUM] LLM Credential Isolation

**Problem**: All tenants share a single LLM API key from global config. No per-tenant billing, quota, or revocation.

**Fix (Phase 2 scope = design only, implementation deferred)**:
- Add `llm_api_key` column to `tenants` table (encrypted at rest)
- Agent creation reads tenant-specific LLM key if present, falls back to global
- Track per-tenant token usage in `sessions` table (fields already exist: `input_tokens`, `output_tokens`)
- Deferred: per-tenant billing integration

**Acceptance Criteria (design)**:
- Schema migration adds encrypted credential column
- Agent factory checks tenant config before global config

---

## Phase 3: Enterprise Observability (HIGH + MODERATE)

### 3.1 [HIGH] OpenTelemetry Integration

**Problem**: Zero distributed tracing. Cannot correlate requests across service boundaries.

**Fix**:
- Add `go.opentelemetry.io/otel` and `go.opentelemetry.io/otel/sdk` dependencies
- Create `internal/observability/tracer.go`:
  - Initialize TracerProvider with OTLP exporter (configurable endpoint)
  - Support Jaeger/OTLP/stdout exporters via env var `OTEL_EXPORTER_OTLP_ENDPOINT`
- Add tracing middleware to `internal/middleware/chain.go`:
  - Create root span per HTTP request
  - Inject trace_id, span_id into context
  - Record HTTP method, path, status, latency as span attributes
- Propagate spans to:
  - PG queries (pgx tracer hook)
  - LLM API calls (wrap HTTP client with otel transport)
  - MinIO operations
- Export `trace_id` in response header `X-Trace-ID`

**Acceptance Criteria**:
- `docker compose up` with Jaeger shows request traces
- Traces include: HTTP handler → PG query → LLM call spans
- trace_id visible in response headers

### 3.2 [HIGH] LLM Call Observability

**File**: `internal/llm/client.go`, `internal/agent/agent.go`

**Problem**: LLM calls log nothing — no latency, token counts, cost, or errors per call.

**Fix**:
- After each LLM API call, log structured entry:
  ```
  slog.Info("llm_call",
      "trace_id", traceID,
      "tenant_id", tenantID,
      "session_id", sessionID,
      "model", model,
      "input_tokens", usage.InputTokens,
      "output_tokens", usage.OutputTokens,
      "cache_read_tokens", usage.CacheReadTokens,
      "latency_ms", elapsed.Milliseconds(),
      "cost_usd", estimatedCost,
      "error", errMsg,
  )
  ```
- Add Prometheus histograms:
  - `hermes_llm_request_duration_seconds{model, tenant_id}`
  - `hermes_llm_tokens_total{model, tenant_id, direction}` (input/output)

**Acceptance Criteria**:
- Every LLM call produces a structured log line with all fields
- Prometheus metrics queryable by tenant and model

### 3.3 [MODERATE] Structured Logging Enrichment

**Problem**: 310 slog calls, only 9 include context identifiers. Cannot correlate logs to tenant/request/session.

**Fix**:
- Create `internal/observability/logger.go`:
  - `ContextLogger(ctx) *slog.Logger` — extracts request_id, tenant_id, session_id, trace_id from context and returns `slog.With(...)` logger
- Add logging middleware that creates enriched logger and injects into context:
  ```go
  logger := slog.With(
      "request_id", requestID,
      "tenant_id", tenantID,
      "trace_id", traceID,
  )
  ctx = WithLogger(ctx, logger)
  ```
- Migrate high-value log sites (errors, warnings, API calls) to use `observability.ContextLogger(ctx)` instead of bare `slog.*`
- Priority order: error paths first, then middleware, then handlers, then internal packages

**Acceptance Criteria**:
- All `slog.Error` and `slog.Warn` calls include request_id + tenant_id
- Log search by request_id returns complete request lifecycle

### 3.4 [MODERATE] Database Query Observability

**Problem**: No query logging, latency tracking, or slow query detection.

**Fix**:
- Use pgx tracer interface (`pgx.QueryTracer`) to intercept all queries:
  - Log query text (sanitized), duration, row count at DEBUG level
  - Log slow queries (>500ms) at WARN level
  - Record Prometheus histogram `hermes_pg_query_duration_seconds{operation}`
- Expose pgxpool stats via Prometheus: idle connections, total connections, wait count

**Acceptance Criteria**:
- Slow queries (>500ms) appear in logs with tenant context
- `hermes_pg_query_duration_seconds` visible in /metrics

### 3.5 [MODERATE] Audit Trail Enhancement

**File**: `internal/middleware/audit.go`, `internal/store/pg/auditlog.go`

**Problem**: Audit log missing request_id, session_id, response status code, latency.

**Fix**:
- Add columns to `audit_logs` table: `request_id`, `session_id`, `status_code`, `latency_ms`
- Migration: `ALTER TABLE audit_logs ADD COLUMN request_id TEXT, ADD COLUMN session_id TEXT, ADD COLUMN status_code INT, ADD COLUMN latency_ms INT`
- Update `AuditLog` struct and `Append()` to populate new fields
- Update audit middleware to capture response status via `ResponseWriter` wrapper

**Acceptance Criteria**:
- Audit query by request_id returns full action trail
- Status code and latency recorded for every audited request

### 3.6 [LOW] Prometheus Tenant Labels

**Problem**: HTTP metrics have no tenant_id label — cannot monitor per-tenant.

**Fix**:
- Add `tenant_id` label to `hermes_http_requests_total` and `hermes_http_request_duration_seconds`
- Extract from context after auth middleware runs
- Use label value `"anonymous"` for unauthenticated requests
- Note: high-cardinality risk — consider bounded tenant set or use exemplars instead for large deployments

**Acceptance Criteria**:
- Grafana dashboard can filter by tenant_id

---

## Implementation Priority

| Order | Phase | Items | Effort | Risk if Skipped |
|-------|-------|-------|--------|-----------------|
| 1 | P1 Concurrency | 1.1, 1.2 (CRITICAL) | 1 day | Data corruption under load |
| 2 | P2 Isolation | 2.1, 2.2 (HIGH) | 0.5 day | Cross-tenant data access |
| 3 | P1 Concurrency | 1.3, 1.4, 1.5 (HIGH) | 1 day | Memory leak / DoS |
| 4 | P3 Observability | 3.3 (structured logging) | 1 day | Cannot debug production issues |
| 5 | P3 Observability | 3.2 (LLM observability) | 0.5 day | Blind to LLM cost/errors |
| 6 | P3 Observability | 3.1 (OpenTelemetry) | 2 days | No distributed tracing |
| 7 | P3 Observability | 3.4, 3.5 (DB + audit) | 1 day | No query-level visibility |
| 8 | P2 Isolation | 2.3 (LLM credentials) | 1 day | Shared billing |
| 9 | P3 Observability | 3.6 (tenant metrics) | 0.5 day | No per-tenant monitoring |

**Total Estimated Effort**: ~8.5 days

---

## Target Score After Completion

| Dimension | Before | After |
|-----------|--------|-------|
| Concurrency Safety | 2.5/5 | 4.5/5 |
| Data Isolation | 4.0/5 | 4.5/5 |
| Enterprise Tracing | 1.5/5 | 4.0/5 |
| **Overall SaaS Readiness** | **~3.9/5** | **~4.5/5** |

---

## Out of Scope (Deferred)

- Per-tenant LLM billing integration (requires external billing system)
- Multi-region deployment / data residency
- Automated key rotation
- Runtime log level reconfiguration endpoint
- Row-Level Security (RLS) in PostgreSQL
