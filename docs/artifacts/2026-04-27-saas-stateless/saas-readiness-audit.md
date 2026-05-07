# Enterprise SaaS Readiness Audit Report

| 字段 | 值 |
|------|-----|
| 日期 | 2026-04-27 |
| 审查人 | tech-lead |
| 项目 | hermesx |
| 基线 commit | d11efed |
| 总评 | **2.6/5** — 基础架构就绪，业务能力层存在关键缺口 |

---

## 评分总览

| # | 维度 | 评分 | 现状摘要 | 最大缺口 |
|---|------|------|---------|---------|
| 1 | Scalability | **4/5** | AgentFactory 无状态、Redis 分布式锁、PG 连接池、3 副本 Docker Compose | 无 HPA/Helm |
| 2 | Security Hardening | **4/5** | 5 层安全模块（exfiltration/redact/git injection/credential guard/tool validation） | 无 TLS、无 Secrets Manager |
| 3 | Multi-tenancy | **3/5** | Store 接口全量 tenant_id、PG 表结构有 FK、tenant middleware | 无 Tenant CRUD API、无 PG RLS |
| 4 | Operations | **3/5** | Dockerfile/CI/Docker Compose/结构化日志 | 无 Helm、无安全扫描 |
| 5 | Auth & Authorization | **2/5** | Bearer token、User.Role 字段存在 | 无 JWT/OIDC、RBAC 未执行 |
| 6 | Observability | **2/5** | slog 全覆盖、3 个 health endpoint | 无 Prometheus、无 Tracing |
| 7 | API Management | **2/5** | /v1/ 路由、OpenAI 兼容 API、Rate limit 代码 | Rate limit 未接入、无 OpenAPI |
| 8 | Billing & Metering | **2/5** | Token 追踪、Cost 估算、Tenant.Plan 字段 | 无用量聚合、无 Stripe 集成 |
| 9 | Compliance & Audit | **2/5** | audit_logs 表结构、Session export | 审计日志从未写入、无 GDPR |
| 10 | User Management | **2/5** | User 表、Approve/Revoke 流程 | 无邀请流程、无团队管理 |

---

## 详细分析

### 1. Multi-tenancy (3/5)

**已有：**
- `store/types.go` — `Tenant` struct（id, name, plan, rate_limit_rpm, max_sessions）
- `store/store.go` — SessionStore/MessageStore/UserStore 所有方法首参 `tenantID`
- `store/pg/migrate.go` — `tenants` 表 UUID PK，sessions/messages/users 表均有 `tenant_id FK`
- `store/pg/sessions.go` — 所有查询 `WHERE tenant_id = $1`
- `middleware/tenant.go` — HTTP middleware 读取 `X-Tenant-ID`，regex 校验 `^[a-zA-Z0-9_-]{1,64}$`

**缺失：**
- 无 `TenantStore` 接口 — 无法通过 API 创建/管理租户
- 无 PG Row-Level Security (RLS) — 隔离仅靠应用层查询参数
- 缺少 tenant 级配置（自定义 model、prompt 覆写、feature flag）
- 无 tenant 入驻/开通流程

### 2. Authentication & Authorization (2/5)

**已有：**
- `acp/auth.go` — Bearer token 认证（`crypto/subtle` 常量时间比较）
- `store/pg/users.go` — `IsApproved`/`Approve`/`Revoke` 平台用户白名单
- `store/types.go` — `User.Role` 字段（`user`/`admin`）

**缺失：**
- 无 JWT/OAuth2/OIDC — 无 token 签发、过期、刷新
- 无 SSO 集成（SAML、Google Workspace、Okta）
- RBAC 未执行 — `Role="admin"` 存了但无中间件检查
- 无 API key 管理（per-tenant 创建/轮换/撤销）
- 单一静态 bearer token — 多租户共享同一 token

### 3. Scalability (4/5)

**已有：**
- `agent/factory.go` — AgentFactory 无状态执行（load → run → persist）
- `store/rediscache/redis.go` — 分布式锁（Lua compare-and-delete）、Rate Limit、Context Cache
- `docker-compose.dev.yml` — PG 16 + Redis 7 + 3x Gateway replicas
- `store/pg/pg.go` — pgxpool 连接池

**缺失：**
- 无 HPA/Kubernetes 自动伸缩
- Lock token 用 `time.Now().UnixNano()` 非密码学随机
- 无读副本/分片策略

### 4. Security Hardening (4/5)

**已有：**
- `agent/exfiltration.go` — URL token/base64/data URI/prompt injection 检测（17+ 模式）
- `agent/redact.go` — 24 编译正则覆盖主流云服务密钥
- `agent/reasoning_guard.go` — 跨 provider 推理泄漏阻断（allowlist 模式）
- `tools/credential_guard.go` — 凭证目录保护（.ssh/.aws/.docker/.kube 等）
- `tools/git_security.go` — Git 参数注入防护（9 正则模式）
- `agent/tool_validation.go` — 截断 tool call 检测
- ACP/API Server 绑定 `127.0.0.1`

**缺失：**
- 无 TLS/HTTPS 配置
- 无加密存储（依赖基础设施级磁盘加密）
- 无 Secrets Manager 集成（Vault/AWS SM）
- 无 HTTP body size limit（DoS 向量）

### 5. Observability (2/5)

**已有：**
- `log/slog` 结构化日志全覆盖（82 文件）
- 3 个 health endpoint（ACP/Dashboard/API Server）
- Redis `SetInstanceStatus` 实例心跳

