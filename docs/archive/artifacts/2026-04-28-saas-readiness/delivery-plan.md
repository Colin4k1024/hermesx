# Delivery Plan: HermesX — SaaS Readiness Phase 1-5

| 字段 | 值 |
|------|-----|
| Slug | `saas-readiness` |
| 日期 | 2026-04-28 |
| 主责 | tech-lead |
| 状态 | draft |
| 阶段 | plan |
| 关联 PRD | `docs/artifacts/2026-04-28-saas-readiness/prd.md` |
| 关联架构 | `docs/artifacts/2026-04-28-saas-readiness/arch-design.md` |

---

## 版本目标

- **版本：** SaaS Readiness v1.0
- **范围：** 18 个任务 + 5 个新增前置任务 = 23 个任务，5 Phase 递进交付
- **放行标准：** SaaS 审计评分 ≥ 4.5/5，7 个 Release Blockers 全部消除，`go test -race ./...` 零竞态

---

## Requirement Challenge Session 结论

### 参与者

| 角色 | 代号 | 职责范围 |
|------|------|---------|
| architect | `challenge-architect` | 系统边界、组件拆分、接口约定、技术选型、风险与约束 |
| backend-engineer | `challenge-backend` | Store 可行性、中间件实现风险、JWT 迁移、任务缺口 |

### 质疑汇总（8 项，5 项导致计划修改）

#### CHL-1: Migration 无版本控制（CRITICAL — 导致新增前置任务 T0a）

- **质疑人：** architect + backend-engineer（双方独立提出）
- **质疑内容：** `internal/store/pg/migrate.go` 使用 `[]string` + `CREATE TABLE IF NOT EXISTS`，无版本跟踪。P2-P5 需要 ALTER 语句添加列/表，`CREATE IF NOT EXISTS` 会静默跳过已存在的表，导致新列不会被添加到已部署的实例上。
- **阻断条件：** P1 之后的任何 schema 变更都会在已有部署上静默失败。
- **结论：** **接受质疑，新增 T0a — 引入 golang-migrate 或手动 `schema_version` 表，作为 P1 的硬性前置。**

#### CHL-2: Middleware Chain 排序 — Auth 必须先于 Tenant（CRITICAL — 导致 P1 设计修改）

- **质疑人：** architect + backend-engineer（双方独立提出）
- **质疑内容：** 当前 `TenantMiddleware` 无条件接受 `X-Tenant-ID` header，默认 `"default"`。若 Rate Limit 和 RBAC 在 Tenant 之后执行，攻击者可以声称任意 tenant ID 来耗尽他人的 rate limit 配额。Audit log 会记录虚假的 tenant 身份。
- **替代路径：** Auth → Tenant（从 JWT claims 派生），仅在 authenticated principal 有 `cross-tenant` 权限时才信任 header。
- **结论：** **接受质疑。P1 中间件链顺序修正为：CORS → RequestID → Auth → Tenant → RBAC → RateLimit → Handler。Tenant 从认证后的身份派生，不再无条件信任 header。**

#### CHL-3: Store Interface 扩展 — SQLite 编译中断（HIGH — 导致新增前置任务 T0b）

- **质疑人：** backend-engineer
- **质疑内容：** 向 `Store` interface 添加 `Tenants()`/`AuditLogs()`/`APIKeys()` 会导致 SQLite 实现无法编译。`state.SessionDB` 无 tenant 概念，所有本地开发者会被阻塞。
- **结论：** **接受质疑，新增 T0b — 在 P1 首个 commit 一次性添加所有 sub-store accessors + SQLite no-op 实现 + 编译时接口断言 `var _ store.Store = (*SQLiteStore)(nil)`。**

#### CHL-4: JWT 向后兼容窗口（HIGH — 导致 P2 设计修改）

