# Closeout Summary — Eino 0.9 Upgrade Remaining

| 字段 | 值 |
|------|----|
| 任务 Slug | 2026-05-21-eino-09-upgrade-remaining |
| 主责角色 | tech-lead |
| 状态 | follow-up-required |
| 日期 | 2026-05-22 |

## 收口结论

本轮已完成昨晚计划清单中的 API handler 回归、错误路径矩阵、同 session 并发语义、checkpoint adapter 负向覆盖、workflow Eino 默认路径适配、API 契约文档和 changelog 更新。

唯一未闭环项是真实 PostgreSQL/MySQL API smoke；当前环境未启动数据库服务，因此没有伪造通过。发布前需要在 staging 或本地 compose 环境补跑。

## 关键决策

| 决策 | 证据 | 影响 |
|------|------|------|
| `/v1/agent/chat` 失败时不持久化当前 user turn | 失败路径测试显示半写 user message 会污染 resume/history | user + assistant 作为完整 turn 成功后一起落库；失败不更新 token |
| 同一 `tenant/session` handler 层串行 | 同 session 并发会同时写消息、checkpoint、token | 避免双写；长流式请求会让同 session 后续请求排队 |
| 暴露 `X-Hermes-Session-Id` 响应头 | 新 session 场景客户端需要稳定拿到 session ID | OpenAPI 和中英文 API reference 同步说明 |
| stale checkpoint 删除失败必须阻断 | stale/preempt 状态继续执行会恢复错误请求 | 对不支持 Delete 的 checkpoint store 返回明确错误 |
| workflow 默认 agent executor 切到 Eino | 文档与 closeout 已承诺 `agent_task` 默认通过 EinoAgentExecutor | `defaultAgentExecutor` 不再走旧 AIAgent，workflow 与 API 共用 TurnLoop 主链 |

## 已完成文件

| 文件 | 变更 |
|------|------|
| `internal/api/agent_chat.go` | 成功后持久化 user/assistant；流式错误不发送伪造 stop；按 tenant/session 串行；返回 session header |
| `internal/api/chat_handler.go` | 增加 session lock map |
| `internal/api/agent_chat_test.go` | 新增 resume、错误路径、SSE 错误、同 session 并发测试 |
| `internal/eino/turnloop_test.go` | 新增 malformed/stale checkpoint 负向测试 |
| `internal/store/checkpoints_test.go` | 新增 checkpoint adapter 错误传播测试 |
| `internal/workflow/eino_executor.go` / `internal/workflow/engine.go` | workflow agent_task 默认走 Eino TurnLoop |
| `internal/workflow/eino_executor_test.go` | 新增 workflow Eino executor 回归 |
| `internal/api/openapi.go` | 补 session header、`agentic_block`、502 契约说明 |
| `docs/api-reference.md` / `docs/api-reference.en.md` | 补 session header、`include_agentic_blocks`、SSE 与失败语义 |
| `docs/CHANGELOG.md` / `docs/CHANGELOG.en.md` | 记录 Eino 0.9 主链和 Agent Chat 修复 |

## 验证

```bash
/usr/local/go/bin/go test ./internal/api -run 'TestServeAgentHTTP_' -count=1
/usr/local/go/bin/go test ./internal/api ./internal/eino ./internal/store ./internal/store/pg ./internal/store/mysql -count=1
/usr/local/go/bin/go test ./internal/workflow -count=1
```

两条命令均通过。

## Follow-up

| 优先级 | 事项 | Owner |
|--------|------|-------|
| P1 | 在真实 PostgreSQL 与 MySQL 环境执行 interrupt/resume/preempt/stale cleanup API smoke | qa-engineer |
| P2 | 若生产流式会话很长，评估同 session 排队的超时/取消 UX | backend-engineer |
