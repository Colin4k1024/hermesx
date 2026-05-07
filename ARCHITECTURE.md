# Architecture

> hermes-agent-go — Enterprise Agent Runtime  
> Single binary, multi-tenant, auditable execution

---

## System Overview

```
                        ┌──────────────────────┐
                        │      Clients         │
                        │  (API / SDK / UI)    │
                        └──────────┬───────────┘
                                   │ HTTPS
                        ┌──────────▼───────────┐
                        │    API Server         │
                        │  (net/http, Go 1.25)  │
                        └──────────┬───────────┘
                                   │
              ┌────────────────────▼────────────────────┐
              │          Middleware Stack (fixed order)  │
              │                                         │
              │  Tracing → Metrics → RequestID → Auth   │
              │  → Tenant → Logging → Audit → RBAC     │
              │  → RateLimit → Handler                  │
              └────────────────────┬────────────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │       Agent Runtime          │
                    │                              │
                    │  LLM Client ←→ Tool Loop     │
                    │  Soul / Skills / Memory      │
                    │  Multimodal Router           │
                    │  Context Compress            │
                    └──────┬───────────┬───────────┘
                           │           │
              ┌────────────▼──┐  ┌─────▼─────────────┐
              │ Tool Runtime   │  │  Skill Runtime    │
              │ (Sandbox)      │  │  (MinIO loader)   │
              └────────────────┘  └───────────────────┘
                           │
         ┌─────────────────▼─────────────────────────┐
         │              Infrastructure                 │
         │                                            │
         │  PostgreSQL   Redis   MinIO   OTel/Prom    │
         └────────────────────────────────────────────┘
```

---

## Why Go?

| Decision | Rationale |
|----------|-----------|
| **Single binary** | 零依赖部署，Docker 镜像 < 50MB（debian-slim base） |
| **goroutine 并发** | 每个 agent session 独立 goroutine，无线程池管理开销 |
| **类型安全** | 编译时捕获 Store 接口不一致、中间件顺序错误 |
| **性能** | 7.97 req/s 单实例（MiniMaxi LLM 瓶颈），Go 本身不是瓶颈 |
| **生态** | pgx (PG), go-redis, gobreaker, OTel SDK, Prometheus 均为一等公民 |
| **交叉编译** | `GOOS=linux GOARCH=arm64` 直接构建，无需 Docker buildx |

---

## Why Single Binary?

两种运行模式共享一个可执行文件：

```
hermes              → CLI 模式 (SQLite, 本地交互)
hermes saas-api     → SaaS 模式 (PostgreSQL, 多租户 HTTP)
```

**优势：**
- 统一构建和测试流程
- 共享 LLM 层和 Skills 系统
- 运维简单：一个镜像、一个 Dockerfile、一组 env vars

---

## Why PostgreSQL?

| Requirement | PostgreSQL Feature |
|-------------|-------------------|
| Multi-tenant isolation | Row-Level Security (RLS) |
| ACID transactions | 事务性 GDPR 删除 |
| Schema evolution | 70+ idempotent migrations (PL/pgSQL DO blocks) |
| Full-text search | pg_trgm + GIN index (CJK trigram) |
| JSON flexibility | JSONB columns (sandbox_policy, metadata) |
| Audit immutability | BEFORE UPDATE/DELETE trigger |
| PITR backup | pgBackRest (RPO < 5min) |

---

## Why Redis?

| Use Case | Implementation |
|----------|---------------|
| Distributed rate limiting | Lua atomic script (tenant + user key) |
| Session lock | SETNX with TTL |
| Context cache | Compressed conversation window |
| Pairing state | Cross-platform pairing codes |
| Instance status | Heartbeat TTL |

**Degradation:** Redis 不可用时，`LocalDualLimiter` 以本地 LRU 降级（精确性降低但不阻塞服务）。

---

## Why MinIO?

| Use Case | Key Pattern |
|----------|-------------|
| Soul templates | `{tenant_id}/soul/SOUL.md` |
| Skill files | `{tenant_id}/{skill_name}/SKILL.md` |
| User uploads | `{tenant_id}/uploads/{file_id}` |

**Why not PG BYTEA?** 对象存储解耦计算与存储，支持 CDN 加速和独立备份策略。

---

## Middleware Stack

固定顺序，由 `middleware.MiddlewareStack` 编译时保证：

