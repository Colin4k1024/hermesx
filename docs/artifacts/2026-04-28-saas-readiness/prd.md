# PRD: Hermes Agent Go — SaaS Readiness Phase 1-5

| 字段 | 值 |
|------|-----|
| Slug | `saas-readiness` |
| 日期 | 2026-04-28 |
| 主责 | tech-lead |
| 状态 | plan |
| 阶段 | plan |
| 前置依赖 | `2026-04-27-saas-stateless` (Phase 0-2 已交付, closed) |
| 审计基线 | `saas-readiness-audit.md` — 当前评分 2.6/5，目标 4.5/5 |

---

## 背景

hermes-agent-go 已完成 SaaS 无状态化 MVP（Phase 0-2）：
- Phase 0: 并发缺陷修复（8 项，通过架构重设计消除）
- Phase 1: 状态外置（Store interface + PG/SQLite 双实现 + Redis + Tenant middleware）
- Phase 2: Gateway 无状态化（AgentFactory + ContextLoader）

但 SaaS 企业发布标准仍有 **重大缺口**（审计评分 2.6/5）。7 个 Release Blockers 全部未实现，接入层、认证、可观测性、API 治理和生产化能力均缺失。

**触发原因：** 项目需要从"可多实例部署"提升到"企业 SaaS 可发布"，补齐安全、审计、计费、监控和运维基础设施。

---

## 代码现状验证（2026-04-28 实际读取）

| 检查项 | 结果 | 说明 |
|--------|------|------|
| Store interface | 存在 | `internal/store/store.go` — SessionStore/MessageStore/UserStore |
| TenantStore interface | **不存在** | 无法通过 API CRUD 租户 |
| AuditLogStore | **不存在** | `audit_logs` 表有 DDL，零写入代码 |
| RBAC middleware | **不存在** | `internal/middleware/` 仅 `tenant.go` |
| Rate limit middleware | **不存在** | Redis rate limit 代码未搜到 |
| JWT/Auth 模块 | **不存在** | 仅 Bearer token 静态校验 |
| Prometheus metrics | **不存在** | 无 `/metrics` endpoint、无 `prometheus/client_golang` |
| OpenAPI spec | **不存在** | 无 swagger/swag 依赖 |
| Helm Chart | **不存在** | 仅 Docker Compose |
| Tenant struct | 存在 | `internal/store/types.go` — 含 Plan/RateLimitRPM/MaxSessions |
| PG migrations | 存在 | `internal/store/pg/migrate.go` — 6 表（tenants/sessions/messages/users/audit_logs/cron_jobs） |

---

## 目标与成功标准

### 业务目标
- 将 SaaS 就绪评分从 **2.6/5 提升到 4.5/5**
- 消除全部 7 个 Release Blockers
- 达到企业 SaaS 核心平台发布标准

### 成功标准
- audit_logs 表有数据写入，管理员可查询
- 非 admin 用户访问 admin 接口返回 403
- 超过 RPM 限制的请求返回 429 + Retry-After
- `curl localhost:PORT/metrics` 返回 Prometheus 格式 metrics
- JWT 可登录、可刷新；API key 可创建/验证/撤销
- 可通过 API CRUD 租户
- `/v1/docs` 页面可查看完整 API 文档
- `go build ./...` 零编译错误、`go test -race ./...` 零竞态

---

## 关键假设

| # | 假设 | 风险 | 验证方式 |
|---|------|------|---------|
| A1 | JWT RS256 密钥由部署时配置注入，不编译内置 | 低 | 配置文件 + 环境变量 |
| A2 | RBAC 三级角色 `user < operator < admin` 足够 MVP | 中 | 后续可扩展为 policy-based |
| A3 | Rate limit 基于 Redis 滑动窗口，单 Redis 实例即可 | 低 | Redis Sentinel 作为 P2 增强 |
| A4 | OpenAPI 通过 Go doc comments + swag 生成，不手写 YAML | 低 | 业界标准做法 |
| A5 | Billing 用量 API 为 read-only 聚合，不含 Stripe 集成 | 中 | Stripe 集成延后到独立任务 |
| A6 | Helm Chart 仅覆盖单集群部署，不含多地域 | 低 | 多地域作为 P2 增强 |
| A7 | GDPR data export 为同步 API，大数据量场景后续改异步 | 中 | MVP 限制单次导出上限 |

---

