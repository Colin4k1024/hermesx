# Architecture Design: Enterprise SaaS GA Hardening v1.2.0

**状态**: Draft  
**主责**: architect  
**日期**: 2026-05-06  
**Slug**: enterprise-saas-ga

---

## 1. 系统边界

### 1.1 边界内（本次变更）

```
┌─────────────────────────────────────────────────────────────────┐
│                        hermesx                            │
│                                                                   │
│  ┌──────────┐  ┌──────────────┐  ┌────────────────────────────┐ │
│  │ auth/    │  │ middleware/  │  │ api/ + api/admin/          │ │
│  │ +OIDC    │  │ +双层限流    │  │ +billing, +pricing, +GDPR  │ │
│  └──────────┘  └──────────────┘  └────────────────────────────┘ │
│  ┌──────────┐  ┌──────────────┐  ┌────────────────────────────┐ │
│  │ llm/     │  │ metering/   │  │ store/pg/                  │ │
│  │ +breaker │  │ +pricing DB │  │ +RLS WITH CHECK migrations │ │
│  └──────────┘  └──────────────┘  └────────────────────────────┘ │
│  ┌──────────┐  ┌──────────────┐  ┌────────────────────────────┐ │
│  │ objstore/│  │ observability│  │ config/                    │ │
│  │ +GDPR del│  │ +trace-log  │  │ +弱密码校验                 │ │
│  └──────────┘  └──────────────┘  └────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
        │              │              │              │
   ┌────▼────┐   ┌────▼────┐   ┌────▼────┐   ┌────▼────┐
   │ PG 16   │   │ Redis 7 │   │ MinIO   │   │ OIDC IdP│
   │ +RLS WC │   │ +2-layer │   │ +delete │   │ (外部)  │
   │ +trigger │   │  Lua     │   │  API    │   │         │
   └─────────┘   └─────────┘   └─────────┘   └─────────┘
```

### 1.2 边界外（不变）

- WebUI / Chat UI 前端
- LLM Providers (Anthropic/OpenAI) — 接口不变，仅 breaker 治理
- Gateway platforms (LINE/Slack/Discord) — 不受影响
- Batch RL Training — 不在范围

### 1.3 外部依赖

| 依赖 | 版本 | 用途 | 风险 |
|------|------|------|------|
| coreos/go-oidc/v3 | latest | OIDC Discovery + JWT 验证 | 新增依赖 |
| gobreaker | v2.x (已有) | Provider 级断路器 | 无 |
| minio-go/v7 | 已有 | GDPR 对象删除 | 无 |
| pgxpool | v5 (已有) | RLS SET LOCAL 事务 | 无 |

---

## 2. 组件设计

### 2.1 OIDCExtractor（新增）

**位置**: `internal/auth/oidc.go`

```go
type OIDCExtractor struct {
    issuerURL string
    claimMap  OIDCClaimMap
    verifier  *oidc.IDTokenVerifier  // coreos/go-oidc
    provider  *oidc.Provider         // handles JWKS rotation
}

type OIDCClaimMap struct {
    TenantID string // env: HERMES_OIDC_TENANT_CLAIM, default "tenant_id"
    Roles    string // env: HERMES_OIDC_ROLES_CLAIM, default "roles"
    Subject  string // default "sub"
}

// Extract implements auth.CredentialExtractor
func (o *OIDCExtractor) Extract(r *http.Request) (*auth.AuthContext, error)
```

**设计决策**：
- 不复用现有 `JWTExtractor`（静态 RSA 公钥，无 JWKS 轮换）
- `ExtractorChain` 顺序：OIDC → APIKey（OIDC 优先匹配 Bearer token）
- alg 不匹配返回 error（非 nil），防止静默降级
- ACR claim 校验仅在 `/admin/*` 路由生效

### 2.2 ProviderBreakerRegistry（新增）

**位置**: `internal/llm/breaker_registry.go`

```go
type ProviderBreakerRegistry struct {
    mu       sync.Mutex
    breakers map[string]*gobreaker.CircuitBreaker[[]byte]
    settings gobreaker.Settings
}

func NewProviderBreakerRegistry(settings gobreaker.Settings) *ProviderBreakerRegistry
func (r *ProviderBreakerRegistry) Get(provider string) *gobreaker.CircuitBreaker[[]byte]
```

**设计决策**：
- 进程级单例（Server 启动时初始化，注入 Transport 层）
- breaker 粒度从 model 改为 provider（`"llm-provider-anthropic"`）
- `ChatStream` 完成/失败必须更新 breaker 计数（修复已知 bug D-09）
- Prometheus gauge: `hermes_circuit_breaker_state{provider}` (0=closed, 1=half-open, 2=open)

