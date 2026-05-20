# Architecture Overview

> HermesX system design, two run modes, and SaaS multi-tenant architecture.

## Two Run Modes

Hermes supports two independent run modes:

| Mode | Command | Use Case | Storage |
|------|---------|----------|---------|
| CLI Mode | `hermes` | Local interactive Agent | SQLite / filesystem |
| SaaS Mode | `hermes saas-api` | Multi-tenant HTTP API service | PostgreSQL |

Both modes share the LLM integration layer and Skills system, but have completely independent networking, storage, and authentication systems.

## SaaS Mode Architecture

```
┌─────────────────────────────────────────────────┐
│                   HTTP Request                    │
└───────────────────────┬─────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────┐
│              CORS Middleware                      │
│         (configured via SAAS_ALLOWED_ORIGINS)    │
└───────────────────────┬─────────────────────────┘
                        │
                   ┌────▼────┐
                   │ SPA Router│ ←── Static files (index.html, admin.html)
                   └────┬────┘
                        │
┌───────────────────────▼─────────────────────────┐
│          9-Layer Middleware Stack (fixed order)   │
│                                                   │
│  Tracing → Metrics → RequestID → Auth → Tenant   │
│  → Logging → Audit → RBAC → RateLimit → Handler  │
└───────────────────────┬─────────────────────────┘
                        │
          ┌─────────────▼─────────────┐
          │        Route Dispatch      │
          │                           │
          │  /health/*    → Public     │
          │  /metrics     → Prometheus │
          │  /v1/tenants  → Admin      │
          │  /v1/api-keys → Admin      │
          │  /v1/chat/*   → User       │
          │  /v1/me       → User       │
          └─────────┬─────────────────┘
                    │
     ┌──────────────▼──────────────┐
     │        Store Layer           │
     │   PostgreSQL (multi-tenant)  │
     └─────────────────────────────┘
```

## Middleware Stack

Middleware executes in fixed order, enforced by `middleware.MiddlewareStack`:

```
Tracing → Metrics → RequestID → Auth → Tenant → Logging → Audit → RBAC → RateLimit → Handler
```

| Layer | Responsibility | Source File |
|-------|---------------|-------------|
| **Tracing** | OpenTelemetry span creation and propagation | `internal/observability/tracer.go` |
| **Metrics** | Prometheus request counts, latency, concurrency | `internal/middleware/metrics.go` |
| **RequestID** | Generate or extract `X-Request-ID` | `internal/middleware/requestid.go` |
| **Auth** | Chain authentication (Static Token → API Key → JWT) | `internal/auth/extractor.go` |
| **Tenant** | Extract tenant_id from AuthContext into Context | `internal/middleware/tenant.go` |
| **Logging** | Inject tenant_id and request_id into slog Logger | `internal/observability/logger.go` |
| **Audit** | Record all authenticated requests to audit_logs table | `internal/middleware/audit.go` |
| **RBAC** | Role-based access control by path prefix | `internal/middleware/rbac.go` |
| **RateLimit** | Per-tenant rate limiting (distributed + local LRU fallback) | `internal/middleware/ratelimit.go` |

**Design notes**:
- Logging layer is placed after Auth + Tenant, ensuring logs automatically include tenant_id
- Auth errors use `ContextLogger` fallback (slog.Default + request_id)
- All middleware slots can be `nil` (passthrough) for testing and optional enabling

## Multi-Tenant Model

### Tenant Isolation

Hermes uses **database-level tenant isolation**:

1. **tenant_id derived from credentials**: never read from request headers, preventing tenant spoofing
2. **All tables FK to tenants**: 11 business tables include `tenant_id UUID NOT NULL REFERENCES tenants(id)`
3. **Automatic query filtering**: all Store methods filter data by tenant_id
4. **RLS FORCE** — PostgreSQL Row-Level Security policies (including INSERT/UPDATE/DELETE write policies); scheduler write operations use `SET LOCAL app.current_tenant` for RLS compatibility

```
AuthContext.TenantID (derived from credential)
       │
       ▼
┌──────────────┐
│ Context Prop. │ ← written by Tenant middleware
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  Store Query  │ ← WHERE tenant_id = $1
└──────────────┘
```

### Default Tenant

Created automatically on first startup:

- ID: `00000000-0000-0000-0000-000000000001`
- Name: `Default Tenant`
- Plan: `pro`
- Rate Limit: 120 RPM
- Max Sessions: 10

Static Token authentication automatically maps to this tenant.

## Store Architecture

### Interface Abstraction

The `store.Store` interface defines a unified data access layer supporting multiple backend implementations:

```go
type Store interface {
    Tenants()      TenantStore
    Sessions()     SessionStore
    Messages()     MessageStore
    Users()        UserStore
    AuditLogs()    AuditLogStore
    APIKeys()      APIKeyStore
    CronJobs()     CronJobStore
    Memories()     MemoryStore
    UserProfiles() UserProfileStore
    Roles()        RoleStore
    PricingRules() PricingRuleStore
}
```

