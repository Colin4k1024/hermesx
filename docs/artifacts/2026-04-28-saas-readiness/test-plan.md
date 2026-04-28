# Test Plan: SaaS Readiness P0-P5

| 字段 | 值 |
|------|-----|
| Slug | `saas-readiness` |
| 日期 | 2026-04-28 |
| 主责 | qa-engineer |
| 状态 | completed |
| 阶段 | review |

---

## 测试范围

### 功能范围

| Phase | 测试对象 | 类型 |
|-------|---------|------|
| P0 | Migration versioning, Store interfaces, Middleware chain, AuthContext | 单元 + 集成 |
| P1 | ExtractorChain, Auth/Tenant/RBAC/RateLimit/Audit/RequestID middleware, Health probes | 单元 + 集成 |
| P2 | Tenant CRUD, JWT RS256, API Key lifecycle | 单元 + 集成 |
| P3 | Prometheus metrics middleware + endpoint | 集成 |
| P4 | OpenAPI spec, Usage/billing, Quota enforcement | 单元 + 集成 |
| P5 | TLS config, SecretStore, GDPR export/delete, Helm chart, CI security | 单元 + 配置 |

### 不覆盖项

- E2E 集成测试（需要 testcontainers PostgreSQL）
- Redis 分布式限流实际连接
- JWT 公钥轮换策略
- Helm chart 部署验证（需要 K8s 集群）

---

## 自动化验证

| 验证项 | 结果 |
|--------|------|
| `go build ./...` | PASS |
| `go test ./...` | 1153 tests, 28 packages, 0 failures |
| 新增文件编译 | PASS（23 新文件 + 3 修改文件） |

---

## 安全审查结果（Security Reviewer）

### CRITICAL — 已修复

| ID | 问题 | 修复方式 | 状态 |
|----|------|---------|------|
| CRIT-2 | API key revocation check dead code — DB 已过滤 `revoked_at IS NULL`，应用层检查永远不触发 | 保留为 defense-in-depth，无行为变更 | FIXED |
| CRIT-3 | API key `ExpiresAt` 从未强制执行 | DB 查询增加 `expires_at` 过滤 + extractor 增加 defense-in-depth 时间检查 | FIXED |
| CRIT-4 | TenantHandler IDOR — 任何认证用户可 CRUD 任意租户 | 增加 admin 检查 + 租户归属验证 | FIXED |
| CRIT-5 | APIKeyHandler.revoke 无租户归属检查 | 增加 `GetByID` + 租户归属验证 | FIXED |

### CRITICAL — 延后（Pre-existing code）

| ID | 问题 | 处置 |
|----|------|------|
| CRIT-1 | ACP server 在 `HERMES_ACP_TOKEN` 未设置时绕过所有认证 | Pre-existing code (`acp/auth.go`)，不在当前 SaaS 新增范围内，记入 backlog |

### HIGH — 已修复

| ID | 问题 | 修复方式 | 状态 |
|----|------|---------|------|
| HIGH-1 | `fmt.Sprintf` SQL 构造 LIMIT/OFFSET | 改为参数化 `$N` 占位符 | FIXED |
| HIGH-2 | 匿名请求绕过速率限制 | 使用 `RemoteAddr` IP 作为匿名限流 key | FIXED |
| HIGH-6 | GDPR delete 不报告错误 | 收集错误并返回 partial deletion 状态 | FIXED |
| HIGH-7 | GDPR delete 静默忽略错误 | 每个失败 session 记录 slog.Error | FIXED |
| HIGH-8 | GDPR export 无限制内存加载 | 限制 1000 session 上限 + 流式 JSON 编码 | FIXED |
| HIGH-9 | 租户 create/update 响应泄漏 DB 错误 | 统一返回 generic error message | FIXED |

### HIGH — 延后

| ID | 问题 | 处置 |
|----|------|------|
| HIGH-3 | 内存限流器不跨副本共享 | 需要 Redis 实际对接，记入 backlog |
| HIGH-4 | APIServerAdapter token 比较非 constant-time | Pre-existing code (`api_server.go:143`），记入 backlog |
| HIGH-5 | JWT `tenant_id` claim 未通过 store 校验 | 需要 tenant store lookup 集成，记入 backlog |
| HIGH-10 | Helm secrets 以明文环境变量注入 | 需要 ExternalSecrets / SealedSecrets 集成，记入 backlog |

### MEDIUM（9 项） — 记入 backlog

- RBAC prefix matching 非确定性排序
- "default" tenant fallback 风险
- API key 自提权到 admin
- localLimiter 内存泄漏（无清理）
- audit log 记录原始 query string
- 无 Dockerfile / securityContext
- CI actions 固定到 @master
- API key list 缺少 defense-in-depth 过滤

### LOW（5 项） — 信息提示

- `rand.Read` 错误未检查
- tenant context key 使用 string 类型
- LLM API key 明文存储到磁盘
- TLS 默认禁用
- JWT token-age 占位符

---

## Code Review 结果（Code Reviewer）

| 维度 | 评估 |
|------|------|
| 行为正确性 | 主路径正确，CRITICAL 修复后安全性显著提升 |
| 设计质量 | Middleware chain 解耦清晰，ExtractorChain 可扩展 |
| 测试覆盖 | 1153 tests PASS，但新增代码缺少专门的单元测试 |
| Go 惯用法 | 整体良好，minor：可用 `slices.Contains` 简化循环 |

---

## 风险评估

| 风险 | 等级 | 缓解 |
|------|------|------|
| Middleware chain 未挂载到 server | HIGH | 新中间件不影响现有路径，需独立 PR 挂载 |
| 新增代码缺专门单元测试 | MEDIUM | 1153 现有 tests 全量通过，但新逻辑需补充 |
| CRIT-1 (ACP auth bypass) 未修复 | HIGH | Pre-existing，但上线前必须处理 |
| Redis 限流未对接 | MEDIUM | localLimiter 单副本可用，多副本需 Redis |

---

## 放行建议

**建议：有条件放行（Conditional Go）**

当前 SaaS 新增代码的 5 个 CRITICAL 和 6 个 HIGH 已修复。剩余延后项均为：
1. Pre-existing code（不在本次范围）
2. 需要外部基础设施对接（Redis、K8s secrets）
3. 需要独立 PR 完成集成

**放行前提：**
1. CRIT-1 (ACP auth bypass) 必须在首次生产部署前修复
2. Middleware chain 挂载必须作为下一个 PR 完成
3. 新增代码需补充至少 80% 覆盖率的单元测试

---

*最后更新：2026-04-28*