### 2.3 双层 Rate Limiter（增强）

**位置**: `internal/middleware/redis_ratelimiter.go`（现有文件修改）

```go
// 扩展后的 Lua 脚本：原子性检查 tenant + user/key 两层
var dualRateLimitScript = redis.NewScript(`
    local tenant_key = KEYS[1]    -- rl:{tenantID}
    local user_key = KEYS[2]      -- rl:{tenantID}:key:{keyID} or rl:{tenantID}:user:{userID}
    local tenant_limit = tonumber(ARGV[1])
    local user_limit = tonumber(ARGV[2])
    local window = tonumber(ARGV[3])
    local now = tonumber(ARGV[4])
    local member = ARGV[5]

    -- Check tenant first
    redis.call('ZREMRANGEBYSCORE', tenant_key, '0', now - window)
    local tenant_count = redis.call('ZCARD', tenant_key)
    if tenant_count >= tenant_limit then
        return {0, 0, 0}  -- denied, tenant_remaining=0, user_remaining=0
    end

    -- Check user/key
    redis.call('ZREMRANGEBYSCORE', user_key, '0', now - window)
    local user_count = redis.call('ZCARD', user_key)
    if user_count >= user_limit then
        return {0, tenant_limit - tenant_count, 0}  -- denied
    end

    -- Both pass: record
    redis.call('ZADD', tenant_key, now, member..':t')
    redis.call('ZADD', user_key, now, member..':u')
    redis.call('EXPIRE', tenant_key, window)
    redis.call('EXPIRE', user_key, window)
    return {1, tenant_limit - tenant_count - 1, user_limit - user_count - 1}
`)
```

**设计决策**：
- 单次 Lua 调用原子性检查两层（避免 TOCTOU race）
- `RateLimitConfig.UserLimitFn` 为 nil 时退化为 tenant-only（向后兼容）
- `X-RateLimit-Remaining` 返回 `min(tenant_remaining, user_remaining)`
- user key 格式取决于认证方式（api_key → key ID, OIDC → user ID）

### 2.4 PricingStore（新增）

**位置**: `internal/metering/pricing_store.go`

```go
type PricingRule struct {
    ModelKey       string    `json:"model_key" db:"model_key"`
    InputPer1K    float64   `json:"input_per_1k" db:"input_per_1k"`
    OutputPer1K   float64   `json:"output_per_1k" db:"output_per_1k"`
    CacheReadPer1K float64  `json:"cache_read_per_1k" db:"cache_read_per_1k"`
    UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

type PricingStore interface {
    List(ctx context.Context) ([]PricingRule, error)
    Upsert(ctx context.Context, rule *PricingRule) error
    Delete(ctx context.Context, modelKey string) error
    GetCost(ctx context.Context, model string) (*PricingRule, error)  // cached, TTL 30s
}
```

**设计决策**：
- 单一价格源（合并 `metering/cost.go` 和 `agent/pricing.go`）
- DB 优先 + 代码 fallback（无规则时使用 defaultCosts map）
- 内存缓存 TTL 30s，启动时预热
- 价格变更不溯及既往

### 2.5 BillingStore（新增）

**位置**: `internal/metering/billing_store.go`

```go
type BillingSummary struct {
    TenantID         string  `json:"tenant_id"`
    Period           string  `json:"period"`  // "2026-05"
    TotalInputTokens int64   `json:"total_input_tokens"`
    TotalOutputTokens int64  `json:"total_output_tokens"`
    TotalCostUSD     float64 `json:"total_cost_usd"`
    RecordCount      int     `json:"record_count"`
    Type             string  `json:"type"`  // "realtime"
}

type InvoiceLineItem struct {
    Model         string  `json:"model"`
    InputTokens   int64   `json:"input_tokens"`
    OutputTokens  int64   `json:"output_tokens"`
    UnitPricePer1K float64 `json:"unit_price_per_1k"`
    SubtotalUSD   float64 `json:"subtotal_usd"`
}

type BillingStore interface {
    GetSummary(ctx context.Context, tenantID string, year int, month int) (*BillingSummary, error)
    GetInvoice(ctx context.Context, tenantID string, year int, month int) (*Invoice, error)
}
```

### 2.6 GDPR MinIO 清理（增强）

**位置**: `internal/api/gdpr.go`（现有文件修改）