### Storage Backends

| Backend | Use Case | Package Path |
|---------|----------|-------------|
| PostgreSQL | SaaS mode (multi-tenant) | `internal/store/pg/` |
| SQLite | CLI mode (single user) | `internal/store/sqlite/` |

The PostgreSQL backend automatically runs 106+ migrations on startup (including RLS policies, pricing_rules, sandbox_policy, cron scheduler, etc.).

## Distributed Cron Scheduler

The SaaS mode scheduled task execution is implemented by the `internal/scheduler/` package, supporting multi-pod distributed execution:

```
┌──────────────────────────────────────────┐
│          SaasScheduler                    │
│                                          │
│  ┌──────────┐    ┌──────────────────┐   │
│  │ pollLoop │───▶│  syncOnce (PG)   │   │
│  │ (30s)    │    │  ListAllEnabled  │   │
│  └──────────┘    └────────┬─────────┘   │
│                           │              │
│                  ┌────────▼─────────┐   │
│                  │  gocron Scheduler │   │
│                  │  (CronJob defs)   │   │
│                  └────────┬─────────┘   │
│                           │              │
│                  ┌────────▼─────────┐   │
│                  │    execute()      │   │
│                  │  execWithTenant   │   │
│                  └────────┬─────────┘   │
│                           │              │
└───────────────────────────┼──────────────┘
                            │
          ┌─────────────────┼─────────────────┐
          │                 │                  │
   ┌──────▼──────┐  ┌──────▼──────┐  ┌───────▼───────┐
   │ Redis Lock  │  │ AgentRunner │  │ ResultDeliverer│
   │ (mutual ex.)│  │ (AI conv.)  │  │ (result push) │
   └─────────────┘  └─────────────┘  └───────────────┘
```

**Key Design Decisions:**

| Mechanism | Description |
|-----------|-------------|
| PG poll sync | Every 30s pulls all enabled jobs from `cron_jobs` table, diffs against in-memory state for add/remove/update |
| Redis distributed lock | `redislock.WithTries(1)` no-retry competition; pods that lose the race skip the execution |
| Idempotent execution | `ON CONFLICT (cron_job_id, scheduled_at) DO NOTHING`; PG unique constraint guarantees at-most-once |
| Tenant isolation | `execWithTenant()` uses `SET LOCAL app.current_tenant` within a transaction, compatible with FORCE RLS |
| Cross-tenant cleanup | `scheduler_cleanup_stale_runs()` SECURITY DEFINER function bypasses RLS |
| Result delivery | After execution completes, `ResultDeliverer` pushes results back to the user's source platform |
| Lifecycle | Scheduler owns an independent ctx/cancelFunc; Stop() drains gocron first then cancels the context |

## LLM Integration

### Pluggable Transport

LLM calls are abstracted through the `Transport` interface, supporting multiple providers:

```
Request → FallbackRouter → RetryTransport → CircuitBreaker → Provider Transport → LLM API
```

Supported providers:
- OpenAI (openai protocol)
- Anthropic (anthropic protocol, with prompt caching)
- Auto-detect (inferred from API URL and key format)

### Resilience Mechanisms

| Component | Responsibility | Package Path |
|-----------|---------------|-------------|
| **FallbackRouter** | Auto-switch to backup provider on primary failure | `internal/llm/fallback_router.go` |
| **RetryTransport** | Exponential backoff retry + ±25% jitter | `internal/llm/retry_transport.go` |
| **Circuit Breaker** | Per-model independent circuit breaking, Prometheus metrics | `internal/llm/breaker.go` |
| **Model Catalog** | Hot-reload model registry with capability metadata | `internal/llm/model_catalog.go` |
| **Multimodal Router** | Dispatch image/audio/video by provider capability | `internal/agent/multimodal.go` |

### Configuration

| Environment Variable | Description |
|---------------------|-------------|
| `LLM_API_URL` | LLM API endpoint |
| `LLM_API_KEY` | API authentication key |
| `LLM_MODEL` | Default model name |
| `LLM_FALLBACK_API_URL` | Fallback provider endpoint |
| `LLM_FALLBACK_API_KEY` | Fallback provider key |
| `OIDC_ISSUER_URL` | OIDC provider URL (activates SSO when set) |

## Chat Request Flow

A complete `/v1/chat/completions` request traverses the following path:

```
1. HTTP request arrives
2. CORS check (if SAAS_ALLOWED_ORIGINS is configured)
3. Tracing: create span
4. Metrics: record request start
5. RequestID: generate unique request identifier
6. Auth: chain authentication, produce AuthContext
7. Tenant: extract tenant_id from AuthContext
8. Logging: inject tenant_id into logger
9. Audit: record audit log (action, status_code, latency_ms)
10. RBAC: check "user" role permission
11. RateLimit: check tenant RPM quota
12. Handler:
    a. Parse ChatCompletionRequest
    b. Get or create Session (isolated by tenant_id)
    c. Call LLM API (via Transport)
    d. Store Message (linked to tenant_id)
    e. Return ChatCompletionResponse
```