- **质疑人：** backend-engineer
- **质疑内容：** `withAuth` 在 `HERMES_ACP_TOKEN` 为空时允许所有请求（dev 模式）。迁移期间，若部署设置了 `HERMES_ACP_TOKEN` 但尚未签发 JWT，所有现有 API key 客户端会被 403。
- **替代路径：** 显式双认证过渡模式 — 若 `HERMES_ACP_TOKEN` 已设置则接受；若 `HERMES_JWT_SECRET` 已设置则也接受有效 JWT；两者共存直到静态 token 被显式移除。
- **结论：** **接受质疑。P2 T6 必须实现 credential extractor chain（static token → JWT → API key），不允许一次性切换。**

#### CHL-5: API Server 中间件缺口（HIGH — 导致新增前置任务 T0c）

- **质疑人：** backend-engineer
- **质疑内容：** `internal/gateway/platforms/api_server.go` 有独立的 HTTP mux 和内联 auth 检查。P1 中间件链仅应用于 ACP server，不保护 OpenAI 兼容的 `/v1/chat/completions` 端点。Rate limit、audit、metrics 必须同时覆盖两个 server。
- **结论：** **接受质疑，新增 T0c — 提取统一 middleware stack builder，同时挂载到 ACP server 和 API server。**

#### CHL-6: Store Interface 策略 — 注册表模式 vs 显式方法（MEDIUM — 保留原方案）

- **质疑人：** architect
- **质疑内容：** 每次添加 sub-store 都是 interface breaking change。建议用 `Store.Sub(name) interface{}` + typed helper 避免接口变更。
- **反驳：** Go 的显式接口提供编译时安全性，注册表模式用运行时灵活性换取类型安全。仅 2 个 backend 实现，churn 成本低。
- **结论：** **保留原方案（显式方法），但在 P1 一次性 commit 所有 accessor（CHL-3 方案），避免跨 Phase 反复修改。**

#### CHL-7: Redis Rate Limit 降级策略（MEDIUM — 标记为 P1 待设计项）

- **质疑人：** architect
- **质疑内容：** Redis down 时 rate limit 是 fail-open（允许滥用）还是 fail-closed（阻塞所有流量）？
- **结论：** **接受质疑，P1 T3 设计必须定义降级策略（建议 fail-open + 告警 + 本地 fallback 计数器）。**

#### CHL-8: AuthContext 接口必须 P1 锁定（MEDIUM — 导致 P1 设计修改）

- **质疑人：** architect
- **质疑内容：** RBAC、audit、rate-limit 都消费身份信息。若 P1 RBAC 直接耦合到静态 token 的身份表达，P2 JWT 切换时 RBAC 需要重写。
- **结论：** **接受质疑。P1 定义 `AuthContext` struct（identity, tenant_id, roles, auth_method），即使 P1 只由 static token 填充。**

---

## Brownfield 上下文快照

| 维度 | 现状 |
|------|------|
| **Store interface** | `store.Store` 含 `Sessions()/Messages()/Users()`；PG 实现有编译时断言 `var _ store.Store = (*PGStore)(nil)`；SQLite 无断言 |
| **认证** | `internal/acp/auth.go` — 静态 Bearer token + constant-time compare；API server 有独立内联 auth |
| **多租户** | `TenantMiddleware` 仅从 header 读取 `X-Tenant-ID`，默认 `"default"`；`Tenant` struct 已定义但无 store interface |
| **中间件** | 仅 `tenant.go`；ACP 用函数级 `withAuth` wrapper，非链式 middleware |
| **迁移** | `[]string` 平铺 DDL，无版本号，无 ALTER 支持 |
| **两个 HTTP Server** | ACP (`internal/acp/server.go`) 和 API (`internal/gateway/platforms/api_server.go`) 各自独立路由和 auth |
| **Redis** | `internal/store/rediscache/redis.go` — session lock（SETNX + Lua release），无 rate limit |
| **Metrics** | 应用级 token 计数（Session struct），无 Prometheus endpoint |
| **审计** | `audit_logs` 表 DDL 存在，零 INSERT 代码 |

---

## Story Slices（可执行工作单元）

### Phase 0 — 前置基础设施（新增，3 任务，2d）

