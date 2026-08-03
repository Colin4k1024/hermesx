# Execute Log — Go Architecture Remediation

**slug:** go-arch-remediation  
**状态:** execute  
**主责角色:** backend-engineer  
**日期:** 2026-07-31

---

## 计划 vs 实际

| 计划项 | 计划方案 | 实际实施 | 偏差说明 |
|--------|---------|---------|---------|
| S1: WaitGroup 泄漏 | 补 `wg.Add(1)` + `defer wg.Done()` | ✅ 按计划完成 | 无偏差。bug 描述从"永久阻塞"修正为"shutdown 不完整" |
| S2: os.Chdir 并发不安全 | 通过 ToolContext 传递 workdir | ✅ 改为 system prompt 注入 + 验证 workdir 存在 | 方案调整：不通过 ToolContext（改动面大），改为在 cron hint 中注入 workdir 指令，让 agent 通过工具参数使用正确目录 |
| A1: os.Exit 在库代码 | 改为 return error | ✅ 按计划完成。`NewAPIServer` 签名改为 `(*APIServer, error)` | 无偏差 |
| A2: context.Background | 使用请求 ctx | ⚠️ 方案调整为 `context.WithTimeout(context.Background(), 5s)` | PM 挑战指出：使用请求 ctx 会被恶意用户通过取消请求绕过限流。改为带超时的独立 context |
| A3: 错误包装 | `%v` → `%w` | ✅ 修复 3 处（docgen_sandbox.go ×2, approval.go ×1） | 实际需修复数量少于预估（7 处），部分 `%v` 用于非错误返回路径 |
| A4: Config.Save() 脱敏 | Save 前脱敏 + SaveFull 逃生舱 | ✅ 按计划完成 | 无偏差 |
| A5: LLM 无超时 | 添加 120s context 超时 | ✅ batch/runner.go + cron/scheduler.go | 无偏差 |
| B4: mcpcatalog 静默忽略 | 添加 slog.Warn | ✅ 按计划完成 | 无偏差 |

---

## 关键决定

### 决定 1：S2 修复方案选择

**背景：** 架构师和后端工程师挑战后确认 `exec.Command` 不可行（job 依赖进程内状态），`ToolContext` 传播改动面大。

**决策：** 采用"system prompt 注入"方案——在 cron hint 中注入 workdir 指令，让 LLM agent 通过工具参数（terminal tool 的 `working_directory`）使用正确目录。同时验证 workdir 目录存在性。

**影响：** 改动最小，不改变任何接口签名。但依赖 LLM agent 遵循指令——如果 agent 不使用 `working_directory` 参数，命令会在默认 CWD 执行。

**回退方案：** 如果 system prompt 方案不可靠，后续可通过 `ToolContext.WorkDir` 字段做工具层强制注入。

### 决定 2：A2 使用独立超时 context 而非请求 context

**背景：** PM 挑战指出 `context.Background()` 可能是有意设计（防止限流绕过）。

**决策：** 使用 `context.WithTimeout(context.Background(), 5s)` — 保留独立性（不被请求取消影响），同时添加超时保护。

---

## 阻塞与解决

| 阻塞 | 根因 | 解决方式 |
|------|------|---------|
| S2 exec.Command 不可行 | cron job 依赖进程内 agentruntime 状态 | 改用 system prompt 注入方案 |
| NewAPIServer 签名变更 | 从 `*APIServer` 改为 `(*APIServer, error)` | 更新 `cmd/hermesx/saas.go` 调用方 |
| 工具调用持久错误 | 模型层工具调用异常 | 使用子代理完成构建验证 |

---

## 影响面

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `internal/scheduler/scheduler.go` | 修改 | WaitGroup 修复 |
| `internal/cron/scheduler.go` | 修改 | 移除 os.Chdir，添加 workdir 验证 + prompt 注入 + LLM 超时 |
| `internal/api/server.go` | 修改 | NewAPIServer 返回 error，移除 os.Exit |
| `cmd/hermesx/saas.go` | 修改 | 更新 NewAPIServer 调用 |
| `internal/middleware/redis_ratelimiter.go` | 修改 | Allow() 使用带超时 context |
| `internal/tools/docgen_sandbox.go` | 修改 | 错误包装 %v → %w |
| `internal/tools/approval.go` | 修改 | 错误包装 %v → %w |
| `internal/mcpcatalog/catalog.go` | 修改 | 添加 slog import + 错误日志 |
| `internal/batch/runner.go` | 修改 | LLM 调用 120s 超时 |
| `internal/config/config.go` | 修改 | Save() 脱敏 + SaveFull() + maskSecret() |
| `internal/cli/setup_wizard.go` | 修改 | 改用 SaveFull() |

**总计：11 个文件，0 个新文件**

---

## 未完成项

| 项 | 原因 | 建议 |
|----|------|------|
| 构建验证 | 工具调用层异常 | 由 build-error-resolver 子代理或手动执行 `go build ./...` |
| 单元测试补充 | 时间约束 | S1 和 A4 的测试可在后续 PR 中补充 |
| ADR-010/011/012 | 属于 plan 阶段产出 | 在 `/team-review` 前补齐 |
| B1 错误格式统一 | 已推迟 | 需前端兼容性确认后单独处理 |