## HTTP Server Configuration

| Parameter | Value | Description |
|-----------|-------|-------------|
| Read Timeout | 30s | Request body read timeout |
| Write Timeout | 60s | Response write timeout (LLM calls may be slow) |
| Idle Timeout | 120s | Keep-alive idle timeout |
| Listen Address | `0.0.0.0:{port}` | Listen on all interfaces |

## Project Structure

```
hermesx/
├── cmd/hermes/
│   ├── main.go           # CLI entry point
│   └── saas.go           # SaaS API entry point (hermes saas-api)
├── internal/
│   ├── api/              # HTTP handlers and routing
│   │   ├── server.go     # APIServer, route registration, middleware assembly
│   │   ├── tenants.go    # Tenant CRUD
│   │   ├── apikeys.go    # API Key management
│   │   ├── mockchat.go   # Chat completions + session store
│   │   ├── health.go     # Health probes
│   │   └── gdpr.go       # GDPR data export/deletion
│   ├── auth/             # Authentication chain
│   │   ├── context.go    # AuthContext definition
│   │   ├── extractor.go  # ExtractorChain interface
│   │   ├── static.go     # Static Token authentication
│   │   ├── apikey.go     # API Key authentication
│   │   └── jwt.go        # JWT authentication (reserved)
│   ├── middleware/        # HTTP middleware
│   │   ├── chain.go      # Fixed-order middleware stack
│   │   ├── rbac.go       # Role-based access control
│   │   ├── ratelimit.go  # Rate limiting
│   │   ├── metrics.go    # Prometheus metrics
│   │   └── audit.go      # Audit logging
│   ├── store/            # Data storage abstraction
│   │   ├── types.go      # Data model definitions
│   │   ├── pg/           # PostgreSQL implementation
│   │   └── sqlite/       # SQLite implementation
│   ├── skills/           # Skills system
│   │   ├── hub.go        # Skills Hub discovery and installation
│   │   ├── scanner.go    # Security scanning
│   │   └── loader.go     # Local loading
│   ├── observability/    # Observability
│   │   ├── tracer.go     # OpenTelemetry initialization
│   │   └── logger.go     # Context-enriched logging
│   ├── objstore/         # MinIO/S3 object storage
│   ├── gateway/          # CLI mode Gateway, media dispatch, lifecycle hooks
│   │   └── platforms/    # 15 platform adapters + registry
│   ├── config/           # Configuration management
│   └── dashboard/        # Admin panel static files
│       └── static/       # HTML/CSS/JS
├── skills/               # 81 built-in Skills
├── deploy/               # Deployment configuration
│   ├── helm/             # Helm Chart
│   └── kind/             # Kind local K8s
├── scripts/              # Test and utility scripts
└── docs/                 # Documentation
```

## v1.4.0 New Modules (Upstream v2026.4.30 Absorption)

### Agent Layer

| Module | Responsibility | Source File |
|--------|---------------|-------------|
| **Memory Curator** | Autonomous deduplication, LLM merging, expiry cleanup | `internal/agent/curator.go` |
| **Self-improvement** | Periodic LLM conversation self-evaluation + insight persistence | `internal/agent/self_improve.go` |
| **Multimodal Router** | Dispatch image/audio/video requests by provider capability | `internal/agent/multimodal.go` |
| **Compress** | Context compression (auto-summarize near token limit) | `internal/agent/compress.go` |

### Gateway Layer

| Module | Responsibility | Source File |
|--------|---------------|-------------|
| **Media Dispatcher** | Platform-capability-aware media routing + fallback chain | `internal/gateway/media_dispatch.go` |
| **Lifecycle Hooks** | Priority-ordered event hooks (RWMutex concurrency-safe) | `internal/gateway/lifecycle_hooks.go` |
| **Platform Registry** | Platform registration and capability declaration | `internal/gateway/registry.go` |

### LLM Layer

| Module | Responsibility | Source File |
|--------|---------------|-------------|
| **Model Catalog** | Hot-reload model registry + capability metadata | `internal/llm/model_catalog.go` |

### Store Layer

| Module | Responsibility | Source File |
|--------|---------------|-------------|
| **Trigram Search** | pg_trgm CJK fuzzy search | `internal/store/pg/trigram_search.go` |

## Related Documentation

- [Getting Started](saas-quickstart.md) — 5-minute quickstart
- [API Reference](api-reference.md) — Complete endpoint documentation
- [Authentication](authentication.md) — Auth Chain and RBAC
- [Database](database.md) — Schema and data models
- [Observability](observability.md) — Monitoring and tracing
- [Skills Guide](skills-guide.md) — Skills system
- [Configuration Guide](configuration.md) — Environment variables
- [Deployment Guide](deployment.md) — Docker / Helm / Kind
- [Scheduler Guide](scheduler-guide.en.md) — Distributed scheduler deployment & testing
