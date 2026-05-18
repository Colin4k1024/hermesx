# Arch Design: v2.3.0 Security Integration Sprint

> **角色**: architect  
> **状态**: draft  
> **日期**: 2026-05-18  
> **关联 PRD**: prd.md  
> **关联 ADR**: ADR-006（WASM 推迟）

---

## 系统边界

三个安全子系统（`internal/safety`、`internal/egress`、`internal/secrets`）已实现并通过独立测试，但尚未接入 agent 主循环。本设计定义它们与 `agent.go` RunConversation 循环、`tools/registry.go` Dispatch 路径和流式响应路径的接入点。

---

## 关键架构决策

### D1 — 流式 Safety 语义：事后审计 + 会话终止

**决策**：流式场景下 safety 仅做完整响应后扫描，检测到违规时终止会话（设置 `interruptRequested`），**不做 chunk 级实时阻断**。

**理由**：
1. `CheckOutput` 接口接受 `string`，chunk 级阻断需要新增 streaming interceptor 抽象，引入缓冲区管理和回收已发送 chunk 的复杂度
2. `consumeStream` 在 delta 全部消费后才组装 `resp.Content`，此时调用 `CheckOutput` 自然且无侵入
3. v2.3.0 是安全接入 sprint，不是接口重设计 sprint；chunk 级阻断延至 v2.4.0

**接入点**：在 `RunConversation` 循环中，`streamingAPICall` 返回后、`assistantMsg` 写入 `messages` 切片之前，插入 `CheckOutput` 调用。若 `ActionBlock`，设 `result.Interrupted = true` + `result.FinalResponse = "[blocked by safety policy]"` + break。

**影响**：流式场景下用户可能已接收到部分 token。客户端需处理"流中断 + 违规信号"语义——通过现有 `StreamCallbacks.OnError` 通知。

---

### D2 — Transport 注入策略：Registry-Level Injection（ToolContext 注入）

**决策**：在 `executeSingleTool` 中统一注入 `SecureTransport`，而非逐工具改造 http.Client 初始化代码。

**架构接入点**：
1. 扩展 `ToolContext` 新增 `HTTPClient *http.Client` 字段
2. 在 `executeSingleTool` 构建 `toolCtx` 时，注入预配置了 `SecureTransport` 的 `http.Client`（tenant context 从 `a.tenantID` 取得）
3. 工具内部使用 `tctx.HTTPClient` 发起外部请求；未使用该字段的工具不产生网络出站，无需改动
4. `buildToolDefs` 无需传递 option——注入发生在 dispatch 时刻，不在定义时刻

**迁移策略**：新增 `ToolContext.HTTPClient`；对现有工具分批替换内部 `http.DefaultClient` 引用为 `tctx.HTTPClient`。未迁移工具过渡期继续使用 `http.DefaultClient`（egress audit-only），v2.4.0 前通过 lint 规则禁止直接引用 `http.DefaultClient`。

---

### D3 — Redirect Policy：Redirect-Depth 分级 + per-tool 覆盖

**决策**：`SecureTransport` 的 `CheckRedirect` 采用 redirect-depth 分级策略，同时支持 per-tool 覆盖。

**设计**：
- 全局默认：max 0 redirect（deny-all redirect）
- `ToolEntry` 新增可选字段 `MaxRedirects int`（默认 0）
- 注入 `http.Client` 时，根据当前 tool 的 `MaxRedirects` 设置 `Client.CheckRedirect`
- OAuth 工具声明 `MaxRedirects: 3`（覆盖 OAuth redirect flow 的 302/307 需求）
- Redirect 目标 host 仍需通过 `EgressPolicy.IsAllowed` 验证——redirect 不能绕过 allowlist

**理由**：比 per-tool boolean 更灵活，比无限制 redirect 更安全；redirect 目标仍经过 policy 检查，防止 SSRF 跳板攻击。

---

## 组件接入图

```
User Request
     │
     ▼
┌──────────────────────────────────────────────────────────┐
│  RunConversation (agent.go)                              │
│                                                          │
│  ① safety.CheckInput(messages)                           │
│     if Block → return immediately (audit: log only)      │
│                                                          │
│  ② LLM Call (stream or sync)                             │
│     └─ consumeStream → resp.Content assembled            │
│                                                          │
│  ③ safety.CheckOutput(resp.Content)                      │
│     if Block → interrupt + OnError callback              │
│                                                          │
│  ④ executeToolCalls → executeSingleTool                  │
│     ├─ Build ToolContext.HTTPClient                       │
│     │   └─ http.Client{Transport: SecureTransport(       │
│     │          egressPolicy, MaxRedirects: tool.Config)} │
│     ├─ SecretResolver injected via ToolContext            │
│     │   (tools use tctx.Secrets instead of os.Getenv)   │
│     └─ Dispatch → tool handler                           │
│                                                          │
│  ⑤ secrets.LeakScanner.Scan(toolResult)                  │
│     → redact before appending to message history         │
└──────────────────────────────────────────────────────────┘

Admin API (server.go)
  /admin/v1/safety/rules         → safety.AdminHandler
  /admin/v1/egress/allowlist     → egress.AdminHandler
  /admin/v1/secrets/patterns     → secrets.AdminHandler (已存在)
  /admin/v1/secrets/canary-tokens → secrets.CanaryAdminHandler
  [全部包裹 RequireScope("admin")]
```

---

## 风险与约束

| 风险 | 影响 | 缓解 |
|------|------|------|
| 流式场景用户已收到违规 token | 敏感内容短暂可见 | 客户端接收 interrupt 信号后清除本地缓冲；v2.4.0 评估 chunk-level buffered mode |
| 工具迁移 HTTPClient 不完整，存在绕过窗口 | 部分工具仍用 DefaultClient 出站 | 过渡期 Audit-Only egress 模式 + v2.4.0 前 lint 扫描 |
| `EgressPolicy.IsAllowed` 依赖 PG，高并发延迟抖动 | 工具执行 P99 增加 | AllowlistPolicy 前置 in-memory LRU cache (TTL 60s) |
| OAuth 工具 redirect 目标 host 未在 allowlist | OAuth flow 失败 | OAuth 工具注册时同步声明 redirect 目标域到 tenant allowlist |
| `CheckInput` 对长上下文扫描耗时 | RunConversation 延迟增加 | safety 扫描超时 500ms，超时降级为 log-only |
| #36+#37 中间提交存在 redirect 裸窗口 | 短暂 SSRF 绕过风险 | 强制要求 #36+#37 原子提交为同一 PR |

---

## 技术选型

| 组件 | 选型 | 理由 |
|------|------|------|
| Multi-pattern matching | Aho-Corasick (已有 internal/secrets/ahocorasick.go) | O(n) 扫描，适合高频 LLM 响应扫描 |
| Transport 注入 | ToolContext 字段注入 | 零侵入工具接口，过渡期兼容 DefaultClient |
| Safety policy 存储 | PostgreSQL (已有 migration) | 与现有 RLS + 租户隔离一致 |
| Egress rules 缓存 | in-memory LRU (过渡期)，v2.3.0 P3 可选 Redis | P2 低延迟够用，P3 再引入 Redis |

---

## 后续 ADR

若 v2.4.0 引入 chunk 级 streaming interceptor，需新增 ADR 描述：
- streaming safety 语义变更（audit-only → chunk-buffer-intercept）
- SafetyInterceptor 接口扩展（ProcessChunk vs ProcessOutput）