| Slice | 任务 | 目标 | 验收标准 | 主责 | 依赖 |
|-------|------|------|---------|------|------|
| S0a | T0a: Migration 版本化 | 引入 numbered migration 方案 | `schema_version` 表存在；`runMigrations` 支持 ALTER；现有 DDL 不受影响 | backend-engineer | 无 |
| S0b | T0b: Store interface 一次性扩展 | 添加 `Tenants()/AuditLogs()/APIKeys()` + SQLite stubs | `go build ./...` 零错误；`var _ store.Store = (*SQLiteStore)(nil)` 编译通过 | backend-engineer | T0a |
| S0c | T0c: 统一 middleware stack | 提取共享 middleware builder，挂载到 ACP + API server | 两个 server 共享同一中间件链；现有测试不回归 | backend-engineer | 无 |

### Phase 1 — 接入层补齐（4 任务，5d）

| Slice | 任务 | 目标 | 验收标准 | 主责 | 依赖 |
|-------|------|------|---------|------|------|
| S1a | T0d: AuthContext struct 定义 | 定义统一身份上下文，P1 由 static token 填充 | `AuthContext{Identity, TenantID, Roles, AuthMethod}` 可从 context 提取；RBAC/audit/rate-limit 消费此结构 | backend-engineer | T0c |
| S1b | T1: AuditLogStore + 关键路径埋点 | `audit_logs` 表有写入 | `GET /v1/audit-logs` 可查询；session create/delete/auth failure 有审计记录 | backend-engineer | T0b |
| S1c | T2: RBAC Middleware | 三级角色访问控制 | 非 admin 访问 admin 接口返回 403；角色从 `AuthContext.Roles` 读取 | backend-engineer | S1a |
| S1d | T3: Rate Limit Middleware | Redis 滑动窗口 + fallback | 超限返回 429 + `Retry-After` + `X-RateLimit-*`；Redis down 时 fail-open + 告警 | backend-engineer | S1a |
| S1e | T4: Health Endpoint 增强 | Liveness/Readiness 区分 | `/health/ready` 检查 DB+Redis；`/health/live` 仅进程 | backend-engineer | 无 |

**Phase 1 目标评分：3.5/5**

### Phase 2 — 认证升级（2 任务，8d）

| Slice | 任务 | 目标 | 验收标准 | 主责 | 依赖 |
|-------|------|------|---------|------|------|
| S2a | T5: Tenant CRUD API + TenantStore | 租户管理 API | 5 个 REST endpoint；创建时自动 seed default admin user；RBAC 保护 | backend-engineer | S1c |
| S2b | T6: JWT + API Key + Credential Chain | 认证体系 | JWT 登录/刷新（RS256）；API Key 创建/列出/撤销；credential extractor chain（static → JWT → API key）共存 | backend-engineer | S2a |

**Phase 2 目标评分：3.8/5**

### Phase 3 — 可观测性（3 任务，4.5d）

| Slice | 任务 | 目标 | 验收标准 | 主责 | 依赖 |
|-------|------|------|---------|------|------|
| S3a | T7: Prometheus Metrics | `/metrics` endpoint | `hermes_*` 前缀；agent/system/business 三类指标；rate limit hit/miss 计数 | backend-engineer | S1d |
| S3b | T8: Request ID Middleware | 链路追踪 | `X-Request-ID` response header；日志含 request_id 字段 | backend-engineer | T0c |
| S3c | T9: Liveness/Readiness + Docker | 探测配置 | 语义正确的 K8s 探测；docker-compose healthcheck 更新 | backend-engineer | S1e |

**Phase 3 目标评分：4.0/5**

### Phase 4 — API 治理（4 任务，6d）