## 最小可行范围 (MVP) 与非目标

### MVP — 5 Phase 递进交付

| Phase | 名称 | 核心产出 | 目标评分 |
|-------|------|---------|---------|
| **P1** | 接入层补齐 | Audit log 写入 + RBAC middleware + Rate limit 接入 + Health 增强 | 3.5/5 |
| **P2** | 认证升级 | JWT 签发/验证 + Tenant CRUD API + API Key 管理 | 3.8/5 |
| **P3** | 可观测性 | Prometheus metrics + Request ID 链路 + Liveness/Readiness 探测 | 4.0/5 |
| **P4** | API 治理 | OpenAPI spec + Per-tenant rate limit + Billing 用量 API + 配额执行 | 4.2/5 |
| **P5** | 生产化 | Helm Chart + TLS + Secrets Manager + GDPR + CI 安全扫描 | 4.5/5 |

### 非目标
- SSO/OIDC 对接（SAML、Okta、Google Workspace）
- Stripe/支付网关集成
- 前端 Dashboard SaaS 化改造
- Skills/Plugins 多租户隔离
- 多地域部署 / 跨集群同步
- PG Row-Level Security（应用层隔离已足够 MVP）
- Elasticsearch 全文搜索替换

---

## 7 个 Release Blockers 执行顺序与依赖

```
依赖关系图：

[P1] ─────────────────────────────────────────────────┐
  ├─ RB#1 Audit Log 写入 (无依赖)                      │
  ├─ RB#2 RBAC Middleware (无依赖)                      │
  └─ RB#3 Rate Limit 接入 (无依赖)                     │
       │                                                │
[P2] ──┤ (依赖 P1: RBAC 执行后才能保护 Tenant API)      │
  ├─ RB#5 Tenant CRUD API (依赖 RB#2 RBAC)             │
  └─ RB#6 JWT/认证 (依赖 RB#5 Tenant)                  │
       │                                                │
[P3] ──┤ (可与 P2 并行，但 metrics 依赖 P1 rate limit)  │
  └─ RB#4 Prometheus Metrics (依赖 RB#3 接入)           │
       │                                                │
[P4] ──┤ (依赖 P2+P3)                                  │
  └─ RB#7 OpenAPI Spec (依赖全部 API 稳定)              │
       │                                                │
[P5] ──┘ (依赖 P1-P4 全部完成)
```

**建议执行顺序：**

| 执行批次 | Release Blocker | 任务 | 预估 | 可并行 |
|----------|----------------|------|------|--------|
| Batch 1 | RB#1 + RB#2 + RB#3 | Audit Log + RBAC + Rate Limit | 4.5d | 三者无依赖，可并行 |
| Batch 2 | RB#5 → RB#6 | Tenant CRUD → JWT | 8d | 串行（Tenant 先于 JWT） |
| Batch 3 | RB#4 | Prometheus Metrics | 4.5d | 可与 Batch 2 并行 |
| Batch 4 | RB#7 | OpenAPI Spec | 2d | 需等 API 稳定 |

---

## 任务拆分（18 个可执行任务）

### Phase 1: 接入层补齐（4 任务，4.5d）

| # | 任务 | 优先级 | 预估 | 依赖 | 验收标准 |
|---|------|--------|------|------|---------|
| T1 | AuditLogStore 实现 + 关键路径埋点 | P0 | 2d | 无 | `audit_logs` 表有数据写入；`GET /v1/audit-logs` 可查询 |
| T2 | RBAC Middleware 实现 + 路由挂载 | P0 | 1d | 无 | 非 admin 访问 admin 接口返回 403 |
| T3 | Rate Limit Middleware 实现 + 路由挂载 | P0 | 1d | 无 | 超限返回 429 + `Retry-After` + `X-RateLimit-*` headers |
| T4 | Health Endpoint 增强（liveness/readiness） | P1 | 0.5d | 无 | `/health/ready` 检查 DB+Redis；`/health/live` 仅检查进程 |

### Phase 2: 认证升级（2 任务，8d）

| # | 任务 | 优先级 | 预估 | 依赖 | 验收标准 |
|---|------|--------|------|------|---------|
| T5 | Tenant CRUD API + TenantStore 实现 | P0 | 3d | T2 | 5 个 REST endpoint 可 CRUD 租户；创建时生成 API key |
| T6 | JWT 签发/验证 + API Key 管理 + Refresh | P0 | 5d | T5 | JWT 登录/刷新；API Key 创建/列出/撤销；RS256 签名 |

