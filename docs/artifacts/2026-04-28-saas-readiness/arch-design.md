# Architecture Design: HermesX — SaaS Readiness

| 字段 | 值 |
|------|-----|
| Slug | `saas-readiness` |
| 日期 | 2026-04-28 |
| 主责 | architect |
| 状态 | draft |
| 阶段 | plan |
| 关联 PRD | `docs/artifacts/2026-04-28-saas-readiness/prd.md` |
| 关联计划 | `docs/artifacts/2026-04-28-saas-readiness/delivery-plan.md` |

---

## 系统边界

### 外部依赖

| 依赖 | 用途 | Phase |
|------|------|-------|
| PostgreSQL | 持久化存储（sessions, messages, users, tenants, audit_logs, api_keys） | 已有 |
| Redis | Session lock（已有）+ Rate limit（P1）+ Token blacklist（P2） | P1 扩展 |
| LLM Provider | Agent 对话（Claude/OpenAI API） | 已有 |
| Vault / AWS SM | JWT 密钥 + API secrets（P5） | P5 |
| Prometheus | Metrics 抓取 | P3 |

### 边界内

- ACP Server（`internal/acp/`）— 管理 API
- API Server（`internal/gateway/platforms/api_server.go`）— OpenAI-compatible 对话 API
- AgentFactory（`internal/agent/factory.go`）— 无状态 Agent 执行
- Store layer（`internal/store/`）— PG/SQLite 双实现
- Middleware chain（`internal/middleware/`）— 跨切面关注点

### 边界外

- 前端 Dashboard
- SSO/OIDC Provider
- Stripe/支付网关
- Kubernetes 集群管理

---

## 组件拆分

### 新增组件概览

```
internal/
├── middleware/          # 中间件链（现有 + 新增）
│   ├── tenant.go        # [修改] Tenant 从 AuthContext 派生
│   ├── auth.go          # [新增 P1] AuthContext 注入 + credential chain
│   ├── rbac.go          # [新增 P1] 角色访问控制
│   ├── ratelimit.go     # [新增 P1] Redis 滑动窗口 + fallback
│   ├── requestid.go     # [新增 P3] Request ID 生成
│   ├── metrics.go       # [新增 P3] Prometheus HTTP metrics
│   └── chain.go         # [新增 P0] 统一 middleware stack builder
│
├── store/
│   ├── store.go         # [修改 P0] 添加 Tenants()/AuditLogs()/APIKeys()
│   ├── types.go         # [修改 P1-P2] 新增 AuditLog/APIKey struct
│   ├── pg/
│   │   ├── pg.go        # [修改 P0] 注册新 sub-store
│   │   ├── migrate.go   # [修改 P0] 版本化迁移
│   │   ├── tenant.go    # [新增 P2] TenantStore PG 实现
│   │   ├── auditlog.go  # [新增 P1] AuditLogStore PG 实现
│   │   └── apikey.go    # [新增 P2] APIKeyStore PG 实现
│   └── sqlite/
│       └── sqlite.go    # [修改 P0] no-op stubs + 编译断言
│
├── auth/                # [新增 P1-P2] 认证模块
│   ├── context.go       # AuthContext struct + context helpers
│   ├── static.go        # Static bearer token extractor
│   ├── jwt.go           # [P2] JWT RS256 签发/验证
│   └── apikey.go        # [P2] API Key 验证
│
├── acp/
│   └── server.go        # [修改 P0] 挂载统一 middleware chain
│
└── gateway/platforms/
    └── api_server.go    # [修改 P0] 挂载统一 middleware chain
```

---

## 关键设计决策

### 决策 1: Store Interface 一次性扩展（CHL-3/CHL-6）

**目标状态（P0 T0b 单次 commit）：**

```go
// internal/store/store.go
type Store interface {
    Sessions()  SessionStore
    Messages()  MessageStore
    Users()     UserStore
    Tenants()   TenantStore    // NEW
    AuditLogs() AuditLogStore  // NEW
    APIKeys()   APIKeyStore    // NEW
    Close() error
    Migrate(ctx context.Context) error
}

type TenantStore interface {
    Create(ctx context.Context, t *Tenant) error
    Get(ctx context.Context, id string) (*Tenant, error)
    Update(ctx context.Context, t *Tenant) error
    Delete(ctx context.Context, id string) error  // soft delete
    List(ctx context.Context, opts ListOptions) ([]*Tenant, int, error)
}

type AuditLogStore interface {
    Append(ctx context.Context, log *AuditLog) error
    List(ctx context.Context, tenantID string, opts AuditListOptions) ([]*AuditLog, int, error)
}

type APIKeyStore interface {
    Create(ctx context.Context, key *APIKey) error
    GetByHash(ctx context.Context, hash string) (*APIKey, error)
    List(ctx context.Context, tenantID string) ([]*APIKey, error)
    Revoke(ctx context.Context, id string) error
}
```