| Slice | 任务 | 目标 | 验收标准 | 主责 | 依赖 |
|-------|------|------|---------|------|------|
| S4a | T10: OpenAPI 3.0 + Swagger UI | API 文档 | `/v1/docs` 可查看完整 API 文档；swag comments 覆盖所有端点 | backend-engineer | S2b |
| S4b | T11: Per-Tenant Rate Limit | 租户级限流 | 从 `Tenant.RateLimitRPM` 读取限制；`GET /v1/tenants/{id}/usage` | backend-engineer | S1d, S2a |
| S4c | T12: Billing 用量 API | 计费数据 | `GET /v1/tenants/{id}/billing/usage` 返回 token/cost 按 model 聚合 | backend-engineer | S2a, S3a |
| S4d | T13: 配额强制执行 | MaxSessions | 超限创建 Session 返回 402/429 | backend-engineer | S2a |

**Phase 4 目标评分：4.2/5**

### Phase 5 — 生产化（5 任务，7.5d）

| Slice | 任务 | 目标 | 验收标准 | 主责 | 依赖 |
|-------|------|------|---------|------|------|
| S5a | T14: Helm Chart + HPA | K8s 部署 | `helm install` 可部署；HPA 基于 Prometheus metrics 自动伸缩 | devops-engineer | S3a, S3c |
| S5b | T15: TLS 配置 | HTTPS 支持 | `server.tls.*` 配置项启用 HTTPS | backend-engineer | 无 |
| S5c | T16: Secrets Manager 集成 | 密钥管理 | `SecretStore` interface；Vault + AWS SM 双实现 | backend-engineer | S2b |
| S5d | T17: GDPR 合规 | 数据导出/删除 | `GET /v1/users/{id}/export` + `DELETE /v1/users/{id}/data`；JSON 格式 | backend-engineer | S2a |
| S5e | T18: CI 安全扫描 | 供应链安全 | CI pipeline 含 govulncheck + Trivy；HIGH 以上阻塞 merge | devops-engineer | 无 |

**Phase 5 目标评分：4.5/5**

---

## 依赖关系图（更新后）

```
[P0 前置] ──────────────────────────────────────────────────────┐
  ├─ T0a Migration 版本化 (无依赖)                                │
  ├─ T0b Store interface 扩展 (→ T0a)                            │
  └─ T0c 统一 middleware stack (无依赖, 可与 T0a 并行)            │
       │                                                          │
[P1 接入层] ────────────────────────────────────────────────────┤
  ├─ T0d AuthContext struct (→ T0c)                               │
  ├─ T1 AuditLogStore (→ T0b)                                    │
  ├─ T2 RBAC Middleware (→ T0d)                                   │
  ├─ T3 Rate Limit Middleware (→ T0d)                             │
  └─ T4 Health Endpoint (无依赖)                                  │
       │                                                          │
[P2 认证] ─────────────────────────────────────────────────────┤
  ├─ T5 Tenant CRUD (→ T2)                                       │
  └─ T6 JWT + API Key (→ T5)                                     │
       │                                                          │
[P3 可观测] ──────────────────── (可与 P2 并行)                  │
  ├─ T7 Prometheus (→ T3)                                         │
  ├─ T8 Request ID (→ T0c)                                        │
  └─ T9 Liveness/Readiness (→ T4)                                │
       │                                                          │
[P4 API 治理] ─────────────────────────────────────────────────┤
  ├─ T10 OpenAPI (→ T6)                                           │
  ├─ T11 Per-Tenant Rate (→ T3, T5)                               │
  ├─ T12 Billing API (→ T5, T7)                                   │
  └─ T13 配额执行 (→ T5)                                         │
       │                                                          │
[P5 生产化] ──┘                                                    
  ├─ T14 Helm (→ T7, T9)
  ├─ T15 TLS (无依赖)
  ├─ T16 Secrets Manager (→ T6)
  ├─ T17 GDPR (→ T5)
  └─ T18 CI 安全扫描 (无依赖)
```

---

## 工作拆解