**缺失：**
- 无 Prometheus metrics（无 `/metrics` 端点、无 `prometheus/client_golang`）
- 无分布式追踪（无 OpenTelemetry/Jaeger）
- Health endpoint 无 readiness 探测（无 DB/Redis ping）
- 无 request ID 链路关联
- 无 Grafana Dashboard

### 6. API Management (2/5)

**已有：**
- `/v1/` 版本化路由
- OpenAI 兼容 `/v1/chat/completions` 端点
- PG store 分页（LIMIT/OFFSET + total count）
- Redis Rate Limit 实现

**缺失：**
- 无 OpenAPI/Swagger spec
- Rate Limit 代码实现但**未接入任何 HTTP handler**
- 无 API 版本升级策略
- 无 schema 校验
- 无 API 开发者文档

### 7. Billing & Metering (2/5)

**已有：**
- `store/types.go` — Session 跟踪 input/output tokens + estimated_cost_usd
- `agent/pricing.go` — 12 模型定价表
- `store/pg/sessions.go` — 原子递增 token 计数器
- `Tenant.Plan`/`RateLimitRPM`/`MaxSessions` 字段

**缺失：**
- 无 per-tenant 用量聚合 SQL
- 无 Stripe/支付集成
- 配额字段存了但**未执行**（max_sessions 从未检查）
- 无计费周期/发票
- 无用量导出 API

### 8. Compliance & Audit (2/5)

**已有：**
- `store/pg/migrate.go` — `audit_logs` 表（tenant_id, user_id, session_id, action, detail）
- `state/export.go` — Session JSON/Markdown 导出

**缺失：**
- **audit_logs 表从未写入** — 零 INSERT 调用
- 无 GDPR 合规工具（right-to-erasure、PII 识别）
- 无数据保留策略（无 TTL 清理 job）
- 仅硬删除，无软删除/墓碑模式
- export.go 操作本地 SQLite，非 PG 多租户 store

### 9. Operations (3/5)

**已有：**
- `Dockerfile` — 多阶段构建、非 root 用户、静态二进制
- `docker-compose.dev.yml` — PG + Redis + 3x Gateway + health check
- `.github/workflows/ci.yml` — build/vet/test/race/cross-compile/docker
- ACP server graceful shutdown（5s timeout）
- Store factory driver 注册模式

**缺失：**
- 无 Helm Chart
- 无 liveness vs readiness 探测区分
- CI 无安全扫描（govulncheck/Trivy）
- 无集中式配置服务（Consul/etcd）

### 10. User Management (2/5)

**已有：**
- `store/pg/users.go` — GetOrCreate/IsApproved/Approve/Revoke/ListApproved
- `store/types.go` — User struct（role, approved_at, metadata）
- `store/pg/migrate.go` — users 表 unique index on (tenant_id, external_id)

**缺失：**
- 无邀请流程（invite token/email onboarding）
- 无团队/组织管理（group/workspace）
- RBAC 存了但**未执行**
- 无自助用户管理（改名/改邮箱）
- 无用户停用（仅 Revoke 清空 approved_at）

---

## 7 个发布阻塞项

| # | 问题 | 现状 | 影响 | 建议工作量 |
|---|------|------|------|-----------|
| 1 | audit_logs 从未写入 | 表存在但零 INSERT | 合规审计完全缺失 | 2d |
| 2 | RBAC 未执行 | Role="admin" 存了没检查 | 任何 approved 用户可执行管理操作 | 1d |
| 3 | Rate limit 未接入 HTTP | CheckRateLimit 实现了没 handler 调用 | DDoS/滥用无防护 | 1d |
| 4 | 无 Prometheus metrics | 零 metrics endpoint | 生产监控盲区 | 3d |
| 5 | 无 Tenant CRUD API | 无法通过 API 创建/管理租户 | 多租户完全不可用 | 3d |
| 6 | 无 JWT/OIDC 认证 | 仅 static bearer token | 多租户认证不隔离 | 5d |
| 7 | 无 OpenAPI spec | 无 API 文档 | 外部开发者无法集成 | 2d |

---

## 已就绪的强项

1. **无状态执行模型** — AgentFactory.Run() 真正无状态，Redis lock 防并发
2. **安全纵深** — 5 层安全模块覆盖 exfiltration/redaction/injection/credential/tool validation
3. **Store 抽象** — interface + PG/SQLite 双实现，tenant_id 全链路传递
4. **CI 管线** — build/vet/test/race/cross-compile/docker 全自动
5. **Transport 可插拔** — 5 个 LLM provider transport，新增 provider 只需实现接口
6. **13 Gateway 平台** — 全球 + 中国市场主流平台覆盖

---

## 达到企业 SaaS 发布标准的路线图

| Phase | 内容 | 预估 | 目标评分 |
|-------|------|------|---------|
| S1 | 接入层补齐：audit log 写入 + RBAC middleware + rate limit 接入 | 1 周 | 3.5/5 |
| S2 | 认证升级：JWT 签发/验证 + Tenant CRUD API + API key 管理 | 2 周 | 3.8/5 |
| S3 | 可观测性：Prometheus metrics + request ID 链路 + liveness/readiness | 1-2 周 | 4.0/5 |
| S4 | API 治理：OpenAPI spec 生成 + rate limit per-tenant + billing 用量 API | 2 周 | 4.2/5 |
| S5 | 生产化：Helm Chart + TLS + Secrets Manager + GDPR data export | 2-3 周 | 4.5/5 |

**预计 8-10 周可将总评从 2.6 提升到 4.5/5，达到企业 SaaS 发布标准。**

---

*审查基准：基于代码实际读取，非文件名推测。所有评分基于企业 SaaS 核心平台标准。*