**SQLite stubs：**

```go
// internal/store/sqlite/sqlite.go
type noopTenantStore struct{}
func (n *noopTenantStore) Create(_ context.Context, _ *store.Tenant) error {
    return fmt.Errorf("tenant store not supported in SQLite mode")
}
// ... all methods return errors

var _ store.Store = (*SQLiteStore)(nil)  // compile-time assertion
```

**取舍：** 显式接口方法 vs 注册表模式。选择显式方法——编译时安全性 > 灵活性，且仅 2 个 backend 实现。

---

### 决策 2: Migration 版本化（CHL-1）

**现状问题：** `migrate.go` 用 `[]string` 平铺 `CREATE TABLE IF NOT EXISTS`，无 ALTER 支持。

**方案：** 引入手动版本号方案（不依赖外部库，保持零依赖原则）：

```go
// internal/store/pg/migrate.go
type migration struct {
    version int
    sql     string
}

var migrations = []migration{
    // 现有 DDL（v1-v6）
    {1, `CREATE TABLE IF NOT EXISTS tenants (...)`},
    {2, `CREATE TABLE IF NOT EXISTS sessions (...)`},
    // ...
    {6, `CREATE INDEX IF NOT EXISTS idx_cron_next ...`},
    // P1 新增（v7+）
    {7, `ALTER TABLE users ADD COLUMN IF NOT EXISTS roles TEXT[] DEFAULT '{user}'`},
    {8, `CREATE TABLE IF NOT EXISTS api_keys (...)`},
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
    // 1. CREATE schema_version table if not exists
    // 2. SELECT MAX(version) FROM schema_version
    // 3. Run only migrations > current version
    // 4. INSERT version after each successful migration
}
```

**取舍：** golang-migrate 库 vs 手动方案。选择手动——保持零外部依赖，migration 逻辑简单（< 50 行）。

---

### 决策 3: Middleware Chain 架构（CHL-2/CHL-5/CHL-8）

**统一 middleware stack builder：**

```go
// internal/middleware/chain.go
type MiddlewareStack struct {
    auth       AuthMiddleware
    tenant     TenantMiddleware
    rbac       RBACMiddleware
    rateLimit  RateLimitMiddleware
    requestID  RequestIDMiddleware
    metrics    MetricsMiddleware
}

func NewStack(cfg StackConfig) *MiddlewareStack { ... }

func (s *MiddlewareStack) Wrap(handler http.Handler) http.Handler {
    // 固定顺序，不允许调用方修改
    h := handler
    h = s.rateLimit.Wrap(h)   // 6. Rate Limit
    h = s.rbac.Wrap(h)        // 5. RBAC
    h = s.tenant.Wrap(h)      // 4. Tenant (derive from AuthContext)
    h = s.auth.Wrap(h)        // 3. Auth (populate AuthContext)
    h = s.requestID.Wrap(h)   // 2. Request ID
    h = s.metrics.Wrap(h)     // 1. Metrics (outermost)
    return h
}
```

**执行顺序（从外到内）：**

```
Request
  → Metrics (observe latency, status code)
    → Request ID (generate/propagate X-Request-ID)
      → Auth (extract credential → populate AuthContext)
        → Tenant (derive tenant from AuthContext, NOT from header)
          → RBAC (check AuthContext.Roles against route policy)
            → Rate Limit (bucket by AuthContext.TenantID)
              → Handler
```

**关键约束：**
- Auth 必须在 Tenant 之前（CHL-2）
- Tenant 从 `AuthContext.TenantID` 派生，仅 `cross-tenant` 权限可覆盖 header
- 同一个 `MiddlewareStack` 实例挂载到 ACP server 和 API server（CHL-5）

---

### 决策 4: AuthContext 与 Credential Chain（CHL-4/CHL-8）

**AuthContext struct（P1 锁定）：**