### Phase 3: 可观测性（3 任务，4.5d）

| # | 任务 | 优先级 | 预估 | 依赖 | 验收标准 |
|---|------|--------|------|------|---------|
| T7 | Prometheus Metrics 实现 + `/metrics` endpoint | P0 | 3d | T3 | `curl /metrics` 返回 Prometheus 格式；含 agent/system/business 三类 |
| T8 | Request ID Middleware + 链路日志关联 | P1 | 1d | 无 | 每个请求日志可通过 request ID 串联；response header 含 `X-Request-ID` |
| T9 | Liveness/Readiness 探测区分 + Docker 配置 | P1 | 0.5d | T4 | `/health/live` vs `/health/ready` 语义正确；docker-compose 配置更新 |

### Phase 4: API 治理（4 任务，6d）

| # | 任务 | 优先级 | 预估 | 依赖 | 验收标准 |
|---|------|--------|------|------|---------|
| T10 | OpenAPI 3.0 Spec 生成 + Swagger UI | P0 | 2d | T5,T6 | `/v1/docs` 可查看完整 API 文档 |
| T11 | Per-Tenant Rate Limit 执行 | P1 | 1d | T3,T5 | 从 `Tenant.RateLimitRPM` 读取限制；`GET /v1/tenants/{id}/usage` |
| T12 | Billing 用量 API | P1 | 2d | T5,T7 | `GET /v1/tenants/{id}/billing/usage` 返回 token/cost 按 model 聚合 |
| T13 | 配额强制执行（MaxSessions） | P1 | 1d | T5 | 超限创建 Session 返回 402/429 |

### Phase 5: 生产化（5 任务，7.5d）

| # | 任务 | 优先级 | 预估 | 依赖 | 验收标准 |
|---|------|--------|------|------|---------|
| T14 | Helm Chart + HPA | P1 | 2d | T7,T9 | `helm install` 可部署；HPA 基于 metrics 自动伸缩 |
| T15 | TLS 配置支持 | P1 | 1d | 无 | 配置 `server.tls.*` 启用 HTTPS |
| T16 | Secrets Manager 集成 | P2 | 2d | T6 | 定义 SecretStore interface；Vault + AWS SM 双实现 |
| T17 | GDPR 合规（数据导出/删除） | P2 | 2d | T5 | `GET /v1/users/{id}/export` + `DELETE /v1/users/{id}/data` |
| T18 | CI 安全扫描（govulncheck + Trivy） | P2 | 0.5d | 无 | CI pipeline 包含漏洞扫描 |

---

## 新增 Store Interface 规划

当前 `store.Store` interface 仅含 `Sessions()/Messages()/Users()`。Phase 1-2 需要扩展：

```go
type Store interface {
    Sessions() SessionStore
    Messages() MessageStore
    Users()    UserStore
    Tenants()  TenantStore   // NEW: Phase 2
    AuditLogs() AuditLogStore // NEW: Phase 1
    APIKeys()  APIKeyStore   // NEW: Phase 2
    Close() error
    Migrate(ctx context.Context) error
}
```

**影响面：** `internal/store/store.go`、`internal/store/pg/pg.go`、`internal/store/sqlite/sqlite.go`、`internal/store/factory.go`

---

## 新增 Middleware 规划

当前 `internal/middleware/` 仅含 `tenant.go`。需新增：

| 文件 | 职责 | Phase |
|------|------|-------|
| `rbac.go` | Role-based access control | P1 |
| `ratelimit.go` | Per-tenant rate limiting | P1 |
| `requestid.go` | Request ID 生成与传播 | P3 |
| `jwt.go` | JWT 验证与 context 注入 | P2 |

---

## 参与角色清单

| 角色 | 职责 | 输入缺口 |
|------|------|---------|
| `tech-lead` | 整体架构决策、Phase 收口、Release 判断 | 无 |
| `backend-engineer` | 全部 18 个任务的实现 | JWT RS256 密钥管理方案需确认 |
| `qa-engineer` | 集成测试、安全测试、放行建议 | 需 Docker PG/Redis 测试环境 |
| `devops-engineer` | Helm Chart、CI pipeline、TLS 配置 | K8s 目标集群信息待确认 |

---

## 待确认项

