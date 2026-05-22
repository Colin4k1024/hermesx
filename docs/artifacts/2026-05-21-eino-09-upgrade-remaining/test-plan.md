# Test Plan — Eino 0.9 Upgrade Remaining Closeout

| 字段 | 值 |
|------|----|
| 任务 Slug | 2026-05-21-eino-09-upgrade-remaining |
| 范围 | `/v1/agent/chat` resume、错误路径、同 session 并发、checkpoint adapter、API 契约 |
| 日期 | 2026-05-22 |
| 主责角色 | qa-engineer |

## 测试矩阵

| ID | 场景 | 覆盖文件 | 期望 |
|----|------|----------|------|
| API-RESUME-01 | handler 级 interrupt -> checkpoint -> resume -> complete | `internal/api/agent_chat_test.go` | interrupted run 不落 user message；resume 后只落 1 个 user + 1 个 assistant；checkpoint 被消费 |
| API-ERR-01 | invalid JSON / missing user message / foreign session / session create failure / runAgent failure | `internal/api/agent_chat_test.go` | 返回对应 4xx/5xx；不写消息；不更新 token |
| API-SSE-01 | streaming runAgent failure | `internal/api/agent_chat_test.go` | 输出 `event: error` 后 `data: [DONE]`；不发送伪造 stop；不写消息与 token |
| API-CONC-01 | 同一 tenant/session 并发请求 | `internal/api/agent_chat_test.go` | handler 串行执行；第二个请求看到第一个完整 turn；无双写 |
| TL-CKPT-01 | malformed checkpoint | `internal/eino/turnloop_test.go` | 启动前删除 malformed checkpoint，避免旧格式阻断会话 |
| TL-CKPT-02 | stale checkpoint 需要删除但 store 不支持 Delete | `internal/eino/turnloop_test.go` | 返回明确错误，不继续执行 |
| STORE-CKPT-01 | checkpoint adapter backend error propagation | `internal/store/checkpoints_test.go` | Get/Set/Delete 后端错误原样向上返回 |
| WF-EINO-01 | workflow `agent_task` 使用 Eino TurnLoop 主链 | `internal/workflow/eino_executor_test.go` | EinoAgentExecutor 走非流式 TurnLoop，注入 workflow payload 并返回 response |
| DOC-API-01 | API contract sync | `docs/api-reference*.md`, `internal/api/openapi.go` | session header、`include_agentic_blocks`、SSE `agentic_block`、失败语义一致 |

## 执行结果

| 命令 | 结果 | 说明 |
|------|------|------|
| `/usr/local/go/bin/go test ./internal/api -run 'TestServeAgentHTTP_' -count=1` | PASS | handler 级新增回归 |
| `/usr/local/go/bin/go test ./internal/api ./internal/eino ./internal/store ./internal/store/pg ./internal/store/mysql -count=1` | PASS | API、TurnLoop、checkpoint adapter、PG/MySQL SQL shape |
| `/usr/local/go/bin/go test ./internal/workflow -count=1` | PASS | workflow Eino executor 回归 |
| `/usr/local/go/bin/go test ./... -count=1` | PASS | 全仓回归 |

## 后端支持矩阵

| Store backend | Agent checkpoint support | 当前验证 |
|---------------|--------------------------|----------|
| PostgreSQL | 支持 `AgentCheckpoints()`，持久化 checkpoint payload | SQL shape + interface compliance 测试通过 |
| MySQL | 支持 `AgentCheckpoints()`，持久化 checkpoint payload | SQL shape + interface compliance 测试通过 |
| 其他 store | 未实现 `AgentCheckpoints()` 时不启用 TurnLoop checkpoint resume | `storeCheckpointAdapter` 返回 nil |

## 未完成验证

真实 PostgreSQL/MySQL API smoke 尚未在本地环境执行。发布前仍需在带数据库服务的环境跑 interrupt、resume、preempt、stale cleanup 四类 smoke，并把结果回填到本 artifact。
