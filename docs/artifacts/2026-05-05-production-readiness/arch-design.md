# Architecture Design: HermesX — Production Readiness

**状态**: Draft  
**主责**: architect  
**日期**: 2026-05-05  
**版本目标**: v1.1.0

---

## 1. 设计原则

- **纯增量（Additive Only）**: 所有现有 API 契约、auth 链路、RBAC、circuit breaker 和 OTel 接线不变
- **Transport 层组合**: 新能力通过 Transport decorator 注入，不修改下游 provider 实现
- **热路径零阻塞**: Token 计量异步写入，不增加 LLM 调用延迟
- **无新运行时依赖**: Redis 已部署，pgBackRest 仅运维工具

---

## 2. 系统边界

```
                    ┌─────────────────────────────────┐
                    │        Nginx LB (ip_hash)        │
                    └───────────────┬─────────────────┘
                                    │
              ┌─────────────────────┼─────────────────────┐
              │                     │                     │
      ┌───────▼───────┐   ┌───────▼───────┐   ┌───────▼───────┐
      │  Replica 1    │   │  Replica 2    │   │  Replica 3    │
      │  hermes-api   │   │  hermes-api   │   │  hermes-api   │
      └───────┬───────┘   └───────┬───────┘   └───────┬───────┘
              │                     │                     │
      ┌───────▼─────────────────────▼─────────────────────▼───────┐
      │                    Shared Infrastructure                    │
      ├──────────────┬──────────────┬──────────────┬──────────────┤
      │ PostgreSQL   │ Redis 7      │ MinIO (S3)   │ OTel Coll.   │
      │ (RLS + WAL)  │ (Rate+Lock)  │ (Skills)     │ (Traces)     │
      └──────────────┴──────────────┴──────────────┴──────────────┘
```

---

## 3. Gap 1: Redis Sliding Window Rate Limiter

### 现状

`RateLimiter` interface 已定义（`internal/middleware/ratelimit.go`），本地 LRU fallback 已实现。`rediscache.Client` 已有 `Allow()` 方法但为固定窗口 INCR 实现，存在窗口边界突发风险。

### 设计

**算法**: ZSET sliding window（精度 > 固定窗口，复杂度可控）

```
文件: internal/middleware/redis_ratelimiter.go
实现 RateLimiter interface

MULTI/EXEC 原子操作:
  1. ZREMRANGEBYSCORE key 0 (now - window)   -- 移除过期条目
  2. ZADD key now member                      -- 添加当前请求
  3. ZCARD key                                -- 计算窗口内请求数
  4. EXPIRE key window                        -- 设置 TTL 防泄漏
```

**存储估算**: 每条目 ~40 bytes（score + member），60 RPM 窗口 = 最多 60 条目/key ≈ 2.4KB/tenant。

**Key 格式**: `rl:{tenant_id}:{minute_bucket}`

**降级**: Redis 不可达时自动切回本地 LRU limiter（已有 fallback 逻辑）。

### 接口

```go
type RedisSlidingWindowLimiter struct {
    client *redis.Client
    window time.Duration
}

func (r *RedisSlidingWindowLimiter) Allow(key string, limit int) (bool, int, error)
```

---

## 4. Gap 2: LLM Fallback Router

### 现状

`ResilientTransport` 已实现 per-model circuit breaker（`internal/llm/breaker.go`），但只包装单个 Transport。无跨 provider 降级能力。

### 设计

**模式**: `FallbackRouter` 作为 Transport decorator，包装两个 `ResilientTransport` 实例。

```go
// internal/llm/fallback_router.go
type FallbackRouter struct {
    primary   Transport  // e.g. ResilientTransport(Anthropic)
    fallback  Transport  // e.g. ResilientTransport(OpenAI)
    logger    *slog.Logger
}

func (f *FallbackRouter) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
func (f *FallbackRouter) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamDelta, <-chan error)
func (f *FallbackRouter) Name() string
```

**降级触发条件**:
- Circuit breaker open（primary 已熔断）
- HTTP 5xx 响应
- Context timeout / deadline exceeded

**不触发降级**:
- 4xx 错误（客户端问题，换 provider 也不会好）
- 正常业务错误

**Response 标记**: 降级时设置 `ChatResponse.Degraded = true`，上层可据此通知用户。

### 组合关系

```
FallbackRouter
├── primary: ResilientTransport(AnthropicTransport) + breaker
└── fallback: ResilientTransport(OpenAITransport) + breaker
```

---

## 5. Gap 3: Token Usage Persistence

### 现状

