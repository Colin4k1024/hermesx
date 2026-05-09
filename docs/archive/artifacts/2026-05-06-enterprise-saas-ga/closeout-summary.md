# Closeout Summary — Enterprise SaaS GA v1.2.0

## 收口对象

| 字段 | 值 |
|------|-----|
| 关联任务 | 2026-05-06-enterprise-saas-ga |
| 版本 | v1.2.0 |
| 范围 | Phase 1 (Security Hardening) + Phase 2 (OIDC + Pricing + Rate Limiting) |
| 收口角色 | tech-lead |
| 收口日期 | 2026-05-07 |
| 最终状态 | closed |

---

## 最终验收状态

**ACCEPTED** — Phase 1 + Phase 2 全部交付完成，QA 双阶段均为 GO，release-plan 已产出并放行。

### 验收证据

| 检查项 | 结果 |
|--------|------|
| go build ./... | clean |
| go vet ./... | clean |
| 全量测试 | 1469/1469 pass (33 packages) |
| Race detector | clean |
| Code Review | 0 CRITICAL, 0 HIGH |
| Security Review | 0 CRITICAL, 0 HIGH |
| Launch Acceptance P1 | GO (B1-B5 修复后) |
| Launch Acceptance P2 | GO (10/10 checks green) |

---

## 观察窗口结论

本次为 POC 环境全链路验证，发布方案已产出 (release-plan.md + deployment-context.md)，发布策略为 canary→50%→full 三阶段灰度。

**观察项设定** (待生产部署后执行):
- Redis Lua EVALSHA 延迟 p99 < 5ms
- PricingStore 缓存命中率 > 95%
- HPA 弹缩行为 stabilization 300s 充分性
- RLS SET LOCAL 高并发写入性能

**当前结论**: 代码层面验收完成，无事故、无回滚。生产部署后需按 release-plan.md 执行 24h 观察。

---

## 变更清单

### Phase 1 — Security Hardening

| 修复项 | 内容 | 关键文件 |
|--------|------|----------|
| B1 | RLS write protection (withTenantTx) | tenant_ctx.go, 8 store files |
| B2 | GDPR error response sanitization | gdpr.go |
| B3 | IDOR fix (X-Hermes-User-Id restricted) | memory_api.go |
| B4 | Session ownership check | memory_api.go |
| B5 | CORS wildcard removal | docker-compose.saas.yml, values.yaml |
| R1-R6 | 6 additional review findings | server.go, tests, docs |

### Phase 2 — Platform Features

| 交付项 | 内容 | 关键文件 |
|--------|------|----------|
| P2-S0 | AuthContext 扩展 (UserID, ACRLevel) | auth/context.go |
| P2-S1 | OIDCExtractor + ClaimMapper | auth/oidc.go |
| P2-S2 | Dynamic PricingStore + CostCalculator + Admin API | metering/pricing_store.go, cost_calculator.go, api/admin/pricing.go |
| P2-S3 | DualLayerLimiter (Redis Lua + Local fallback) | middleware/dual_limiter.go, redis_dual_limiter.go |
| Infra | store.ErrNotFound sentinel | store/store.go, store/pg/pricing.go |

---

## 残余风险处置

| 风险 | 等级 | 处置 | Owner |
|------|------|------|-------|
| OIDC 未 wire 到 auth chain | MEDIUM | 延后 — Phase 3 由运维配置激活 | devops-engineer |
| LocalDualLimiter 多副本倍增 | MEDIUM | 接受 — ADR-002 文档化, 仅 Redis 故障时触发 | backend-engineer |
| HasScope 对 empty scopes 放行 | MEDIUM | 接受 — P1 既有行为, 文档化为遗留兼容 | backend-engineer |
| GDPR 大批量删除超时 | MEDIUM | 延后 — 生产部署后按实际数据量评估 | backend-engineer |
| RLS SELECT policy 未启用 | LOW | 延后 — 评估读隔离需求后决定 | architect |
| Helm PDB selector 跨 release 重叠 | LOW | 接受 — 单 release 部署不触发 | devops-engineer |

---

## Backlog 回写

以下事项作为 Phase 3+ 候选项:

| # | 事项 | 优先级 | 触发条件 | Owner |
|---|------|--------|----------|-------|
| 1 | OIDC wiring 到 server.go auth chain | P1 | 运维提供 IdP 配置 | devops-engineer |
| 2 | 断路器 registry (ChatStream breaker 重构) | P2 | Phase 3 规划 | backend-engineer |
| 3 | CI/CD Pipeline (GitHub Actions) | P2 | Phase 3 规划 | devops-engineer |
| 4 | RLS SELECT policies 评估 | P3 | 读隔离需求确认 | architect |
| 5 | pgxmock 引入 (store 层 mock 测试) | P3 | 测试覆盖率提升 | backend-engineer |
| 6 | CORS 动态管理 (DB/config 加载) | P3 | 多域名需求出现 | backend-engineer |
| 7 | Admin UI for pricing rules | P3 | 产品需求确认 | frontend-engineer |
| 8 | 多副本 LocalDualLimiter 精确性优化 | P3 | 生产 Redis 频繁故障 | backend-engineer |

---

## 知识沉淀

### Lessons Learned

1. **store.ErrNotFound 是跨层解耦的关键** — handler 不应直接依赖 pgx/sql 驱动特定错误类型。引入 sentinel error 后，handler 层的错误分类逻辑变得清晰且可测试。此模式应推广到所有 store 操作。

2. **DualLayerLimiter 接口设计要预留 context** — 初版接口没有 context 参数，导致 Redis 调用无法响应请求取消。接口一旦发布就难以改，第一版就应该带 context。

3. **RLS set_config 参数语义易混淆** — `set_config(name, value, is_local)` 的第三参数 `true` 表示 transaction-local；`current_setting(name, missing_ok)` 的第二参数 `false` 表示未设置时报错。布尔参数语义完全不同，文档和 review 应明确标注。

4. **IDOR 修复需区分 ErrNoRows 和基础设施错误** — 简单的 `if err != nil` 会把 DB 超时伪装成 404。所有 ownership check 应遵循 `errors.Is(err, ...)` + fallback 500 模式。

5. **输入验证要在 handler 入口做，不要让无效数据进入 store 层** — 负数/NaN/Inf 价格和注入字符应在 HTTP handler 层用 regex + math 验证拦截，而非依赖 DB 约束。

6. **Redis Lua 脚本要考虑 Cluster hash tag** — 多 key Lua 脚本在 Redis Cluster 下需要所有 key 路由到同一 slot，`{tenantID}` hash tag 模式是标准做法。

---

## 任务关闭结论

- **Phase 1 状态**: CLOSED — 全部 blocking items 修复并通过 review
- **Phase 2 状态**: CLOSED — 全部 CRITICAL/HIGH 修复, 1469 测试通过, QA GO
- **整体任务状态**: CLOSED
- **重开条件**: 生产部署后观察窗口内出现 P0 事故或回滚
- **下一步**: Phase 3 规划 (OIDC wiring, 断路器 registry, CI/CD)

---

## 确认记录

| 角色 | 结论 | 日期 |
|------|------|------|
| QA (Phase 1) | GO | 2026-05-06 |
| QA (Phase 2) | GO | 2026-05-07 |
| DevOps | GO (release-plan 放行) | 2026-05-07 |
| Tech Lead | CLOSED | 2026-05-07 |
