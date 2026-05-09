# Launch Acceptance: enterprise-saas-ga v1.2.0 Phase 1

| 字段 | 值 |
|------|-----|
| 验收对象 | Phase 1 — CRITICAL Security Hardening (7 slices) |
| 验收时间 | 2026-05-06 |
| 验收角色 | qa-engineer |
| 验收方式 | Code Review + Security Review + 自动化测试 |

---

## 验收范围

### 业务范围
- 多租户 RLS 写策略加固
- 审计日志不可篡改
- GDPR 右删除覆盖对象存储
- 容器编排高可用保障 (PDB/HPA)
- 用户会话隔离安全修复

### 技术范围
- PostgreSQL migrations 65-70
- Go API handlers (gdpr.go, agent_chat.go, server.go)
- Docker Compose + .env.example 凭证治理
- Helm chart templates (pdb.yaml, hpa.yaml, values.yaml)

### 不在范围内
- OIDC/SSO 集成 (Phase 2)
- 计费/用量计量 (Phase 2)
- 断路器 registry (Phase 2)
- CI/CD Pipeline (Phase 3)

---

## Go / No-Go 检查项

| # | 检查项 | 状态 | 说明 |
|---|--------|------|------|
| 1 | Go build 编译通过 | ✅ | `go build ./...` clean |
| 2 | Go vet 无警告 | ✅ | `go vet ./...` clean |
| 3 | 全量单元测试通过 | ✅ | 20/20 packages pass |
| 4 | Code review 无 CRITICAL | ❌ | 2 CRITICAL 发现 |
| 5 | Security review 无 CRITICAL | ✅ | 0 CRITICAL |
| 6 | Security review 无 HIGH | ❌ | 4 HIGH (IDOR, CORS, creds) |
| 7 | RLS 集成测试验证 | ❌ | Store 层缺 SET LOCAL |
| 8 | GDPR 207 路径测试 | ❌ | 无测试覆盖 |
| 9 | Helm template 渲染验证 | ❌ | 未执行 helm template |
| 10 | Session owner 测试 | ❌ | 无测试覆盖 |

---

## 风险判断

### 已满足项
- 编译和现有测试回归 — 无破坏
- Migration 幂等性设计 (DO $$ IF EXISTS guards)
- Session owner check 逻辑正确（agent_chat.go 路径）
- SQL injection 安全（allowlist + parameterized queries）
- SSRF 安全（tenantID regex 验证 + 静态 endpoint）
- pitr-drill.sh 完整可用

### 可接受风险
- Helm PDB selector 跨 release 重叠 (LOW, 单 release 部署不触发)
- Migration 70 重复列 IF NOT EXISTS (LOW, 无副作用)
- docker-compose.dev.yml 硬编码 (MEDIUM, 仅 dev 环境)
- NULL tenant_id audit rows 全租户可见 (LOW, 系统事件无租户上下文)

### 阻塞项
| 编号 | 描述 | 影响 |
|------|------|------|
| B1 | Store 层缺少 SET LOCAL → RLS 写入全断 | P0 生产事故 |
| B2 | GDPR handler 泄露内部错误信息 | 信息泄露 |
| B3 | memory_api.go IDOR (同租户跨用户) | 数据泄露 |
| B4 | session messages 无 ownership 检查 | 数据泄露 |
| B5 | CORS `:-*` 默认值 | CSRF 风险 |

---

## 上线结论

**结论: NO-GO — 不允许上线**

### 前提条件（解除阻塞后方可重新评审）

1. **B1**: 在 Store 层实现 `SET LOCAL app.current_tenant` 事务包装，并通过集成测试验证非 superuser 写入成功
2. **B2**: GDPR 响应体不含内部路径/表名/bucket 名
3. **B3+B4**: `memory_api.go` 移除 X-Hermes-User-Id header fallback 或限 admin scope，handleGetSessionMessages 添加 ownership 检查
4. **B5**: 移除 docker-compose.saas.yml 中 `SAAS_ALLOWED_ORIGINS` 的 `:-*` 默认值

### 观察重点（修复后上线时）
- RLS 写策略在真实 PG 下的性能开销
- GDPR 大批量 MinIO 对象删除耗时
- HPA 在负载波动下的弹缩行为

### 确认记录
- Code Reviewer: BLOCK (2 CRITICAL + 4 HIGH)
- Security Reviewer: BLOCK (4 HIGH + 6 MEDIUM)
- QA 结论: NO-GO

---

## 下一步动作

