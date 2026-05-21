# Eino 0.9 升级剩余事项计划清单

## 版本目标

- 目标：完成 HermesX Eino 0.9 升级的收口工作，把当前“代码已接通、全仓测试已绿”的状态推进到“关键回归闭环、并发策略明确、对外契约同步、可发布说明完整”。
- 当前基线：
  - native provider model、agentic blocks、TurnLoop 主链、checkpoint store 已接入。
  - request-scoped TurnLoop 的中断恢复与 stale/preempt checkpoint 清理已有定向测试。
  - `go test ./...` 当前通过。
- 放行标准：
  - API 主链的真实 resume 场景有回归保护。
  - handler 失败路径与并发策略有明确验证。
  - 文档、OpenAPI、changelog 与当前行为一致。
  - 至少完成一轮真实 backend smoke 验证。

## 范围说明

- In Scope：
  - `/v1/agent/chat` 的真实恢复回归。
  - handler 错误路径补测。
  - 同 session 并发/抢占策略收口。
  - checkpoint store 负向测试与最小硬化。
  - API 契约文档同步。
  - 发布前 smoke、CHANGELOG、backend 支持说明。
- Out of Scope：
  - 新增产品功能。
  - 非 Eino 0.9 升级直接相关的重构。
  - 大范围 UI 或 WebUI 功能变更。

## 工作拆解

### P0. 补真实 API 级 resume 回归

- [ ] 在 `internal/api/agent_chat_test.go` 增加真实 handler 级 interrupt -> checkpoint -> resume -> complete 回归测试。
- [ ] 使用带 `Sessions`、`Messages`、`AgentCheckpoints` 的 fake store，走两次 `ServeAgentHTTP`，不要只走 `runAgent` seam。
- [ ] 验证恢复后 checkpoint 被消费或清空，assistant reply 不重复落库，历史中不会重复注入同一 user turn。
- 建议验证：`go test ./internal/api -run 'TestServeAgentHTTP_.*Resume|TestServeAgentHTTP_.*Checkpoint' -count=1`

### P0. 补 handler 错误路径矩阵

- [ ] 覆盖 invalid JSON、missing user message、foreign session、session create failure、runAgent failure。
- [ ] 补流式场景下的 error event 顺序与输出约束验证。
- [ ] 确认失败时不写 assistant message、不更新 token、不残留脏状态。
- 建议验证：`go test ./internal/api -count=1`

### P1. 收口同 session 并发与抢占策略

- [ ] 明确 Eino API 路径是否需要接入现有 session lock，或显式接受 preempt 模式并补测试。
- [ ] 对同一 `tenant/session` 的并发请求建立可验证语义：串行、抢占、或拒绝其一，不能保持隐式行为。
- [ ] 验证消息持久化、checkpoint ownership、token 统计不会在并发下出现双写或污染。
- 建议验证：
  - `go test ./internal/api -run 'TestServeAgentHTTP_.*Concurrent|TestServeAgentHTTP_.*Preempt' -count=1`
  - 如新增 runtime 级测试，再补 `go test ./internal/eino -count=1`

### P1. 补 checkpoint store 负向测试与最小硬化

- [ ] 覆盖 malformed checkpoint、checkpoint save failure、delete stale checkpoint failure、no result produced 等失败路径。
- [ ] 为 `internal/store/checkpoints_test.go`、`internal/store/pg/checkpoints_test.go`、`internal/store/mysql/checkpoints_test.go` 补异常分支测试，而不是只测 round-trip。
- [ ] 明确不支持 delete 的 store 在 stale checkpoint 清理时的预期行为。
- 建议验证：
  - `go test ./internal/store ./internal/store/pg ./internal/store/mysql -count=1`
  - `go test ./internal/eino -count=1`

### P2. 同步 API 契约与文档

- [ ] 对齐 `/v1/agent/chat` 的请求头、session ID 行为、`include_agentic_blocks`、SSE `agentic_block` 事件说明。
- [ ] 检查 `docs/api-reference.md`、`docs/api-reference.en.md`、`internal/api/openapi.go` 是否与当前返回格式一致。
- [ ] 如果决定暴露 `X-Hermes-Session-Id` 响应头，补实现与文档；如果不暴露，也要在文档中明确当前获取 session 的方式。
- 建议验证：文档 diff 自查 + 手工 curl/SSE smoke。

### P2. 做一轮发布前 backend smoke

- [ ] 在 PostgreSQL 与 MySQL 上各跑一轮 interrupt、resume、preempt、stale cleanup smoke。
- [ ] 确认 backend 支持矩阵：PG/MySQL 已支持 checkpoint，其他 backend 的行为要明确标注。
- [ ] 把 smoke 结果沉淀到 docs，避免发布时只靠会话结论。
- 建议验证：`go test ./...` + 一轮真实 API smoke 记录。

### P2. 补 CHANGELOG 与发布收口说明

- [ ] 在 `docs/CHANGELOG.md` 与 `docs/CHANGELOG.en.md` 记录 Eino 0.9 升级的核心变化。
- [ ] 说明 native provider path、TurnLoop resume、agentic blocks、checkpoint store backend 支持范围。
- [ ] 如果需要，补一份简短 release note 或 closeout summary。
- 建议验证：文档评审通过，且 release note 与代码行为一致。

## 执行顺序

1. 先完成 API 级 resume 回归。
2. 再补 handler 错误路径矩阵。
3. 然后收口并发/抢占策略。
4. 接着补 checkpoint 负向测试与最小硬化。
5. 最后统一同步文档、CHANGELOG、backend smoke 与发布说明。

## 风险与缓解

- 风险：handler 级 resume 暴露重复 user turn 或重复 assistant 持久化。
  - 缓解：优先补真实 API 测试，不再只依赖 runtime 单测。
- 风险：同 session 并发请求语义不清晰，导致 checkpoint 与 token 统计竞争。
  - 缓解：在进入发布前先明确串行/抢占策略，并以测试固定行为。
- 风险：文档仍保留旧的 session 或 SSE 说明，导致集成方理解偏差。
  - 缓解：将 OpenAPI、中文文档、英文文档一起同步，不接受只改一处。
- 风险：不同 backend 支持范围未讲清，生产上出现“测试环境能恢复，目标环境不能恢复”的落差。
  - 缓解：把 backend 支持矩阵和 smoke 结果一并写入 changelog 或发布说明。

## 节点检查

- 检查点 1：API 级 resume 回归测试通过。
- 检查点 2：handler 错误路径与并发策略测试通过。
- 检查点 3：checkpoint store 负向测试通过。
- 检查点 4：文档与 OpenAPI 同步完成。
- 检查点 5：`go test ./...` 通过，且 backend smoke 记录完成。