```
 1. Tracing      — OTel span creation, W3C context propagation
 2. Metrics      — Prometheus counter/histogram/gauge
 3. RequestID    — Generate or extract X-Request-ID
 4. Auth         — Chain: Static → API Key → JWT → OIDC
 5. Tenant       — Derive tenant_id from AuthContext (NEVER from header)
 6. Logging      — Inject tenant_id + request_id into slog
 7. Audit        — Record to audit_logs (immutable)
 8. RBAC         — method×path role check, admin bypass
 9. RateLimit    — Redis Lua atomic counter, 120 RPM default
10. Handler      — Business logic
```

**Design Invariant:** Logging 在 Auth + Tenant 之后，确保日志始终包含 tenant context。

---

## LLM Resilience Stack

```
Request
  → FallbackRouter        (主 provider 故障 → 自动切换备用)
    → RetryTransport      (指数退避 + ±25% jitter, max 3 retries)
      → CircuitBreaker    (per-model, gobreaker v2.4, Prometheus state gauge)
        → Provider Transport
          → LLM API
```

| Component | Failure Mode | Recovery |
|-----------|-------------|----------|
| FallbackRouter | Primary 502/503/timeout | Switch to `LLM_FALLBACK_*` |
| RetryTransport | Transient 429/500/timeout | Exponential backoff (1s → 2s → 4s) |
| CircuitBreaker | 5 consecutive failures | Open for 60s, half-open probe |

**API Mode Detection:**

```
Explicit HERMES_API_MODE env    → use directly
URL contains "anthropic.com"   → anthropic mode
URL contains "minimaxi.com"    → requires explicit mode
Default                        → openai mode
```

---

## Agent Runtime

```
┌─────────────────────────────────────────────────────┐
│                    AIAgent                           │
│                                                     │
│  ┌─────────┐  ┌──────────┐  ┌────────────────┐    │
│  │  Soul    │  │  Skills  │  │  Memory        │    │
│  │  (MinIO) │  │  (MinIO) │  │  (PG + search) │    │
│  └─────────┘  └──────────┘  └────────────────┘    │
│                                                     │
│  ┌─────────────────────────────────────────────┐   │
│  │  Tool Loop (max 50 iterations)              │   │
│  │                                              │   │
│  │  LLM call → parse tool_use → execute tool   │   │
│  │  → append result → LLM call → ...           │   │
│  │                                              │   │
│  │  Termination: no tool_use OR max iterations  │   │
│  └─────────────────────────────────────────────┘   │
│                                                     │
│  ┌──────────────────┐  ┌───────────────────────┐  │
│  │ Context Compress  │  │ Multimodal Router     │  │
│  │ (auto-summarize)  │  │ (image/audio/video)   │  │
│  └──────────────────┘  └───────────────────────┘  │
│                                                     │
│  ┌──────────────────┐  ┌───────────────────────┐  │
│  │ Memory Curator    │  │ Self-Improvement      │  │
│  │ (dedup + merge)   │  │ (periodic self-eval)  │  │
│  └──────────────────┘  └───────────────────────┘  │
└─────────────────────────────────────────────────────┘
```

---

## Observability

```
┌──────────────────────────────────────────────────┐
│  Application                                      │
│                                                   │
│  slog (structured)  ──→  stdout / log collector   │
│  OTel spans         ──→  OTLP endpoint           │
│  Prometheus metrics ──→  /metrics scrape          │
└──────────────────────────────────────────────────┘
         │                    │                │
         ▼                    ▼                ▼
   Log Aggregation     Trace Backend     Prometheus
   (Loki/ELK)         (Jaeger/Tempo)    + Grafana
```

**Key Metrics:**
- `http_requests_total{method, path, status}`
- `http_request_duration_seconds{method, path}`
- `llm_call_duration_seconds{model, tenant_id}`
- `circuit_breaker_state{model}` (0=closed, 1=half-open, 2=open)
- `rate_limit_hits_total{tenant_id}`

---

## Sandbox Isolation

```
┌────────────────────────────────────────┐
│  Agent Tool Call                        │
│                                         │
│  1. Policy Check (AllowedTools)         │
│  2. Max iterations check                │
│  3. Select sandbox mode:                │
│                                         │
│     ┌──────────────┐  ┌─────────────┐  │
│     │ Local Process │  │ Docker      │  │
│     │ - fork/exec   │  │ - OCI image │  │
│     │ - env strip   │  │ - --net=none│  │
│     │ - timeout     │  │ - --memory  │  │
│     │ - stdout cap  │  │ - --cpus    │  │
│     └──────────────┘  └─────────────┘  │
│                                         │
│  4. Capture output (truncate > 50KB)    │
│  5. Return result to Agent              │
└────────────────────────────────────────┘
```