| Phase | 任务数 | 预估工作日 | 主责角色 | 里程碑检查点 |
|-------|--------|-----------|---------|-------------|
| P0 前置 | 3+1 | 2.5d | backend-engineer | Store 编译通过、migration 版本化验证 |
| P1 接入层 | 4 | 4.5d | backend-engineer | SaaS 评分 3.5/5、rate limit 429 验证 |
| P2 认证 | 2 | 8d | backend-engineer | JWT 登录/刷新可用、双认证共存 |
| P3 可观测 | 3 | 4.5d | backend-engineer | `/metrics` 可抓取、request ID 可追踪 |
| P4 API 治理 | 4 | 6d | backend-engineer | OpenAPI 可浏览、per-tenant 限流生效 |
| P5 生产化 | 5 | 7.5d | backend + devops | Helm 可部署、安全扫描通过 |
| **合计** | **23** | **~33d / 8-9 周** | | **SaaS 4.5/5** |

---

## 风险与缓解

| # | 风险 | 影响 | 概率 | 缓解措施 | Owner |
|---|------|------|------|---------|-------|
| R1 | Tenant 身份欺骗 | 高 | 已确认 | Auth → Tenant 排序（CHL-2），P1 必须修复 | backend-engineer |
| R2 | Migration 静默失败 | 高 | 已确认 | T0a 引入版本化迁移（CHL-1） | backend-engineer |
| R3 | API server 无中间件保护 | 高 | 已确认 | T0c 统一 middleware stack（CHL-5） | backend-engineer |
| R4 | JWT 切换导致全面 403 | 中 | 高 | Credential extractor chain 双认证共存（CHL-4） | backend-engineer |
| R5 | Redis 不可用导致 rate limit 失效 | 中 | 低 | Fail-open + 告警 + local fallback（CHL-7） | backend-engineer |
| R6 | SQLite 实现编译中断 | 中 | 已确认 | T0b 一次性扩展 + no-op stubs（CHL-3） | backend-engineer |
| R7 | P2 工期偏长（8d）阻塞后续 Phase | 中 | 中 | P3 可与 P2 并行；T5/T6 串行但各有独立验收 | tech-lead |
| R8 | K8s 目标集群信息缺失 | 低 | 中 | P5 开始前必须由 devops 提供（Q5） | devops-engineer |

---

## 角色分工

| 角色 | 任务范围 | 交接点 |
|------|---------|--------|
| tech-lead | 挑战会仲裁、Phase 收口、Release 判断、冲突升级 | 每 Phase 完成后 handoff review |
| backend-engineer | T0a-T0d、T1-T13、T15-T17（19 个任务） | 每 Phase 完成后 → qa-engineer |
| qa-engineer | 各 Phase 集成测试、安全验证、放行建议 | P5 → release 放行 |
| devops-engineer | T14（Helm）、T18（CI 安全扫描）、部署配置 | T14 依赖 P3 完成 |

---

## 节点检查

| 检查节点 | 时间点 | 检查内容 | 通过标准 |
|---------|--------|---------|---------|
| P0 完成 | W1 Day 3 | migration 版本化、Store 编译、middleware stack | `go build ./...` + migration 测试 |
| P1 收口 | W2 Day 5 | Audit/RBAC/Rate Limit/Health | 评分 ≥ 3.5/5、7 RB 中 3 个消除 |
| P2 中期 | W4 Day 3 | Tenant CRUD 完成、JWT 进行中 | Tenant API 5 端点可用 |
| P2 收口 | W5 Day 5 | 认证体系完整 | 双认证共存、API key CRUD |
| P3 收口 | W6 Day 3 | 可观测性完整 | `/metrics` 可抓取 |
| P4 收口 | W7 Day 4 | API 治理完整 | OpenAPI 可浏览、per-tenant 限流 |
| P5 收口 | W9 Day 3 | 生产化就绪 | Helm 可部署、安全扫描通过 |
| Release 门禁 | W9 Day 5 | 全面评审 | SaaS ≥ 4.5/5、7 RB 全部消除 |

---

## 待确认项决策状态