```go
// internal/auth/context.go
type AuthContext struct {
    Identity   string   // user ID or API key ID
    TenantID   string   // derived from credential, NOT from header
    Roles      []string // ["user"], ["admin"], ["operator"]
    AuthMethod string   // "static_token", "jwt", "api_key"
}

type contextKey struct{}

func FromContext(ctx context.Context) (*AuthContext, bool) {
    ac, ok := ctx.Value(contextKey{}).(*AuthContext)
    return ac, ok
}

func WithContext(ctx context.Context, ac *AuthContext) context.Context {
    return context.WithValue(ctx, contextKey{}, ac)
}
```

**Credential extractor chain（P1 定义接口，P2 实现 JWT/API key）：**

```go
// internal/auth/chain.go
type CredentialExtractor interface {
    Extract(r *http.Request) (*AuthContext, error)
    Name() string
}

type ExtractorChain struct {
    extractors []CredentialExtractor
}

func (c *ExtractorChain) Extract(r *http.Request) (*AuthContext, error) {
    for _, e := range c.extractors {
        ac, err := e.Extract(r)
        if err == nil && ac != nil {
            ac.AuthMethod = e.Name()
            return ac, nil
        }
    }
    return nil, ErrNoValidCredential
}
```

**P1 chain：** `[StaticTokenExtractor]`
**P2 chain：** `[JWTExtractor, APIKeyExtractor, StaticTokenExtractor]` — 优先级从高到低

**向后兼容保证：** 静态 token 始终是 chain 的最后一个 extractor。只要 `HERMES_ACP_TOKEN` 环境变量非空，现有客户端不受影响。

---

### 决策 5: Tenant Middleware 信任模型修改（CHL-2）

**现状（将被修改）：**

```go
// internal/middleware/tenant.go — CURRENT (unsafe)
tenantID := r.Header.Get("X-Tenant-ID")
if tenantID == "" { tenantID = "default" }
```

**修改后：**

```go
// internal/middleware/tenant.go — AFTER MODIFICATION
func TenantMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ac, ok := auth.FromContext(r.Context())
        if !ok {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }

        tenantID := ac.TenantID  // from authenticated credential

        // cross-tenant override: only if principal has permission
        if headerTenant := r.Header.Get("X-Tenant-ID"); headerTenant != "" {
            if headerTenant != tenantID && !hasRole(ac.Roles, "cross-tenant") {
                http.Error(w, "forbidden: cannot access other tenant", http.StatusForbidden)
                return
            }
            if headerTenant != tenantID {
                tenantID = headerTenant  // admin cross-tenant access
            }
        }

        ctx := context.WithValue(r.Context(), tenantKey{}, tenantID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

### 决策 6: Rate Limit 降级策略（CHL-7）

```go
// internal/middleware/ratelimit.go
type RateLimiter struct {
    redis    *redis.Client
    fallback *localLimiter  // in-memory per-process fallback
    metrics  *prometheus.CounterVec
}

func (rl *RateLimiter) Allow(tenantID string, limit int) (bool, error) {
    allowed, err := rl.redis.SlidingWindowCheck(tenantID, limit)
    if err != nil {
        // Redis unavailable: fail-open with local fallback
        rl.metrics.WithLabelValues("redis_fallback").Inc()
        slog.Warn("rate limit redis unavailable, using local fallback", "error", err)
        return rl.fallback.Allow(tenantID, limit), nil
    }
    return allowed, nil
}
```

**策略：** Redis 不可用时 fail-open + 本地 in-memory 计数器 + Prometheus 告警指标。
**理由：** SaaS 服务可用性优先于精确限流。本地 fallback 提供"尽力而为"保护，不会因 Redis 故障阻塞所有用户。

---

## 关键数据流

### 认证请求处理流程

```
Client Request
  │
  ▼
Metrics Middleware (record start time)
  │
  ▼
Request ID Middleware (generate UUID, set header)
  │
  ▼
Auth Middleware
  ├─ Try JWT token → extract tenant_id from claims
  ├─ Try API Key → lookup hash → get tenant_id
  ├─ Try Static token → use "default" tenant + admin role
  └─ All fail → 401 Unauthorized
  │
  ▼ (AuthContext in context)
Tenant Middleware
  ├─ Read AuthContext.TenantID
  ├─ If X-Tenant-ID header differs → check cross-tenant permission
  └─ Set TenantID in context
  │
  ▼