```go
// deleteViaTx 增加 MinIO 清理
func (h *GDPRHandler) deleteViaTx(ctx context.Context, tenantID string) (int, error) {
    // 1. PG transaction (existing)
    if err := h.deletePGData(ctx, tenantID); err != nil {
        return http.StatusInternalServerError, err
    }

    // 2. MinIO cleanup (post-commit, best-effort with audit)
    if h.minioClient != nil {
        if err := h.deleteMinIOObjects(ctx, tenantID); err != nil {
            // Record failure in purge_audit_logs
            h.recordMinIOCleanupFailure(ctx, tenantID, err)
            return http.StatusMultiStatus, err  // 207
        }
    }
    return http.StatusNoContent, nil
}

func (h *GDPRHandler) deleteMinIOObjects(ctx context.Context, tenantID string) error {
    prefix := tenantID + "/"
    objectCh := h.minioClient.ListObjects(ctx, h.bucket, minio.ListObjectsOptions{
        Prefix:    prefix,
        Recursive: true,
    })
    for obj := range objectCh {
        if obj.Err != nil {
            return obj.Err
        }
        if err := h.minioClient.RemoveObject(ctx, h.bucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
            return fmt.Errorf("remove %s: %w", obj.Key, err)
        }
    }
    return nil
}
```

---

## 3. 关键数据流

### 3.1 RLS WITH CHECK 写入路径

```
Application Layer         PostgreSQL
─────────────────        ──────────────────────────────────
pool.Begin(ctx)    →     BEGIN
SET LOCAL app.current_tenant = 'T1'
INSERT INTO sessions  →  WITH CHECK (tenant_id = current_setting('app.current_tenant', false))
                          ├── tenant_id = 'T1' → PASS
                          └── tenant_id = 'T2' → ERROR: policy violation
COMMIT             →     COMMIT
```

**关键约束**：
- 所有写操作必须在事务中执行（`SET LOCAL` 仅事务作用域）
- `current_setting('app.current_tenant', false)` — GUC 未设置时报错（非返回 NULL）
- `FORCE ROW LEVEL SECURITY` — table owner 也受 RLS 约束

### 3.2 OIDC 认证流

```
Client                hermes-agent              OIDC Provider
──────               ────────────              ─────────────
GET /v1/chat
Authorization: Bearer <id_token>
        │
        ▼
ExtractorChain:
  1. OIDCExtractor.Extract()
     ├── Parse Bearer token
     ├── verifier.Verify(token)  ──────── JWKS fetch (cached 5min)
     ├── Extract claims (tenant_id, roles, sub)
     ├── If /admin/* : check acr claim contains "mfa"
     └── Return AuthContext{TenantID, Roles, Identity, Scopes}
  2. (fallback) APIKeyExtractor — skipped if OIDC matched
```

### 3.3 双层限流流

```
Request → AuthMiddleware → RateLimitMiddleware
                                │
                                ▼
                    ┌─ tenant_key: rl:{tenantID} ──────────┐
                    │  user_key:   rl:{tenantID}:key:{kid} │
                    └──────────────────────────────────────┘
                                │
                    Lua Script (atomic dual check)
                                │
                    ├── Both OK → 200 + X-RateLimit-Remaining: min(t,u)
                    ├── Tenant exceeded → 429 + Retry-After
                    └── User exceeded → 429 + Retry-After
```

### 3.4 Provider 断路器流

```
ChatRequest → FallbackRouter → ProviderBreakerRegistry.Get("anthropic")
                                        │
                                        ▼
                               breaker.Execute(func() {
                                   transport.Chat(req)
                               })
                                        │
                               ├── Success → breaker count success
                               ├── Failure → breaker count failure
                               │              if threshold → state=OPEN
                               └── Open → skip provider → next fallback
```

---

## 4. 接口约定

### 4.1 新增 API 端点

| Method | Path | Auth | 描述 |
|--------|------|------|------|
| GET | /admin/v1/pricing-rules | admin scope | 列出所有定价规则 |
| PUT | /admin/v1/pricing-rules/{model_key} | admin scope | upsert 定价规则 |
| DELETE | /admin/v1/pricing-rules/{model_key} | admin scope | 删除（回落代码默认） |
| GET | /admin/v1/tenants/{id}/billing/summary | admin scope | 月度用量汇总 |
| GET | /admin/v1/tenants/{id}/billing/invoice | admin scope | 月度发票 JSON |
| POST | /admin/v1/tenants | admin scope | 创建租户 |
| PATCH | /admin/v1/tenants/{id} | admin scope | 部分更新租户 |
| DELETE | /admin/v1/tenants/{id} | admin scope | 软删除租户 |
| POST | /admin/v1/tenants/{id}/restore | admin scope | 恢复软删除租户 |
| POST | /v1/gdpr/cleanup-minio | admin scope | 幂等重试 MinIO 清理 |

