# SaaS 就绪任务拆分

|| 字段 | 值 |
||------|-----|
|| 日期 | 2026-04-27 |
|| 依据 | `saas-readiness-audit.md` (2.6/5) + `prd.md` |
|| 目标 | 将评分从 2.6 提升至 4.5/5 |
|| 预计 | 8-10 周 |

---

## 阶段 0：并发 Bug 修复（Phase 0 from PRD）

**优先级：P0 — 必须先修，否则多实例部署会 panic**

| # | 严重度 | 文件 | 问题 | 修复方案 | 工作量 |
|---|--------|------|------|---------|--------|
| C-1 | Critical | `agent.go:391-394` | `isInterrupted()` 用写锁查 bool | `atomic.Bool` 替代 | 0.5d |
| C-2 | Critical | `agent.go:386-388` | `Interrupt()` 用写锁写 bool | `atomic.Bool` 替代 | 0.5d |
| C-3 | High | `agent.go:442-460` | 并行 tool 无 WaitGroup 超时保护 | `sync.WaitGroup` + `context.WithTimeout(5min)` | 1d |
| C-4 | High | `approval.go:272-281` | `ClearSession` 对已关闭 channel 写 → panic | `select` 保护 + 标记关闭 | 1d |
| C-5 | Medium | `approval.go:206-216` | `Submit` 持锁时 make channel | 提前 pool 化 | 0.5d |
| C-6 | Medium | `events.go:78-84` | SSE Publish 静默丢事件 | 增加 metrics 计数 + 日志 | 0.5d |
| C-7 | Low | `state/db.go:100+` | 有 FTS5 但无全文搜索 API 暴露 | 暴露搜索接口 | 1d |
| C-8 | Low | `approval.go:139-142` | `IsApproved` map 并发写非安全 | `sync.Map` 替代 | 0.5d |

**阶段 0 交付物：** 所有 8 个并发 bug 修复 + 并发测试用例
**预估：** 5 天

---

## 阶段 1：接入层补齐（对应 S1）

**目标评分：3.5/5**

### 1.1 [RELEASE BLOCKER #1] 审计日志写入

| 项目 | 值 |
|------|-----|
| 优先级 | P0 |
| 预估 | 2d |
| 现状 | `audit_logs` 表存在但零 INSERT |

**任务：**
- [ ] 在 `store/pg/` 下实现 `audit_logs.go`，定义 `AuditLogStore` interface
  - 方法：`Insert(tenantID, userID, sessionID, action, detail string) error`
  - 方法：`List(tenantID string, limit, offset int) ([]*AuditLog, error)`
- [ ] 在所有关键操作处插入 audit log 调用：
  - `middleware/tenant.go` — tenant 解析时记录
  - `acp/auth.go` — 认证成功/失败
  - Session 创建/删除
  - User approved/revoked
  - Rate limit 触发
- [ ] 确保所有 audit log 携带 `tenant_id`（多租户审计隔离）
- [ ] 添加基础 audit log 读取 API `GET /v1/audit-logs`

**验收标准：** `audit_logs` 表有数据写入，管理员可查询

---

### 1.2 [RELEASE BLOCKER #2] RBAC Middleware 执行

| 项目 | 值 |
|------|-----|
| 优先级 | P0 |
| 预估 | 1d |
| 现状 | `Role="admin"` 存了但无中间件检查 |

**任务：**
- [ ] 创建 `middleware/rbac.go`
  - 读取 `User.Role` 从 context（由 auth middleware 注入）
  - 定义 role 权限：`user < operator < admin`
  - 检查点：
    - `GET /v1/admin/*` 需要 `admin` role
    - `POST /v1/tenants` 需要 `admin` role
    - `GET /v1/users` 需要 `operator` 及以上
- [ ] 将 RBAC middleware 挂载到 `gateway/` 和 `acp/` 的路由
- [ ] 统一错误返回：403 Forbidden + `{"error": "insufficient_permissions", "required_role": "admin"}`

**验收标准：** 非 admin 用户访问 admin 接口返回 403

---

### 1.3 [RELEASE BLOCKER #3] Rate Limit 接入 HTTP Handler

| 项目 | 值 |
|------|-----|
| 优先级 | P0 |
| 预估 | 1d |
| 现状 | `CheckRateLimit` 实现了但无 handler 调用 |

**任务：**
- [ ] 在 `middleware/` 下创建 `ratelimit.go`
  - 读取 tenant 的 `RateLimitRPM` 从 PG（或降级到配置默认值）
  - 调用 `rediscache.CheckRateLimit()`
  - 超限返回 `429 Too Many Requests` + `Retry-After` header