RBAC Middleware
  ├─ Match route to required role
  └─ AuthContext.Roles contains required? → pass : 403
  │
  ▼
Rate Limit Middleware
  ├─ Lookup Tenant.RateLimitRPM
  ├─ Redis sliding window check (fallback to local)
  └─ Over limit? → 429 + Retry-After : pass
  │
  ▼
Handler (ACP or API server)
  │
  ▼
Audit Log (async write to AuditLogStore)
  │
  ▼
Metrics Middleware (record duration, status code)
```

### Audit Log 写入路径

```go
// 同步写入（关键操作）
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
    // ... business logic ...
    store.AuditLogs().Append(ctx, &store.AuditLog{
        TenantID:  tenant.ID,
        UserID:    ac.Identity,
        SessionID: session.ID,
        Action:    "session.create",
        Detail:    fmt.Sprintf("model=%s", session.Model),
    })
}
```

**覆盖范围：** session.create, session.delete, auth.login, auth.failure, tenant.create, tenant.update, tenant.delete, apikey.create, apikey.revoke

---

## 接口约定

### 新增 API 端点

| Method | Path | Phase | Auth | RBAC |
|--------|------|-------|------|------|
| GET | `/v1/audit-logs` | P1 | required | admin |
| GET | `/v1/tenants` | P2 | required | admin |
| POST | `/v1/tenants` | P2 | required | admin |
| GET | `/v1/tenants/{id}` | P2 | required | admin |
| PUT | `/v1/tenants/{id}` | P2 | required | admin |
| DELETE | `/v1/tenants/{id}` | P2 | required | admin |
| POST | `/v1/auth/login` | P2 | none | none |
| POST | `/v1/auth/refresh` | P2 | jwt | none |
| POST | `/v1/api-keys` | P2 | required | admin |
| GET | `/v1/api-keys` | P2 | required | admin |
| DELETE | `/v1/api-keys/{id}` | P2 | required | admin |
| GET | `/metrics` | P3 | none | none |
| GET | `/v1/tenants/{id}/usage` | P4 | required | admin/operator |
| GET | `/v1/tenants/{id}/billing/usage` | P4 | required | admin |
| GET | `/v1/docs` | P4 | none | none |
| GET | `/v1/users/{id}/export` | P5 | required | admin |
| DELETE | `/v1/users/{id}/data` | P5 | required | admin |

### 错误响应格式

```json
{
  "error": {
    "code": "rate_limit_exceeded",
    "message": "Rate limit exceeded. Retry after 30 seconds.",
    "details": {
      "retry_after": 30,
      "limit": 60,
      "remaining": 0
    }
  }
}
```

---

## 技术选型

| 类别 | 选择 | 原因 |
|------|------|------|
| JWT 签名 | RS256 | 公钥可分发给 API gateway 做验证，私钥仅 auth 服务持有 |
| Rate Limit 算法 | Redis 滑动窗口 | 精确度高于固定窗口，Redis MULTI/EXEC 原子操作 |
| Migration | 手动 schema_version | 零外部依赖，逻辑简单（< 50 行） |
| API 文档 | swaggo/swag | Go 社区标准，doc comments 生成 OpenAPI 3.0 |
| Metrics | prometheus/client_golang | Go 社区标准，K8s 原生集成 |
| Helm Chart | Helm 3 | K8s 标准部署工具 |

---

## 风险与约束

| # | 风险 | 严重度 | 约束/缓解 |
|---|------|--------|---------|
| AR1 | 两个 HTTP server 中间件不一致 | 高 | T0c 统一 middleware stack，单一来源 |
| AR2 | Tenant 身份欺骗 | 高 | Auth 先于 Tenant，JWT claims 派生 |
| AR3 | Migration 无回滚 | 中 | 每次 ALTER 必须是向后兼容的（add column，不 drop） |
| AR4 | Redis SPOF for rate limit | 中 | Local fallback + metrics 告警 |
| AR5 | JWT 密钥泄露 | 高 | P5 Vault 集成；P1-P4 文件配置 + 权限控制 |
| AR6 | SQLite no-op stubs 可能掩盖 bug | 低 | 仅 dev 使用；CI 用 PG testcontainer |

---

*最后更新：2026-04-28*
*来源：architect challenge + backend challenge + 代码实际读取*