| # | 问题 | 影响 | 建议决策 | 状态 |
|---|------|------|---------|------|
| Q1 | JWT 密钥管理：RS256 密钥对存放位置？ | T6 实现方式 | 开发环境文件配置 + 生产环境 Vault | 待确认 |
| Q2 | Tenant 删除策略：硬删除还是软删除？级联范围？ | T5 实现方式 | 软删除（`deleted_at`），不级联删除 sessions | 待确认 |
| Q3 | API Key 格式与 hash 算法 | T6 安全性 | `hk_live_{tenant_id}_{random32}`，bcrypt 存储 | 建议采纳 |
| Q4 | Prometheus metrics 前缀命名 | T7 指标命名 | `hermes_` 前缀 | 建议采纳 |
| Q5 | K8s 目标集群与 namespace 约定 | T14 Helm 配置 | 待 devops 提供 | 待确认 |
| Q6 | GDPR 数据导出上限与格式 | T17 实现方式 | JSON 格式，单次上限 1GB | 待确认 |
| Q7 | CI 安全扫描的 fail threshold | T18 阻塞策略 | HIGH 以上阻塞 merge | 建议采纳 |

---

## 企业治理待确认项

| # | 维度 | 当前状态 | 需确认 |
|---|------|---------|--------|
| G1 | 应用等级 | 未评定 | 是否为 T2/T3 应用？影响 HA 和隔离要求 |
| G2 | 数据合规 | 无 GDPR 工具 | 是否有跨境数据要求？ |
| G3 | 集团组件 | 无约束 | 开源项目，无集团组件限制 |
| G4 | 技术架构等级 | 未评定 | 影响 Redis Sentinel/Cluster 选择 |

**结论：** 当前为开源 POC 项目，暂按 T3 基线执行，不强制集团组件约束。

---

## 领域技能包启用建议

| 技能 | 原因 | 阶段 |
|------|------|------|
| `golang-patterns` | Go 惯用模式（middleware chain, interface 设计） | 全程 |
| `golang-testing` | 表驱动测试、并发测试 | 全程 |
| `postgres-patterns` | PG 查询优化、索引策略 | P1-P2 |
| `api-design` | RESTful API 设计、错误码规范 | P2-P4 |
| `security-review` | JWT、RBAC、Rate Limit 安全审查 | P1-P2 |
| `docker-patterns` | Docker Compose、Helm Chart | P5 |

---

## UI 范围

**无前端变更。** 本任务纯后端 API 层建设，不涉及 Dashboard 或前端页面改造。

---

## 需求挑战会候选分组

### 分组 1: 接入层安全（T1 + T2 + T3）
- **挑战焦点：** RBAC 角色粒度是否足够？Rate limit 滑动窗口 vs 固定窗口？Audit log 覆盖范围？
- **参与者：** tech-lead, backend-engineer, qa-engineer

### 分组 2: 认证体系（T5 + T6）
- **挑战焦点：** JWT vs API Key 的使用场景划分？Token 刷新策略？密钥轮换方案？
- **参与者：** tech-lead, backend-engineer

### 分组 3: 可观测性与生产化（T7 + T14）
- **挑战焦点：** Metrics 命名约定？HPA 扩缩指标选择？告警阈值？
- **参与者：** tech-lead, backend-engineer, devops-engineer

---

## 总工期与里程碑

| 里程碑 | 内容 | 预估 | 累计 |
|--------|------|------|------|
| M1: 接入层就绪 | P1 完成，评分 3.5/5 | 1 周 | 1 周 |
| M2: 认证体系就绪 | P2 完成，评分 3.8/5 | 2 周 | 3 周 |
| M3: 可观测性就绪 | P3 完成，评分 4.0/5 | 1 周 | 4 周 |
| M4: API 治理就绪 | P4 完成，评分 4.2/5 | 1.5 周 | 5.5 周 |
| M5: 生产化就绪 | P5 完成，评分 4.5/5 | 2 周 | 7.5 周 |

**总预估：7-8 周，18 个任务，~35.5 工作日**

---

## 下一步

1. 确认待确认项 Q1-Q7
2. 进入 `/team-plan` 生成 Delivery Plan
3. Phase 1（T1-T4）可立即进入 design + implement

---

*最后更新：2026-04-28*
*来源：`saas-readiness-audit.md` (2.6/5) + `tasks-breakdown.md` (26 tasks) + 代码实际读取验证*