- [ ] 将 middleware 挂载到所有 `/v1/` 路由
- [ ] 添加 rate limit 响应头：`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`

**验收标准：** 超过 RPM 限制的请求返回 429

---

### 1.4 Health Endpoint 增强

| 项目 | 值 |
|------|-----|
| 优先级 | P1 |
| 预估 | 0.5d |

**任务：**
- [ ] `/health/ready` 增加 DB ping（`pgxpool.Ping()`）
- [ ] `/health/ready` 增加 Redis ping（`redis.Ping()`）
- [ ] 区分 `liveness`（进程存活）和 `readiness`（可接收流量）

---

**阶段 1 交付物：** audit log 写入、RBAC 执行、rate limit 生效
**预估：** 4.5 天

---

## 阶段 2：认证升级（对应 S2）

**目标评分：3.8/5**

### 2.1 [RELEASE BLOCKER #6] JWT 签发/验证

| 项目 | 值 |
|------|-----|
| 优先级 | P0 |
| 预估 | 5d |
| 现状 | 仅 static bearer token，多租户共享 |

**任务：**
- [ ] 实现 JWT 签发
  - 登录 endpoint：`POST /v1/auth/login`（user + password）
  - 返回 `access_token` (15min TTL) + `refresh_token` (7d TTL)
  - Token payload: `{tenant_id, user_id, role, exp, iat}`
- [ ] 实现 JWT 验证 middleware
  - 替换现有 `acp/auth.go` 的 bearer token 校验
  - 支持 RS256 算法
  - 支持 token 刷新：`POST /v1/auth/refresh`
- [ ] 实现 API Key 管理（per-tenant）
  - `POST /v1/tenants/{id}/api-keys` — 创建 API key（返回明文一次）
  - `GET /v1/tenants/{id}/api-keys` — 列出 keys（不返回明文）
  - `DELETE /v1/tenants/{id}/api-keys/{key_id}` — 撤销
  - API key 格式：`hk_live_<tenant_id>_<random_32chars>`
  - 存储：`bcrypt hash`，验证时 bcrypt compare

**验收标准：** JWT 可登录、可刷新；API key 可创建/验证/撤销

---

### 2.2 [RELEASE BLOCKER #5] Tenant CRUD API

| 项目 | 值 |
|------|-----|
| 优先级 | P0 |
| 预估 | 3d |
| 现状 | 无 TenantStore 接口，无法通过 API 创建租户 |

**任务：**
- [ ] 实现 `store/pg/tenants.go`
  - `Create(tenant *Tenant) error`
  - `GetByID(id string) (*Tenant, error)`
  - `Update(tenant *Tenant) error`
  - `List(limit, offset int) ([]*Tenant, int, error)`
- [ ] 实现 API endpoints：
  - `POST /v1/tenants` — 创建租户（admin only）
  - `GET /v1/tenants/{id}` — 获取租户详情
  - `PATCH /v1/tenants/{id}` — 更新租户（plan, rate_limit_rpm, max_sessions）
  - `DELETE /v1/tenants/{id}` — 删除租户（admin only，级联删除？）
  - `GET /v1/tenants` — 列出所有租户（admin only）
- [ ] Tenant 入驻流程：
  - 创建时生成 `api_key` 和 `api_secret`
  - 返回给管理员（一次性的）
- [ ] 添加 tenant 级配置字段（future-proof）：
  - `custom_model_override`
  - `prompt_overrides`
  - `feature_flags` (JSONB)

**验收标准：** 可通过 API CRUD 租户

---

### 2.3 API Key 管理增强

（见 2.1，已包含在 JWT 任务中）

---

**阶段 2 交付物：** JWT 认证、Tenant CRUD、API Key 生命周期管理
**预估：** 8 天

---

## 阶段 3：可观测性（对应 S3）

**目标评分：4.0/5**

### 3.1 [RELEASE BLOCKER #4] Prometheus Metrics

| 项目 | 值 |
|------|-----|
| 优先级 | P0 |
| 预估 | 3d |
| 现状 | 零 metrics endpoint，生产监控盲区 |

