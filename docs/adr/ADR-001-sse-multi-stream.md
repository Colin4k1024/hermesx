# ADR-001: SSE Multi-Stream 并发方案

## 决策信息

| 字段 | 值 |
|------|-----|
| 编号 | ADR-001 |
| 状态 | Accepted |
| 日期 | 2026-07-06 |
| Owner | architect |
| 关联需求 | WorkBuddy 能力对齐 — 多任务并行管理 (Slice-1/2) |

## 背景与约束

### 当前现状

HermesX 当前 SSE 流式架构：
- 端点 `POST /v1/chat/completions`（`stream=true`）为每个请求创建独立 SSE 连接
- `agent_chat.go:206-275` 使用 `http.NewResponseController` + `Flusher` 模式
- 每个 SSE 连接经历完整 9 层中间件栈（Tracing → Metrics → ... → RateLimit → Handler）
- 事件类型：`data:` chunk（content delta）、`event: agentic_block`、`event: tool_call`、`event: tool_result`
- 后台心跳 goroutine 保活连接

### 新需求

多任务并行管理要求：
- 用户可同时运行 ≥ 3 个独立任务，每个任务产生独立的 SSE 流
- 用户切换任务时需要恢复（或保持）后台流
- 前端 `useSse` hook 当前仅支持单个 `AbortController`

### 约束

- 中间件栈中 RateLimit 按 tenant_id 维度限流
- 写超时 60s，需要长连接 keep-alive
- 心跳保活已有实现
- `writeMu` 互斥锁保护并发写入

### 非目标

- 不改中间件栈顺序
- 不替换 SSE 协议（不迁移到 WebSocket/gRPC streaming）

## 备选方案

### 方案 A: Per-Session 独立 SSE 流

每个任务（session）使用独立的 HTTP SSE 连接，事件格式不变，前端通过 `Map<sessionId, AbortController>` 管理。

| 维度 | 评估 |
|------|------|
| 连接数 | N 用户 × M 并行任务 = N×M 个长连接 |
| 实现复杂度 | 低 — 前端只需改造 useSse 为多实例，后端无需改动 |
| 事件隔离 | 天然隔离，无需 session_id 标记 |
| 断线恢复 | 每条流独立重连，逻辑简单 |
| 资源消耗 | 较高，每个连接占用 goroutine + 写缓冲 |

**适用条件：** 并行任务数有限（≤5），用户规模可控。

### 方案 B: 单连接多路复用

每个用户维持 1 个 SSE 连接，所有任务的事件在同一连接上传输，通过 `event` 类型 + JSON 字段中的 `session_id` 区分。

| 维度 | 评估 |
|------|------|
| 连接数 | N 用户 × 1 = N 个长连接 |
| 实现复杂度 | 中 — 后端需要事件路由层，前端需要按 session_id 分发 |
| 事件隔离 | 逻辑隔离，需要客户端解析路由 |
| 断线恢复 | 一个连接断开影响所有任务 |
| 资源消耗 | 最低 |

**适用条件：** 大规模用户，并行任务多，资源敏感。

### 方案 C: 混合模式 — 默认独立流 + 连接池上限

每个任务默认独立 SSE 连接（方案 A），但设置每用户最大并发 SSE 连接数上限（默认 5），超出时排队等待或拒绝。

| 维度 | 评估 |
|------|------|
| 连接数 | N 用户 × min(M, MAX_STREAMS_PER_USER) |
| 实现复杂度 | 低 — 方案 A + 连接数计数器 |
| 事件隔离 | 天然隔离 |
| 断线恢复 | 独立重连 |
| 资源消耗 | 可控 — 有上限保护 |

**适用条件：** 中等规模，需要平衡实现成本和资源安全。

## 决策结果

**采用方案 A（Per-Session 独立 SSE 流）**

### 理由

1. **当前实现零后端改动**：`agent_chat.go` 已经按 session 创建独立 SSE 流，session_id 在 URL 参数中传递
2. **事件已有独立通道**：每个连接有独立的 `writeMu`、独立的心跳、独立的 `Flusher`
3. **前端改造成本低**：`useSse` hook 只需从单实例改为 `Map<sessionId, AbortController>`
4. **HermesX 并行任务上限为 5**（由 `tenant.MaxSessions` 控制），N×M 增长有限
5. **复杂度最低**：无需引入事件路由层、无需改造中间件栈

### 连接上限保护

在 RateLimit 中间件中增加 per-user SSE 连接计数：
- 默认上限：`MAX_SSE_STREAMS_PER_USER = 5`（与 `MaxSessions` 对齐）
- 超出时返回 `429 Too Many Requests` + `Retry-After` header
- 连接关闭时递减计数器

### 影响范围

| 组件 | 变更 |
|------|------|
| `internal/api/agent_chat.go` | 无改动（已有独立流） |
| `internal/middleware/ratelimit.go` | 新增 SSE 连接数计数 |
| `webui/src/shared/hooks/useSse.ts` | 改造为 per-session Map 管理 |
| `webui/src/shared/stores/workspaceStore.ts` | 新增，管理多 SSE 控制器 |

### 失败或回退

若方案 A 在并发测试中出现资源问题（goroutine 泄漏、内存增长）：
1. 先尝试降低 `MAX_SSE_STREAMS_PER_USER`
2. 若仍不足，升级到方案 C（加入连接池排队）
3. 方案 B 作为长期优化方案

## 后续动作

| 动作 | Owner | 完成条件 |
|------|-------|---------|
| 实现 per-user SSE 连接计数中间件 | backend-engineer | Slice-2 |
| 改造 useSse hook 为 per-session 实例化 | frontend-engineer | Slice-1/2 |
| 编写 SSE 并发压力测试（3 用户 × 5 并行任务） | qa-engineer | Slice-2 完成后 |
