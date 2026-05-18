# Launch Acceptance: HermesX Security Enhancement

> **Slug:** security-enhancement-ironclaw  
> **State:** accepted  
> **Date:** 2026-05-18  
> **Owner:** qa-engineer  
> **Status:** draft

---

## 验收概览

| 项目 | 内容 |
|------|------|
| 验收对象 | F3 Prompt Injection Defense + F2 Credential Isolation + F4 Network Allowlisting |
| 时间 | 2026-05-18 |
| 角色 | qa-engineer (评审), security-reviewer (安全评审), code-reviewer (代码评审) |
| 验收方式 | 自动化测试 + 代码评审 + 安全评审 + 构建验证 |

---

## 验收范围

### 业务验收

- 用户输入注入检测能力 (27 pattern categories)
- LLM 输出合规检查 (系统提示泄漏防护)
- Canary token 上下文泄漏检测
- Per-tenant 安全策略 (enforce/log_only/disabled)
- 50+ 凭证泄漏检测 pattern
- Aho-Corasick O(n) 高性能多模式匹配
- Secret redaction (输出脱敏)
- Per-tenant 网络出口白名单
- SSRF 防护 (私有IP/CGNAT 阻断)
- DNS rebinding 防护 (DialContext IP 验证)

### 技术验收

- Go 1.25 构建通过
- Single binary 部署模型保持
- 81 新增测试, 1765 全量测试通过
- 并发安全 (sync.RWMutex 保护所有共享状态)
- 性能预算 < 50ms (benchmark 验证)

### 不在范围内

- WASM Sandbox (F1, POC only)
- 工具层 SecureTransport 集成 (下一迭代)
- Admin API HTTP handler 集成到主 server (下一迭代)
- DB migrations 实际执行 (需 DBA review)
- Agent loop 集成 (接口已就绪, 集成测试阶段完成)

---

## 验收证据

### 测试结果

| 维度 | 结果 |
|------|------|
| 全项目构建 | `go build ./...` 通过 |
| 安全包测试 | 81 tests (safety 28 + secrets 34 + egress 19) 全部通过 |
| 全量测试 | 1765 tests / 41 packages 全部通过 |
| 回归 | 零回归 |
| Race 检测 | `go test -race` 无报告 |

### 关键 Artifact

| Artifact | 位置 |
|----------|------|
| PRD | `docs/artifacts/2026-05-18-security-enhancement-ironclaw/prd.md` |
| Arch Design | `docs/artifacts/2026-05-18-security-enhancement-ironclaw/arch-design.md` |
| Delivery Plan | `docs/artifacts/2026-05-18-security-enhancement-ironclaw/delivery-plan.md` |
| Execute Log | `docs/artifacts/2026-05-18-security-enhancement-ironclaw/execute-log.md` |
| Test Plan | `docs/artifacts/2026-05-18-security-enhancement-ironclaw/test-plan.md` |
| ADR-006 | `docs/adr/ADR-006-security-layer-placement-wasm-deferral.md` |

### Code Review Summary

| 严重级别 | 发现数 | 已修复 | 延后 |
|----------|--------|--------|------|
| CRITICAL | 1 (LeakMatch plaintext) | 1 | 0 |
| HIGH | 6 | 6 | 0 |
| MEDIUM | 3 (lint suggestions) | 0 | 3 (non-blocking) |

### Security Review Summary

| 严重级别 | 发现数 | 当前状态 |
|----------|--------|----------|
| CRITICAL | 4 (C1-C4) | C1 注释+下一迭代 auth 集成, C2 已修复, C3 下一迭代, C4 下一迭代 |
| HIGH | 6 (H1-H6) | H1 已修复, H2 下一迭代, H3 设计决策, H4 低风险, H5 下一迭代, H6 下一迭代 |
| MEDIUM | 7 (M1-M7) | M7 已修复, 其余下一迭代 |
| LOW | 4 (L1-L4) | 全部延后 |

---

## 风险判断

### 已满足项

- [x] 全项目构建通过, 零编译错误
- [x] 全量 1765 tests 通过, 零回归
- [x] 81 新增安全测试覆盖核心路径
- [x] 并发安全问题已修复 (InMemoryPolicyStore)
- [x] 明文泄漏 CRITICAL 问题已修复
- [x] 接口向后兼容 (无 breaking change)
- [x] Single binary 部署模型保持

### 可接受风险

| 风险 | 理由 | Owner |
|------|------|-------|
| C3: 工具层 HTTP client 未接入 SecureTransport | 现有 Docker sandbox + IsSafeURL 提供基础防护; SecureTransport 是增强层 | backend-engineer |
| C4: redirect 绕过 | 需 CheckRedirect, 当前 DialContext 已对直接请求有效 | backend-engineer |
| H5: Canary map 增长 | Token 对象 ~60 bytes, 短期不构成 OOM 风险 | backend-engineer |
| H6: ResolvedValues 接口 | 工具 handler 已在 Docker sandbox 中隔离运行 | architect |
| M1: 默认 log_only | 设计决策: 新租户 onboarding 需观察期 | tech-lead |

### 阻塞项

无硬阻塞项。

---

## 上线结论

### 结论：**允许上线 (有条件)**

**前提条件：**
1. DB migrations 经 DBA review 后执行 (safety_policies + egress_rules 表)
2. Agent loop 集成 interceptor 调用 (S1.6)
3. AdminHandler 接入主 server 时必须包裹 RequireScope("admin") 中间件

**观察重点：**
- InputGuard 误报率 (前 72h 监控 log_only 模式告警量)
- LeakScanner 检测率 (验证真实环境下 50+ patterns 命中情况)
- SecureTransport 连接延迟 (DNS 解析 + IP 验证增量)
- Canary map 内存增长速率

**确认记录：**

| 角色 | 结论 | 日期 |
|------|------|------|
| qa-engineer | 有条件放行 | 2026-05-18 |
| code-reviewer | 通过 (CRITICAL/HIGH 已修复) | 2026-05-18 |
| security-reviewer | 有条件通过 (C3/C4 下一迭代) | 2026-05-18 |