**任务：**
- [ ] 添加 `github.com/prometheus/client_golang` 依赖
- [ ] 实现 `internal/metrics/metrics.go`
  - **Agent metrics:**
    - `hermes_agent_requests_total{tenant_id, status}` — 请求计数
    - `hermes_agent_request_duration_seconds{tenant_id}` — 延迟直方图
    - `hermes_agent_llm_tokens_total{tenant_id, model, direction}` — input/output tokens
    - `hermes_agent_llm_cost_usd_total{tenant_id, model}` — 累计成本
  - **System metrics:**
    - `hermes_active_sessions{tenant_id}` — 活跃会话数
    - `hermes_redis_operations_total{operation}` — Redis 操作计数
    - `hermes_pg_pool_connections{state}` — PG 连接池状态
  - **Business metrics:**
    - `hermes_tenant_rate_limit_hits_total{tenant_id}` — rate limit 触发次数
    - `hermes_audit_events_total{tenant_id, action}` — 审计事件数
- [ ] 添加 `/metrics` endpoint（Prometheus 抓取端）
- [ ] 在关键路径埋点：
  - `agent/factory.go` — Run() 入口/出口
  - `store/pg/` — 所有 DB 操作
  - `middleware/ratelimit.go` — rate limit 判断

**验收标准：** `curl localhost:PORT/metrics` 返回 Prometheus 格式 metrics

---

### 3.2 Request ID 链路追踪

| 项目 | 值 |
|------|-----|
| 优先级 | P1 |
| 预估 | 1d |

**任务：**
- [ ] `middleware/requestid.go`
  - 检查请求头 `X-Request-ID`，如果不存在则生成 UUIDv4
  - 将 request ID 注入 context
  - 在所有 log 语句中包含 request ID
  - 在 response header 中返回 `X-Request-ID`
- [ ] 在 `agent/factory.go` 和 `store/` 的日志中加入 request ID 字段

**验收标准：** 每个请求的日志可以通过 request ID 串联

---

### 3.3 Liveness / Readiness 探测区分

| 项目 | 值 |
|------|-----|
| 优先级 | P1 |
| 预估 | 0.5d |

**任务：**
- [ ] `/health/live` — 仅检查进程存活（goroutine 不死锁）
- [ ] `/health/ready` — 检查 DB + Redis 连接可用
- [ ] 更新 `docker-compose.dev.yml` 中的 healthcheck 配置

---

**阶段 3 交付物：** Prometheus metrics、链路追踪、生产 health check
**预估：** 4.5 天

---

## 阶段 4：API 治理（对应 S4）

**目标评分：4.2/5**

### 4.1 [RELEASE BLOCKER #7] OpenAPI Spec

| 项目 | 值 |
|------|-----|
| 优先级 | P0 |
| 预估 | 2d |
| 现状 | 无 API 文档，外部开发者无法集成 |

**任务：**
- [ ] 使用 `github.com/swaggo/swag` 生成 OpenAPI 3.0 spec
- [ ] 为所有 `/v1/` endpoints 添加 Go doc comments
- [ ] 生成 `docs/openapi.yaml`
- [ ] 可选：集成 `swaggo/gin-swagger` 提供 `/v1/docs` UI

**验收标准：** `/v1/docs` 页面可查看完整 API 文档

---

### 4.2 Per-Tenant Rate Limit 执行

| 项目 | 值 |
|------|-----|
| 优先级 | P1 |
| 预估 | 1d |

**任务：**
- [ ] 从 `Tenant.RateLimitRPM` 读取 per-tenant 限制（已有字段）
- [ ] 确保 middleware 从 PG 实时读取（不缓存，或短 TTL 缓存）
- [ ] 添加 per-tenant 用量查询：`GET /v1/tenants/{id}/usage`

---

### 4.3 Billing 用量 API

| 项目 | 值 |
|------|-----|
| 优先级 | P1 |
| 预估 | 2d |

**任务：**
- [ ] 实现 per-tenant 用量聚合 SQL
  - 当日/本周/本月 token 用量
  - 当日/本周/本月 cost 估算
  - 按 model 分组
- [ ] `GET /v1/tenants/{id}/billing/usage`
  - Query params: `period=daily|weekly|monthly&start_date=&end_date=`
  - 返回：token count, estimated cost, by model breakdown
- [ ] `GET /v1/tenants/{id}/billing/invoices`（mock 数据，stripe 集成在 S5）

---

### 4.4 配额强制执行

| 项目 | 值 |
|------|-----|
| 优先级 | P1 |
| 预估 | 1d |

**任务：**
- [ ] 在创建 Session 前检查 `Tenant.MaxSessions`
- [ ] 超限返回 `402 Payment Required` 或 `429 Too Many Requests`
- [ ] 在 middleware 中统一处理

---

**阶段 4 交付物：** OpenAPI 文档、per-tenant rate limit、用量 API、配额执行
**预估：** 6 天

---

## 阶段 5：生产化（对应 S5）

**目标评分：4.5/5**

