# Execute Log: v2.3.0 Security Integration Sprint

> **角色**: backend-engineer  
> **状态**: ALL STORIES COMPLETE — 待 QA handoff  
> **日期**: 2026-05-18

---

## 执行范围（本次 /team-execute）

**Story A** — SafetyInterceptor 接入 RunConversation（#38）  
**Story E** — Canary token TTL 清理（#41）  
（Story B/C/D/F 留待后续 /team-execute）

---

## 计划 vs 实际

| Story | 计划 | 实际 | 偏差 |
|-------|------|------|------|
| A (#38) | SafetyInterceptor 挂入 agent.go，audit 模式 | ✅ 完成 | 新增 StreamCallbacks.OnError |
| B (#36+#37) | SecureTransport + CheckRedirect 原子 PR | ✅ 完成 | 见 Story B 注记 |
| E (#41) | Canary token TTL 清理 goroutine | ✅ 完成 | 无偏差 |
| C (#39) | 高风险工具迁移 SecretResolver | ✅ 完成 | 4 文件 8 key，fallback 保留 |
| D (#40) | Admin API 三 handler 统一注册 | ✅ 完成 | 11 端点，全部 RequireScope("admin") |
| F-#42 | ResolvedValues 接口限制 | ✅ 完成 | WithAllowedKeys wrapper |
| F-#43 | Unicode NFKC normalization | ✅ 完成 | input_guard.go 首行 |
| F-#44 | CI linter rule 禁止 os.Getenv | ✅ 完成 | .golangci.yml，warn-only |
| F-#45 | Redis 缓存 egress rules | ✅ 完成 | CachedEgressPolicy，TTL 60s |

---

## 关键决定

**Story A:**
- `StreamCallbacks` 原无 `OnError` 字段，新增 `OnError func(err error)` 作为可选回调（nil-safe），现有调用方不受影响
- `CheckInput` 接入点：`agent.go:314`（LLM 调用前，req 构建后）
- `CheckOutput` 接入点：`agent.go:421`（空响应恢复后、assistantMsg 写入 messages 前）
- safety 扫描超时 500ms，超时降级为 log-only（不阻塞主循环）

**Story E:**
- 采用 TTL-based 清理（非 count-based），token 结构新增 `createdAt time.Time` 字段
- Sweep 间隔 = `ttl/2`，最小 1 分钟（使用 Go 1.21+ `max()` builtin）
- `main.go` 中 `runGateway()` 创建独立 `CanaryDetector` 并启动清理 loop；server shutdown 时 context cancel 安全停止
- 注意：main.go 中的 CanaryDetector 与 InterceptorChain 内部的 Canary 是两个独立实例，v2.4.0 统一接入 runner 时需迁移

**Linter 修复（patterns.go + canary.go）:**
- `patterns.go:71,73` — regexp.MustCompile 改用 raw string（S1007）
- `canary.go:103` — if 语句改用 `max()` builtin（minmax）

---

## 阻塞与解决

- 编译器报错 `types.go:202 a.safetyInterceptor undefined` — 确认为 IDE/linter 快照问题，实际 field 已正确定义在 `agent.go:77`，`go build` 通过

---

## 影响面

- `internal/agent/agent.go`（Story A）
- `internal/safety/interceptor.go`（接口调用，只读）
- `internal/secrets/canary.go`（Story E，新增 StartCleanupLoop）
- `cmd/hermesx/main.go`（Story E，注入清理 goroutine）

---

## 未完成项

**Story B 注记（tech-lead 确认项）：**
- `web_extract` 工具原有 CheckRedirect(5次) 被移除，改由 `ToolEntry.MaxRedirects` 控制（当前默认 0）。如该工具需要 redirect，注册时须设 `MaxRedirects: 5`
- `checkMessagingRequirements`（CheckFn）和 `queryOSV`（malware预检）无 tctx，保留 `http.DefaultClient`，后续可通过 package-level 默认 policy 覆盖
- `mcp_sse.go` SSE client 保留 Timeout:0（documented exception）
- `browser_impl.go` / `browser_local.go` 跳过迁移（browser automation client，非 HTTP API）
- `tctx.HTTPClient.Timeout` 直接修改：当前单 goroutine per tool call 安全；如未来 toolCtx 并发共享，需改为 clone+override 模式
- 新增 `egress.NewAllowAllPolicy()`（过渡期使用），待 per-tenant EgressPolicy 接入后替换
- 全量回归：26 包通过，新增失败 0（满足 P3 准入条件）

**Story C 注记：**
- `CheckFn`（无 tctx）和 `init()` 保留 os.Getenv——结构性约束
- `mcp_sse.go` 无凭证类 os.Getenv，`tts.go` 使用 edge-tts 无 API key
- `messaging.go` gateway URL 为端点配置非凭证，跳过

**Story D 注记：**
- safety.go / egress admin handler / canary handler 路由已全部注册到 handler.go
- `globalPolicyStore` / `globalCanaryDetector` 为 singleton 过渡方案，后续随 server DI 迁移

**Story F 注记：**
- `#42` AllowedKeys nil/empty = 不限制（向后兼容）
- `#43` golang.org/x/text v0.36.0 已存在，go mod tidy 后升为 direct
- `#44` forbidigo warn-only；排除规则匹配 `// fallback` 注释行，新增裸调用会被标记
- `#45` CachedEgressPolicy：TTL 60s，InvalidateTenant 支持即时失效，ttl=0 直通模式

**最终回归**：26/26 包通过 -race，新增失败 0