---

## Data Flow: Chat Request

```
POST /v1/agent/chat
     │
     ▼
[Middleware Stack] ─── auth, tenant, audit, rate limit
     │
     ▼
[Handler] ─── parse request, load session from PG
     │
     ▼
[Agent.New()] ─── configure LLM, soul, skills, memory
     │
     ▼
[Agent.RunConversation()] ─── tool loop with LLM
     │
     ├── [Tool Call] ─── sandbox execution
     ├── [Memory Write] ─── PG upsert
     ├── [Skill Load] ─── MinIO fetch
     │
     ▼
[Response] ─── persist to PG, update token counters
     │
     ▼
[SSE Stream OR JSON Response]
```

---

## Deployment Topology

### Docker Compose (Development / Small Production)

```
┌─────────────────────────────────────────────────┐
│  Docker Host                                     │
│                                                  │
│  ┌──────────┐  ┌─────┐  ┌─────┐  ┌──────────┐ │
│  │hermes-saas│  │ PG  │  │Redis│  │  MinIO   │ │
│  │  :18080   │  │:5432│  │:6379│  │:9000/9001│ │
│  └──────────┘  └─────┘  └─────┘  └──────────┘ │
└─────────────────────────────────────────────────┘
```

### Kubernetes (Production)

```
┌─────────────────────────────────────────────────────┐
│  K8s Cluster                                         │
│                                                      │
│  ┌─────────────────────────────────────────────┐    │
│  │  Deployment: hermes-saas                     │    │
│  │  - HPA (CPU 70% / Memory 80%)               │    │
│  │  - PDB (minAvailable: 1)                     │    │
│  │  - Conservative scale-down (300s cooldown)   │    │
│  └─────────────────────────────────────────────┘    │
│                                                      │
│  ┌──────────┐  ┌──────────────┐  ┌────────────┐   │
│  │ PG (HA)  │  │ Redis Cluster │  │ MinIO (HA) │   │
│  │ pgBackRest│  │ 3 replicas   │  │ erasure    │   │
│  └──────────┘  └──────────────┘  └────────────┘   │
└─────────────────────────────────────────────────────┘
```

---

## Project Structure

```
hermes-agent-go/
├── cmd/hermes/             Entry points (CLI + SaaS)
├── internal/
│   ├── agent/              Agent runtime (tool loop, soul, memory, compress)
│   ├── api/                HTTP handlers + routing
│   ├── auth/               Auth chain (static, apikey, jwt, oidc)
│   ├── config/             Configuration management
│   ├── gateway/            CLI gateway + platform adapters
│   ├── llm/                LLM client (retry, breaker, fallback, catalog)
│   ├── metering/           Usage recording
│   ├── middleware/         HTTP middleware (9-layer stack)
│   ├── objstore/           MinIO client
│   ├── observability/      OTel + structured logging
│   ├── skills/             Skill loader + parser + scanner
│   ├── store/              Data access (pg/ + sqlite/)
│   ├── tools/              Tool registry + sandbox
│   └── toolsets/           Tool group management
├── skills/                 126 bundled skills
├── deploy/                 Helm, Kind, PITR, multi-replica
├── scripts/                Tooling (stress test, migration, etc.)
├── tests/                  Integration + E2E tests
└── docs/                   Architecture, API, guides
```

---

## Key Design Decisions

| Decision | Choice | Alternative Considered | Why |
|----------|--------|----------------------|-----|
| Language | Go | Rust, TypeScript | Performance + ecosystem + single binary |
| Store | PostgreSQL | CockroachDB | RLS support, mature ecosystem, operational simplicity |
| Object store | MinIO | S3 directly | Self-hosted, S3-compatible, no vendor lock |
| Rate limiting | Redis Lua | In-memory only | Distributed accuracy across replicas |
| Circuit breaker | gobreaker | Custom | Battle-tested, Prometheus integration |
| Auth | Chain pattern | Single extractor | Flexible enterprise integration |
| Tracing | OpenTelemetry | Datadog/custom | Vendor-neutral, W3C standard |
| Sandbox | Process + Docker | gVisor/Firecracker | Pragmatic balance of isolation and complexity |
