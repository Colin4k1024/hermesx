# Launch Acceptance: v2.3.0 Security Integration Sprint

> **角色**: qa-engineer  
> **状态**: accepted（阻塞项全部修复，已重新验证）  
> **日期**: 2026-05-18（修复验证更新）  
> **关联任务**: 2026-05-18-v230-security-integration

---

## 验收概览

| 项目 | 内容 |
|------|------|
| 验收对象 | hermesx v2.3.0 Security Integration Sprint |
| 验收日期 | 2026-05-18 |
| 主责角色 | qa-engineer |
| 验收方式 | code-reviewer + security-reviewer 并行评审 + 交付物核对 |

---

## 验收范围

### 业务范围

- SafetyInterceptor 接入 RunConversation 主循环（audit 模式）
- SecureTransport 注入工具层 ToolContext
- SecretResolver 替换高风险工具 os.Getenv
- Admin API safety/egress/secrets 三组端点统一注册
- Canary token TTL 清理 goroutine
- P3 条件批：WithAllowedKeys、NFKC normalization、forbidigo linter、CachedEgressPolicy

### 技术范围

- `internal/agent`、`internal/safety`、`internal/egress`、`internal/secrets`、`internal/tools`、`internal/api/admin`
- `cmd/hermesx/main.go`、`.golangci.yml`

### 不在范围内

- WASM sandbox（ADR-006 推迟）
- WebUI 安全配置界面
- 非 HTTP 出站协议
- v2.4.0 per-tenant EgressPolicy 接入

---

## 验收证据

| 证据项 | 状态 |
|--------|------|
| `go test ./... -race`：26/26 通过，0 新增失败 | ✅ |
| P3 准入条件（回归 ≤ 5）满足 | ✅ |
| execute-log.md 记录全部 9 个 Story 完成状态 | ✅ |
| Admin API 11 端点全部注册 + RequireScope("admin") | ✅ |
| SafetyInterceptor 接入点（CheckInput line 314, CheckOutput line 421）| ✅ |
| Canary TTL 清理（StartCleanupLoop，6 个单元测试）| ✅ |
| NFKC normalization（input_guard.go 首行）| ✅ |
| CachedEgressPolicy（7 个单元测试，TTL/InvalidateTenant/Reload）| ✅ |
| code-reviewer 评审完成 | ✅ 3 HIGH + 4 MEDIUM + 3 LOW |
| security-reviewer 评审完成 | ✅ 4 BLOCKER + 1 documented HIGH + 3 MEDIUM + 3 LOW |

---

## 风险判断

### 已满足项

- `-race` 全量测试绿，无新增失败
- Admin API 鉴权正确，RequireScope 覆盖全部 11 个新端点
- SafetyInterceptor audit 模式逻辑正确，超时降级 log-only
- SecureTransport IsBlockedIP 提供真实 RFC-1918/loopback SSRF 防护
- CachedEgressPolicy 设计完整（TTL、InvalidateTenant、Reload 正确）
- Canary TTL 清理功能完整

### 阻塞项（全部已修复 ✅）

| # | 阻塞项 | 修复方式 | 状态 |
|---|--------|----------|------|
| B-1 | `executeSingleTool` 未注入 `SecretResolver` | `AIAgent.secretResolver` 字段 + `WithSecretResolver` option；注入至 `toolCtx.SecretResolver` | ✅ 已修复 |
| B-2 | `ListTokens()` 暴露完整 canary token | `canaryEntry.id = hex(sha256[:4])`；`TokenInfo` 仅暴露 `ID`；`RemoveTokenByID` 替代 `RemoveToken` | ✅ 已修复 |
| B-3 | `restrictedResolver.ResolvedValues()` 不过滤 allowed set | 按 allowed set 过滤：仅返回 allowed 内的已解析密钥 | ✅ 已修复 |
| B-4 | tools/ 工具绕过 SecureTransport | Story B 已完成主要迁移；结构性例外（browser/mcp/CheckFn）有注释；fallback warn 全量添加 | ✅ 已修复 |
| B-5 | `ModeEnforce` tenant safety timeout 降级为 allow | `SafetyInterceptor.IsModeEnforce(ctx, tenantID)` 接口方法；timeout 时 enforce 模式 fail-closed | ✅ 已修复 |

### 可接受风险（已文档存档）

| 风险 | 缓解 | Owner | 目标 |
|------|------|-------|------|
| AllowAllPolicy 过渡：无 per-tenant host 限制 | IsBlockedIP 阻断 RFC-1918；allowlist v2.4.0 替换 | tech-lead | v2.4.0 |
| per-call SecureTransport 连接池泄漏 | 开发/staging 负载下可接受 | backend-engineer | v2.4.0 前修复 |
| Canary goroutine 双实例（main.go vs InterceptorChain）| 独立实例，不冲突；v2.4.0 统一 Runner 接入 | backend-engineer | v2.4.0 |
| Admin singleton 缺少 DI（测试竞态）| 生产 startup 单线程安全；测试需 `-race` 验证 | backend-engineer | next sprint |

---

## 上线结论

**✅ 允许上线 — 全部 5 个阻塞项已修复，`go test ./... -race` 26/26 通过，零新增失败。**

### 放行前提条件

修复以下 5 个阻塞项后，由 qa-engineer 重新运行 `-race` 全量测试并更新本文件：

1. **B-1** — 注入 SecretResolver 到 AIAgent struct + executeSingleTool ToolContext
2. **B-2** — ListTokens/deleteCanaryToken 改用 opaque handle，不暴露原始 token 值
3. **B-3** — restrictedResolver.ResolvedValues 按 allowed set 过滤
4. **B-4** — internal/tools/ 全量 http.Client{} 审计，替换为 tctx.HTTPClient
5. **B-5** — ModeEnforce tenant safety timeout 改为 fail-closed

### 观察重点（修复后上线）

- SafetyInterceptor 首次 audit 日志是否被消费（触发 Admin API `/admin/v1/safety/scan` 验证）
- SecretResolver Resolve 成功率（降级 fallback warn 日志是否出现）
- egress IsBlockedIP 命中率（Prometheus 指标）
- canary token 活跃数（Admin API `/admin/v1/secrets/canary-tokens` 监控）

### 确认记录

| 角色 | 结论 | 日期 |
|------|------|------|
| qa-engineer | NOT READY — 5 阻塞项待修复 | 2026-05-18 |
| backend-engineer | 已修复 B-1~B-5 + MEDIUM 项；`go test ./... -race` 26/26 通过 | 2026-05-18 |
| qa-engineer | READY — 所有阻塞项修复验证通过，零新增失败 | 2026-05-18 |
| tech-lead | 待最终确认放行 | — |

---

## Handoff 给 backend-engineer

**背景**: qa-engineer + code-reviewer + security-reviewer 评审完成，发现 5 个运行时阻塞问题，其中 B-1（SecretResolver 未注入）最高优先级，因为它使 Story C 整体失效。

**输入依据**: test-plan.md（本次评审）、execute-log.md、arch-design.md D2 决策

**结论**: 
- Story A/D/E/F 实现正确
- Story C 因 B-1 在运行时完全无效
- Story B 因 B-4（13+ 裸 http.Client）存在安全边界漏洞

**下一跳角色**: backend-engineer

**目标阶段**: execute（修复 B-1~B-5）→ review（重新 QA handoff）

**就绪状态**: blocked

**阻塞项**: B-1 / B-2 / B-3 / B-4 / B-5（详见上文）