### 5.1 Helm Chart

| 项目 | 值 |
|------|-----|
| 优先级 | P1 |
| 预估 | 2d |

**任务：**
- [ ] 创建 `deploy/helm/hermes-agent/`
  - `Chart.yaml`
  - `values.yaml` — 镜像、副本数、PG/Redis 连接、resources
  - `templates/deployment.yaml`
  - `templates/service.yaml`
  - `templates/ingress.yaml`（可选）
  - `templates/hpa.yaml`（HPA 配置）
- [ ] 实现 HPA（Horizontal Pod Autoscaler）
  - 基于 `hermes_active_sessions` 或 CPU/memory 利用率自动伸缩

---

### 5.2 TLS 配置

| 项目 | 值 |
|------|-----|
| 优先级 | P1 |
| 预估 | 1d |

**任务：**
- [ ] Gateway 支持 TLS termination
- [ ] 配置项：`server.tls.enabled`, `server.tls.cert_file`, `server.tls.key_file`
- [ ] 支持 ACME（Let's Encrypt）自动证书

---

### 5.3 Secrets Manager 集成

| 项目 | 值 |
|------|-----|
| 优先级 | P2 |
| 预估 | 2d |

**任务：**
- [ ] 定义 `SecretStore` interface
- [ ] 实现 HashiCorp Vault provider
- [ ] 实现 AWS Secrets Manager provider
- [ ] 迁移所有 hardcoded secrets（DB password, JWT secret, API keys）到 vault

---

### 5.4 GDPR 合规

| 项目 | 值 |
|------|-----|
| 优先级 | P2 |
| 预估 | 2d |

**任务：**
- [ ] 实现数据导出 API：`GET /v1/users/{id}/export`
  - 导出该用户所有数据为 JSON
  - 包含 sessions, messages, audit logs
- [ ] 实现数据删除 API：`DELETE /v1/users/{id}/data`（right-to-erasure）
  - 软删除（设置 `deleted_at` 时间戳）
  - 级联删除该用户的 messages
- [ ] 添加 `PII` 标记字段到 User struct
- [ ] 数据保留策略：audit_logs 保留 2 年后自动清理

---

### 5.5 CI 安全扫描

| 项目 | 值 |
|------|-----|
| 优先级 | P2 |
| 预估 | 0.5d |

**任务：**
- [ ] 添加 `govulncheck` 到 `.github/workflows/ci.yml`
- [ ] 添加 Trivy 镜像扫描到 CI

---

**阶段 5 交付物：** Helm Chart、HPA、TLS、Secrets Manager、GDPR
**预估：** 7.5 天

---

## 任务总览

| 阶段 | 内容 | 任务数 | 预估 |
|------|------|--------|------|
| **阶段 0** | 并发 Bug 修复（8 项） | 8 | 5d |
| **阶段 1** | 接入层补齐（audit log + RBAC + rate limit） | 4 | 4.5d |
| **阶段 2** | 认证升级（JWT + Tenant CRUD + API key） | 2 | 8d |
| **阶段 3** | 可观测性（Prometheus + tracing + health） | 3 | 4.5d |
| **阶段 4** | API 治理（OpenAPI + billing + quota） | 4 | 6d |
| **阶段 5** | 生产化（Helm + TLS + Vault + GDPR） | 5 | 7.5d |
| **总计** | | **26** | **~35.5d ≈ 7-8 周** |

---

## 优先级排序（直接对应 7 个 Release Blockers）

| 优先级 | 阻塞项 | 所属阶段 | 任务 |
|--------|--------|----------|------|
| P0 | #1 audit_logs 从未写入 | S1 | 1.1 |
| P0 | #2 RBAC 未执行 | S1 | 1.2 |
| P0 | #3 Rate limit 未接入 | S1 | 1.3 |
| P0 | #4 无 Prometheus metrics | S3 | 3.1 |
| P0 | #5 无 Tenant CRUD API | S2 | 2.2 |
| P0 | #6 无 JWT/OIDC | S2 | 2.1 |
| P0 | #7 无 OpenAPI spec | S4 | 4.1 |

---

## 下一步行动

1. **立即执行**：阶段 0（8 个并发 bug）— 1 周
2. **并行执行 S1**：audit log + RBAC + rate limit — 1 周
3. **S2 准备**：JWT + Tenant CRUD 可以与 S1 并行准备 design doc
4. **建议：** 每阶段结束时有可演示的成果，而非最后一次性交付

---

*最后更新：2026-04-27*
*依据：`saas-readiness-audit.md` + `prd.md`*