| 序号 | 动作 | Owner | 目标 |
|------|------|-------|------|
| 1 | 实现 Store 层 SET LOCAL 事务包装 | backend-engineer | 解除 B1 |
| 2 | GDPR error response 脱敏 | backend-engineer | 解除 B2 |
| 3 | memory_api.go IDOR 修复 | backend-engineer | 解除 B3+B4 |
| 4 | CORS 默认值移除 | backend-engineer | 解除 B5 |
| 5 | 补充单元测试 (207, owner, cleanup) | backend-engineer | 覆盖缺口 |
| 6 | 重新提交 /team-review | qa-engineer | 二次评审 |

---
---

# Launch Acceptance: enterprise-saas-ga v1.2.0 Phase 2

| 字段 | 值 |
|------|-----|
| 验收对象 | Phase 2 — OIDC + Dynamic Pricing + Dual-Layer Rate Limiting |
| 验收时间 | 2026-05-07 |
| 验收角色 | qa-engineer |
| 验收方式 | Code Review + Security Review + 自动化测试 (1469 tests) |

---

## 验收范围

### 业务范围
- OIDC ID token 验证 (JWKS 旋转, 可配置 claim 映射)
- 动态定价规则管理 (Admin CRUD API, 缓存失效)
- 原子双层限流 (租户 + 用户, Redis Lua 脚本)

### 技术范围
- `internal/auth/oidc.go` — OIDCExtractor + ClaimMapper
- `internal/metering/pricing_store.go` — 30s 缓存 + DB 回退
- `internal/metering/cost_calculator.go` — DB-first 计费
- `internal/api/admin/pricing.go` — 定价规则 CRUD + 输入验证
- `internal/middleware/dual_limiter.go` — DualLayerLimiter 接口 + Redis/Local 实现
- `internal/middleware/ratelimit.go` — 双层路径集成
- `internal/store/store.go` — ErrNotFound sentinel
- `internal/store/pg/pricing.go` — PG pricing CRUD

### 不在范围内
- OIDC wiring 到 server.go auth chain (需运维配置)
- 多副本 LocalDualLimiter 精确性 (已知限制)
- Admin UI
- CI/CD Pipeline (Phase 3)

---

## Go / No-Go 检查项

| # | 检查项 | 状态 | 说明 |
|---|--------|------|------|
| 1 | Go build 编译通过 | ✅ | `go build ./...` clean |
| 2 | Go vet 无警告 | ✅ | `go vet ./...` clean |
| 3 | 全量测试通过 | ✅ | 1469/1469 pass, 33 packages |
| 4 | Race detector 无竞态 | ✅ | `-race` clean |
| 5 | Code review 无 CRITICAL | ✅ | 0 CRITICAL (2 已修复) |
| 6 | Code review 无 HIGH | ✅ | 0 HIGH (2 已修复) |
| 7 | Security review 无 CRITICAL | ✅ | 0 CRITICAL |
| 8 | 输入验证完整 | ✅ | modelKey regex + price validation |
| 9 | 错误分类正确 | ✅ | store.ErrNotFound vs 500 |
| 10 | Context 正确传递 | ✅ | request context → Redis |

---

## 风险判断

### 已满足项
- 编译和全量测试回归 — 无破坏
- DualLayerLimiter 原子性 — Lua 脚本单 EVALSHA 保证
- Redis Cluster 兼容 — hash tag `{tenantID}` 确保同 slot
- 分布式限流器故障降级 — 自动 fallback 到 local + 日志告警
- 定价输入安全 — 非负有限数 + modelKey 正则白名单
- Delete 错误分类 — ErrNotFound vs 内部错误正确区分

### 可接受风险
| 风险 | 等级 | 处置 |
|------|------|------|
| OIDC nil,nil on verify fail | MEDIUM | 符合 ExtractorChain 语义; wiring 时复审 |
| Local fallback 倍增 | MEDIUM | 文档化于 ADR-002; 仅故障态触发 |
| HasScope 遗留兼容 | MEDIUM | P1 既有行为; admin scope 已单独保护 |
| OIDC 未填 Scopes | LOW | Phase 3; 当前角色检查覆盖需求 |

### 阻塞项
无。

---

## 上线结论

**结论: GO — 允许上线**

### 前提条件
- Phase 1 B1-B5 修复已在本次会话前完成且全量测试通过
- Phase 2 所有 CRITICAL/HIGH review 发现已修复并验证

### 观察重点（上线后）
- Redis Lua 脚本 EVALSHA 延迟 (p99 < 5ms baseline)
- PricingStore 缓存命中率 (预期 >95%)
- 限流误判率 (DualLimiter denied 但业务正常的比例)
- OIDC extractor wiring 后的 token 验证延迟

### 确认记录
- Code Reviewer: PASS (修复后 0 CRITICAL, 0 HIGH)
- Security Reviewer: PASS (0 CRITICAL, 0 HIGH, 3 MEDIUM accepted)
- QA 结论: GO