| # | 问题 | 挑战会结论 | 状态 |
|---|------|-----------|------|
| Q1 | JWT 密钥管理 | dev 文件配置 + prod Vault，P2 T6 实现 | **采纳** |
| Q2 | Tenant 删除策略 | 软删除（`deleted_at`），不级联 | **采纳** |
| Q3 | API Key 格式 | `hk_live_{tenant}_{random32}`，bcrypt | **采纳** |
| Q4 | Prometheus 前缀 | `hermes_` | **采纳** |
| Q5 | K8s 集群 | 待 devops 提供 | **待确认** |
| Q6 | GDPR 导出上限 | JSON，1GB 上限 | **采纳** |
| Q7 | 安全扫描阈值 | HIGH 以上阻塞 | **采纳** |
| Q8（新增） | Rate limit Redis 降级策略 | fail-open + 告警 + local fallback | **采纳** |
| Q9（新增） | Tenant 身份信任模型 | Auth 后派生，非 header 直接信任 | **采纳** |

---

## Karpathy Guidelines 收口检查

### 显式假设
- A1-A7（PRD 已列出），新增 A8: Auth → Tenant 排序是安全基线
- A9: SQLite 作为 dev-only backend，no-op stubs 可接受

### 更简单备选路径
- **已评估并拒绝：** Store 注册表模式（CHL-6，类型安全优先于灵活性）
- **已评估并拒绝：** 一次性切换 JWT 替换 static token（CHL-4，必须双认证共存）
- **已评估并采纳：** 一次性 Store interface commit（CHL-3，避免跨 Phase 反复破坏）

### 当前不做项
- SSO/OIDC、Stripe、前端 Dashboard、多地域、PG RLS、Elasticsearch（PRD 非目标）
- Redis Sentinel/Cluster（P1 单 Redis 足够，后续增强）
- Policy-based RBAC（三级角色 MVP 足够）

### 为什么本轮范围已经足够
- 23 个任务覆盖全部 10 个审计维度，从 2.6 提升到 4.5/5
- 7 个 Release Blockers 全部有对应任务
- 挑战会识别的 5 个 CRITICAL/HIGH 风险均已有缓解方案
- Phase 递进交付，每个 Phase 有独立可验证的评分提升

---

## Implementation-Readiness 结论

| 维度 | 状态 | 说明 |
|------|------|------|
| 需求挑战会 | ✅ 完成 | 8 项质疑、5 项导致计划修改、3 项保留原方案 |
| 设计收口 | ✅ 完成 | arch-design.md 产出 |
| Brownfield 诊断 | ✅ 完成 | 9 维度现状评估 |
| Story Slices | ✅ 完成 | 23 个任务、依赖图、验收标准 |
| 待确认项 | ⚠️ 1 项待确认 | Q5 K8s 集群信息（不阻塞 P1-P4） |
| 前置 Gate | ✅ | T0a-T0c 已加入，阻塞 P1 |

**结论：`handoff-ready`。** P1 可进入实现，从 T0a Migration 版本化开始。Q5 在 P5 开始前必须解决。

---

## Handoff

| 字段 | 值 |
|------|-----|
| 背景 | SaaS Readiness 交付计划已通过需求挑战会收口 |
| 输入依据 | PRD (18 tasks)、architect challenge、backend challenge、代码实际读取 |
| 结论 | 23 任务交付计划（含 5 个新增前置任务）、arch-design 已产出 |
| 风险 | R1-R8 已识别并有缓解措施 |
| 待确认项 | Q5 K8s 集群信息（不阻塞 P1-P4） |
| 下一跳角色 | backend-engineer（P0-P1 实现） |
| 当前阶段 | plan |
| 目标阶段 | handoff-ready |
| 就绪状态 | handoff-ready |
| readiness proof | 挑战会完成（8 质疑/5 修改）、arch-design 已产出、brownfield 诊断完成 |
| accepted_by | 待 backend-engineer 接受 |
| 阻塞项 | 无（Q5 不阻塞 P1-P4） |

---

*最后更新：2026-04-28*
*来源：PRD + architect challenge + backend challenge + 代码实际读取*