### 4.2 现有端点变更

| 端点 | 变更 |
|------|------|
| DELETE /v1/gdpr/data | 返回 207 Multi-Status（MinIO 部分失败） |
| POST /v1/keys | 强制设置 ExpiresAt（默认 90d） |
| GET /v1/keys | 响应增加 `expires_soon` 字段 |
| ALL /v1/chat/* | session owner 校验（非 owner 返回 403） |

### 4.3 新增 PostgreSQL 对象

| 类型 | 名称 | 用途 |
|------|------|------|
| Migration | RLS WITH CHECK (9 tables) | 写操作隔离 |
| Migration | FORCE ROW LEVEL SECURITY (9 tables) | table owner 也受限 |
| Migration | REVOKE DELETE ON audit_logs | 不可篡改 |
| Migration | purge_audit_logs INSERT only | 证据链保护 |
| Function | gdpr_purge_audit_logs() SECURITY DEFINER | GDPR 删除特权函数 |
| Table | pricing_rules | 动态定价存储 |
| Role | hermes_app (补全约束) | 应用角色权限最小化 |

---

## 5. 技术选型

| 选择 | 原因 | 备选 |
|------|------|------|
| coreos/go-oidc/v3 | 轻量、维护活跃、标准 OIDC Discovery | lestrrat-go/jwx（更重，功能更多） |
| Lua 双层限流 | 原子性、避免 TOCTOU、Redis 单次往返 | 两次独立 Lua 调用（多一次 RTT） |
| REVOKE DELETE + SECURITY DEFINER | 数据库层强制不可篡改 | BEFORE DELETE 触发器（可被 disable） |
| SET LOCAL | pgBouncer transaction mode 安全 | SET (session 级，有连接池污染风险) |
| realtime 账单 | 避免 invoices 表状态机复杂度 | 物化视图（过早优化） |

---

## 6. 风险与约束

### 6.1 技术风险

| 风险 | 约束/缓解 |
|------|---------|
| SET LOCAL 遗漏（写路径不在事务中） | 代码审查 + 集成测试覆盖所有 store 写方法 |
| OIDC token 过期处理 | verifier 配置 `SkipExpiry: false`（默认行为） |
| pricing 缓存雪崩 | singleflight + jitter（缓存刷新时只有一个 goroutine 查库） |
| MinIO 删除大量对象超时 | ListObjects 分批 + context deadline 控制 |

### 6.2 向后兼容

| 组件 | 兼容策略 |
|------|---------|
| API Key 认证 | 不受 OIDC 影响，ExtractorChain fallback |
| 现有 /v1/tenants | 保持工作，标记 Deprecated |
| 限流 | UserLimitFn 为 nil 时退化为 tenant-only |
| 成本计算 | DB 无规则时 fallback 到代码默认值 |
| sessions.UserID 为空 | owner 校验豁免（legacy session） |

---

## 7. Migration 执行顺序

```
Migration 65: ALTER TABLE ... FORCE ROW LEVEL SECURITY (9 tables)
Migration 66: CREATE POLICY ... WITH CHECK ... FOR INSERT/UPDATE/DELETE (9 tables)
Migration 67: REVOKE DELETE ON audit_logs FROM hermes_app
Migration 68: REVOKE DELETE, UPDATE ON purge_audit_logs FROM hermes_app
Migration 69: CREATE FUNCTION gdpr_purge_audit_logs() SECURITY DEFINER
Migration 70: CREATE TABLE pricing_rules (model_key TEXT PK, ...)
Migration 71: ALTER TABLE api_keys ADD COLUMN expires_at TIMESTAMPTZ
Migration 72: (optional) usage_records RLS or GRANT restriction
```

**回滚策略**：每个 migration 有对应 down script；FORCE RLS 可通过 `ALTER TABLE ... NO FORCE ROW LEVEL SECURITY` 回退。

---

## 8. 可观测性增强

| 指标/日志 | 类型 | 说明 |
|-----------|------|------|
| `hermes_circuit_breaker_state{provider}` | Prometheus gauge | 0/1/2 |
| `hermes_backup_last_success_timestamp` | Prometheus gauge | pitr-drill 更新 |
| `hermes_ratelimit_denied_total{layer,tenant}` | Prometheus counter | tenant/user 层 |
| `trace_id` / `span_id` in slog | 结构化日志 | trace-log 关联 |
| `hermes_oidc_auth_total{status}` | Prometheus counter | OIDC 认证统计 |
| `hermes_billing_query_duration_seconds` | Prometheus histogram | 账单查询性能 |