`ChatResponse` 已包含 `InputTokens`/`OutputTokens` 字段，Prometheus metrics 已记录。但未持久化到数据库，无法按租户/时段查询。

### 设计

**模式**: 异步批量写入（buffered channel + batch INSERT）

```go
// internal/metering/usage_recorder.go
type UsageRecorder struct {
    ch     chan UsageRecord
    store  UsageStore
    flush  time.Duration  // 5s
    batch  int            // 100
}
```

**触发 flush 条件**:
- 缓冲区达到 100 条记录
- 5 秒定时器触发
- Shutdown signal（graceful drain）

**为什么异步**: LLM 调用已 500ms-5s，不能在热路径再加 DB 写入延迟。

### 数据库

```sql
-- Migration v61: usage_records
CREATE TABLE usage_records (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id),
    session_id TEXT NOT NULL,
    user_id TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL,
    provider TEXT NOT NULL,
    input_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    cache_read_tokens INT NOT NULL DEFAULT 0,
    cache_write_tokens INT NOT NULL DEFAULT 0,
    cost_usd NUMERIC(10,6) DEFAULT 0,
    degraded BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_usage_records_tenant_date ON usage_records(tenant_id, created_at);
```

**成本计算**: 静态配置 map（model name → cost per 1K tokens）。未知模型 log warning + 按最高已知 tier 计费。

**RLS**: 该表启用 RLS，`USING (tenant_id = current_setting('app.current_tenant'))`。

---

## 6. Gap 4: API Key Scope Enhancement

### 现状

`api_keys` 表已有 `roles` 字段。但缺乏细粒度 scope 控制（如只读 key、只能调用特定 endpoint）。

### 设计

```sql
-- Migration v62
ALTER TABLE api_keys ADD COLUMN scopes TEXT[] DEFAULT '{}';
```

**Scope 枚举**: `read`, `write`, `admin`, `sandbox`

**兼容性**: `scopes = '{}'`（空数组）表示 legacy key，回退到现有 role-based 检查。非空 scopes 则由新 middleware 精确校验。

**Middleware 层**:
```go
// internal/middleware/scope_check.go
func RequireScope(scope string) Middleware
```

路由绑定示例:
- `GET /v1/sessions` → RequireScope("read")
- `POST /v1/chat` → RequireScope("write")
- `POST /admin/v1/...` → RequireScope("admin")

---

## 7. Gap 5: Admin API

### 现状

SandboxPolicy 只能通过 SQL 设置。API Key 管理缺少 rotation endpoint。

### 设计

**Package**: `internal/api/admin/`

| Method | Path | Scope | 说明 |
|--------|------|-------|------|
| POST | /admin/v1/tenants/:id/sandbox-policy | admin | Set sandbox policy |
| GET | /admin/v1/tenants/:id/sandbox-policy | admin | Get sandbox policy |
| DELETE | /admin/v1/tenants/:id/sandbox-policy | admin | Reset to default |
| POST | /admin/v1/tenants/:id/api-keys | admin | Create new API key |
| POST | /admin/v1/tenants/:id/api-keys/:kid/rotate | admin | Rotate key |
| DELETE | /admin/v1/tenants/:id/api-keys/:kid | admin | Revoke key |
| GET | /v1/usage | read | Tenant usage summary |
| GET | /v1/usage/details | read | Per-session usage detail |
| GET | /admin/v1/audit-logs | admin | Query audit logs |

**Key Rotation Flow**:
1. 生成新 32-byte base62 key
2. INSERT 新 api_keys 记录（同 tenant、同 roles/scopes）
3. SET 旧 key `revoked_at = NOW()`
4. 返回 one-time raw key（此后不可再次查看）
5. 写入 audit log: `API_KEY_ROTATED`

---

## 8. Gap 6: PG PITR Backup

### 现状

无备份策略。

### 设计

**目标**: RPO < 5min, RTO < 1h

**方案**: pgBackRest（比 pg_basebackup + wal-g 更成熟的单一工具链）

**配置**:
- WAL archive timeout: 5 min（保证 RPO）
- Full backup: 每周日
- Differential: 每日
- 存储: 本地 + 可选 S3

**交付物**: 
- `deploy/pitr/docker-compose.pitr.yml`（验证环境）
- `docs/runbooks/pg-pitr-recovery.md`（恢复 runbook）
- CI job 验证 runbook 可执行性

**不引入应用代码变更** — 纯运维层。

---

## 9. Gap 7: CI Integration Tests

### 现状

`.github/workflows/ci.yml` 有 build/test/lint/race/docker，但 `go test` 只跑 unit tests（不连真实 DB）。

### 设计

**Build Tag**: `//go:build integration`（与 unit tests 隔离）

