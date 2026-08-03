# Go Architecture Review — Second Pass

**日期:** 2026-07-31  
**审查范围:** 第一轮审查未覆盖的深层架构维度  
**审查维度:** 包耦合与上帝对象、资源管理、接口设计、并发安全与全局状态、优雅关闭完整性

---

## 审查结论

本轮发现 **18 项问题**：CRITICAL 3 项、HIGH 6 项、MEDIUM 9 项。与第一轮修复的 8 项无重叠。

| 严重度 | 数量 | 关键主题 |
|--------|------|---------|
| CRITICAL | 3 | MCP 关闭泄漏、重复进程注册表、环境注册表竞态 |
| HIGH | 6 | 跨租户全局状态、AIAgent 上帝对象、Store 胖接口、HTTP 客户端碎片化、上下文传播缺失、终端进程无清理 |
| MEDIUM | 9 | 后台 goroutine 无 context、委托 goroutine 泄漏、Toolsets 竞态、可变全局 Map、响应体关闭脆弱、Config 全局耦合 |

---

## CRITICAL — 必须修复

### C1: MCP 关闭泄漏 — `ShutdownAllMCP()` 从未被调用

**文件:** `internal/tools/mcp.go:1018`, `cmd/hermesx/saas.go`  
**问题:** `ShutdownAllMCP()` 已实现但从未在 shutdown 路径中调用。`RegisterMCPToolsWithSampling()` 在 line 905 启动了 `startNotificationWatcher` 和 `startHealthMonitor` 两个后台 goroutine，使用 `context.Background()` 作为生命周期。关闭时这些 goroutine 会泄漏。  
**影响:** 优雅关闭时 MCP 连接不释放，可能导致连接池耗尽或 goroutine 泄漏。  
**修复:** 在 `cmd/hermesx/saas.go` 的 shutdown 序列中添加 `defer tools.ShutdownAllMCP()`。

### C2: 双重进程注册表 — 旧实现无清理

**文件:** `internal/tools/terminal.go:29-375`, `internal/tools/process_registry.go`  
**问题:** 存在两套并行的进程管理实现：
- **旧版:** `terminal.go` 中的 `processRegistry`（package-level map + `processMu`），`startBackground()` 启动的后台进程没有 `Cleanup()` 方法。
- **新版:** `process_registry.go` 中的 `ProcessRegistry` struct，有完整的 `Cleanup()` 和 `Kill()` 方法。

旧版 `startBackground()` 启动的 goroutine（line 368）无法被取消或清理。  
**影响:** 进程泄漏——旧版启动的后台进程在关闭时不会被终止。  
**修复:** 将 `terminal.go` 的 `startBackground()` 迁移到使用 `ProcessRegistry`，或在 shutdown 路径中添加旧版清理逻辑。

### C3: 环境注册表无并发保护

**文件:** `internal/tools/environments/base.go:21-42`  
**问题:** 全局 `registry` map 没有 mutex 保护。`RegisterEnvironment()` 写入、`GetEnvironment()` 和 `ListEnvironments()` 读取均无同步。当前仅在 `init()` 中注册所以安全，但任何运行时注册都会导致 data race。  
**影响:** 如果未来添加运行时环境注册，会触发 `-race` 检测到的 data race。  
**修复:** 添加 `sync.RWMutex` 保护，或明确文档约束"仅限 init() 注册"。

---

## HIGH — 强烈建议修复

### H1: 跨租户全局审批状态

**文件:** `internal/tools/approval.go:184,355`  
**问题:** `globalApprovalStore` 和 `globalGatewayQueue` 是 package-level 单例，在多租户 SaaS 中被所有租户共享。`ApprovalStore.permanentApproved` 存储的永久批准模式没有租户隔离。  
**影响:** 租户 A 的永久批准可能影响租户 B 的命令审批决策。  
**修复:** 将 `ApprovalStore` 改为 per-tenant 实例，或在 key 中包含 tenantID。

### H2: AIAgent 上帝对象

**文件:** `internal/agent/agent.go`（1057 行，30+ 字段，26 个方法）  
**问题:** `AIAgent` struct 承担了过多职责：LLM 调用、工具执行、内存管理、安全拦截、流式传输、上下文压缩、预算管理、进化/自我改进、多模态路由、回退模型链。  
**影响:** 难以测试、难以理解、修改一个功能可能影响其他功能。  
**修复:** 按职责拆分为 `ConversationManager`、`ToolExecutor`、`StreamHandler`、`ContextManager` 等子组件，通过组合而非继承。

### H3: Store 接口过胖

**文件:** `internal/store/store.go`（15 个子接口，30+ 方法）  
**问题:** `Store` 接口聚合了 15 个子接口（SessionStore、MessageStore、UserStore、TenantStore、AuditLogStore、APIKeyStore、MemoryStore、UserProfileStore、CronJobStore、RoleStore、PricingRuleStore、ExecutionReceiptStore、FileEntryStore、WorkflowStore、AgentProfileStore），违反接口隔离原则。  
**影响:** 实现者必须实现所有子接口，即使只用到部分；测试 mock 成本高。  
**修复:** 按使用场景拆分为更小的 Store 组合（如 `SessionManager`、`TenantManager`），通过组合注入。

### H4: HTTP 客户端碎片化

**文件:** `internal/llm/transports/*.go`, `internal/tools/*.go`  
**问题:** 每个 LLM transport 创建独立的 `http.Client{Timeout: 300s}`，工具层也各自创建 `http.Client`。连接池无法共享，且缺少统一的 Transport 层配置（如 OTel tracing、egress policy）。  
**影响:** 连接资源浪费，无法统一管理超时、重试和可观测性。  
**修复:** 在 `llm` 包中提供共享的 `http.Client` 工厂，通过依赖注入传入 transport 和 tool 层。