**GHA Service Containers**:
```yaml
services:
  postgres:
    image: postgres:16
    env: { POSTGRES_DB: hermes_test, ... }
  redis:
    image: redis:7
  minio:
    image: minio/minio:latest
```

**Job**: `integration-test`，依赖 `test` job 通过后执行。

**门禁**: PR 合并前强制通过 integration tests。

---

## 10. Gap 8: Multi-Replica Verification

### 现状

38 个集成测试在单实例通过。未验证多副本下的状态一致性。

### 设计

**文件**: `deploy/docker-compose.multi-replica.yml`

```yaml
services:
  hermes-1: { ... }
  hermes-2: { ... }
  hermes-3: { ... }
  nginx:
    image: nginx:alpine
    # ip_hash upstream for session affinity
```

**验证脚本**: `scripts/verify-multi-replica.sh`
- 限流一致性：3 副本下同 tenant 限流计数准确
- Session 可见性：任意副本创建的 session 在其他副本可读
- Failover：停掉 1 副本，请求自动路由到存活副本

---

## 11. 关键数据流

### LLM 请求完整链路

```
Client Request
    │
    ▼
[Auth Middleware] → API Key → TenantID injection
    │
    ▼
[Rate Limit Middleware] → Redis ZSET check → 429 if exceeded
    │
    ▼
[Tracing Middleware] → Create root span, set X-Trace-ID
    │
    ▼
[Chat Handler] → Build ChatRequest
    │
    ▼
[FallbackRouter]
    ├── [Primary: ResilientTransport(Anthropic)]
    │       ├── Circuit Breaker check
    │       └── AnthropicTransport.Chat()
    │
    ├── (on 5xx/timeout/breaker-open) →
    │
    └── [Fallback: ResilientTransport(OpenAI)]
            ├── Circuit Breaker check
            └── OpenAITransport.Chat()
    │
    ▼
[UsageRecorder.Record()] → async buffered channel
    │
    ▼
[Response] → {content, degraded flag, usage tokens}
```

### Token Usage Batch Flush

```
LLM Response → UsageRecorder.Record(record)
                     │
                     ▼ (buffered channel, cap=1000)
               [Flush goroutine]
                     │
          ┌──────────┼──────────┐
          │                     │
    100 records            5s timer tick
          │                     │
          └──────────┬──────────┘
                     │
                     ▼
         BatchInsert([]UsageRecord) → PostgreSQL
```

---

## 12. 技术选型总结

| 决策 | 选型 | 替代方案 | 为什么 |
|------|------|----------|--------|
| Rate Limiter | ZSET sliding window | INCR fixed window / Token bucket | 精度 > 固定窗口，实现复杂度 < token bucket |
| Fallback 层级 | Transport decorator | Client 层 / Handler 层 | Transport 层组合性最好，不侵入业务逻辑 |
| Usage 写入 | Async batch flush | 同步写 / WAL-based CDC | 保护热路径延迟，实现简单 |
| Scopes | Additive TEXT[] column | 新表 / JWT claims | 最小迁移风险，空=legacy 兼容 |
| Backup | pgBackRest | pg_basebackup + wal-g | 单一工具链，社区成熟度高 |
| Multi-replica | Docker Compose + Nginx | Kind / K8s | 验证成本最低，本地可跑 |

---

## 13. 风险与约束

| 风险 | 影响 | 缓解 |
|------|------|------|
| ZSET 在极高并发下内存增长 | Redis OOM | EXPIRE 保证 key 自动回收，监控 key 数量 |
| FallbackRouter 双路由增加 LLM 费用 | 预算 | 只在 primary 失败时触发，正常路径不多花钱 |
| Async flush 丢数据（进程崩溃） | 计量缺失 | 缓冲区 < 100 条 × 5s，最坏丢 5s 数据，可接受 |
| Scopes 空数组语义变更 | 旧 key 行为漂移 | 空=legacy fallback，显式兼容 |

---

## 14. 工作量估算

| Phase | 工作项 | 估算 |
|-------|--------|------|
| P1 | Redis limiter + OTel verify + scope + metrics | 2-3 days |
| P2 | FallbackRouter + retry + integration tests + multi-replica | 3-4 days |
| P3 | Admin API + usage persistence + PITR | 2-3 days |
| P4 | Audit + compliance + security scan | 1-2 days |
| **Total** | | **8-11 days** |

**Critical Path**: P1 → P2（usage endpoint 依赖 usage store）。P3、P4 可并行。

---

**已创建**: `docs/artifacts/2026-05-05-production-readiness/arch-design.md`