### H5: MCP Handler 使用 `context.Background()`

**文件:** `internal/tools/mcp.go:278,300`  
**问题:** `handleSamplingOnStdio` 和 `handleSamplingOnSSE` 中的 `handler.HandleRequest(context.Background(), ...)` 不传播请求上下文，导致无法取消或超时控制。  
**影响:** MCP sampling 请求无法被上层取消，可能导致长时间阻塞。  
**修复:** 使用 `sseT.ctx`（SSE 场景）或创建带超时的 context。

### H6: 终端旧版进程无清理路径

**文件:** `internal/tools/terminal.go:335-385`  
**问题:** `startBackground()` 启动的后台进程存储在 `processRegistry` map 中，但没有清理方法。shutdown 时这些进程会成为孤儿进程。  
**影响:** 关闭时遗留的 shell 进程可能继续运行。  
**修复:** 添加 `CleanupAll()` 方法并在 shutdown 路径中调用。

---

## MEDIUM — 建议改进

### M1: 后台 goroutine 无 context 取消

**文件:** `internal/tools/terminal.go:368`  
**问题:** `startBackground()` 中的 `go func() { cmd.Wait() ... }()` 不接受 context，无法被外部取消。  
**修复:** 传入 context 并在 `select` 中监听 `ctx.Done()`。

### M2: 委托进度 goroutine 可能泄漏

**文件:** `internal/tools/delegate.go:147-151`  
**问题:** `go func() { for msg := range progressCh { ... } }()` 在函数返回后可能仍在运行，因为 `progressCh` 是 buffered channel，如果所有任务完成但 channel 未关闭，goroutine 会阻塞。  
**影响:** 每次委托调用可能泄漏一个 goroutine。  
**修复:** 确保 `close(progressCh)` 在 `wg.Wait()` 之后调用。

### M3: Toolsets 全局 Map 潜在竞态

**文件:** `internal/toolsets/toolsets.go:88,314`  
**问题:** `var Toolsets = map[string]*ToolsetDef{...}` 是导出的可变 map，虽然有 `mu sync.RWMutex`，但外部调用者可能不持有锁就直接读取。  
**修复:** 将 `Toolsets` 改为不可导出，通过带锁的 getter 方法访问。

### M4: KnownModels 和 ModelAliases 可变全局 Map

**文件:** `internal/llm/models.go:12`, `internal/llm/model_aliases.go:7`  
**问题:** `var KnownModels = map[string]ModelMeta{...}` 和 `var ModelAliases = map[string]string{...}` 是导出的可变 map，无并发保护。如果运行时修改（如 model catalog hot-reload）会触发 data race。  
**修复:** 改为不可导出，通过带锁的 getter/setter 访问；或在 hot-reload 路径中使用 `sync.Map`。

### M5: Firecrawl 轮询响应体关闭脆弱

**文件:** `internal/tools/web.go:420`  
**问题:** `pollResp.Body.Close()` 非 deferred，如果后续代码在 Close 前 panic 或 return，响应体会泄漏。  
**修复:** 改为 `defer pollResp.Body.Close()`。

### M6: Config.Load() 全局耦合

**文件:** `internal/tools/delegate.go:131`  
**问题:** `cfg := config.Load()` 直接调用全局单例，绕过了依赖注入。这使得测试难以 mock 配置。  
**修复:** 通过 `ToolContext` 或构造函数注入配置。

### M7: ResultStorage 全局状态

**文件:** `internal/tools/result_storage.go:32`  
**问题:** `var` 块中的全局状态（ResultStore 等）没有 tenant 隔离。  
**修复:** 改为 per-agent 或 per-session 实例。

### M8: 全局文件状态协调器无租户隔离

**文件:** `internal/tools/file_state.go:24`  
**问题:** `var globalFileState = &FileStateCoordinator{...}` 是全局单例，所有租户共享。文件冲突检测基于 sessionID 而非 tenantID。  
**影响:** 不同租户的文件修改可能被误报为冲突。  
**修复:** 按 tenantID 分片或在 key 中包含 tenantID。

### M9: shutdown 序列不完整

**文件:** `cmd/hermesx/saas.go:475-495`  
**问题:** shutdown 序列关闭了 cron scheduler、evolution store 和 data store，但未关闭：
- MCP 连接（C1）
- 终端后台进程（C2/H6）
- 全局文件状态协调器  
**修复:** 补全 shutdown 序列，按依赖顺序关闭所有资源。

---

## 优先级建议

| 优先级 | 项目 | 理由 |
|--------|------|------|
| P0 | C1 MCP 关闭泄漏 | 生产环境资源泄漏 |
| P0 | C2 双重进程注册表 | 进程泄漏 + 代码重复 |
| P0 | H1 跨租户全局审批 | 多租户安全隔离 |
| P1 | H6 + M9 shutdown 补全 | 与 C1/C2 联动 |
| P1 | H5 MCP context 传播 | 超时控制缺失 |
| P1 | C3 环境注册表竞态 | 防御性修复 |
| P2 | H2 AIAgent 拆分 | 可维护性（大重构） |
| P2 | H3 Store 接口拆分 | 可测试性（大重构） |
| P2 | H4 HTTP 客户端统一 | 资源效率 |
| P3 | M1-M9 | 代码质量改进 |

---

## 与第一轮修复的关系

第一轮修复了 8 项（S1/S2/A1-A5/B4），覆盖了并发 bug、错误处理、配置安全和超时。本轮发现的 18 项与第一轮无重叠，聚焦在更深层的架构结构问题：资源生命周期管理、包边界、接口设计和全局状态治理。
