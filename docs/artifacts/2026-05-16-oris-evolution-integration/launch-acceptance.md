# Launch Acceptance — Oris Evolution Integration

**版本：** v1.0  
**日期：** 2026-05-16  
**Slug：** oris-evolution-integration  
**主责角色：** qa-engineer  
**状态：** accepted（评审完成，结论：NO-GO）  

---

## 验收概览

| 项目 | 内容 |
|------|------|
| 评审对象 | hermes-agent-go Oris evolution integration |
| 评审时间 | 2026-05-16 |
| 参与角色 | qa-engineer, code-reviewer, security-reviewer |
| 验收方式 | 静态代码评审 + 单元测试 25 项 |
| 构建状态 | ✅ `go build ./...` 通过 |
| 测试状态 | ✅ 25/25 PASS |

---

## 验收范围

### In Scope
- `internal/evolution/` 全部新增代码
- `internal/agent/agent.go` evolution 路径改动
- `internal/api/chat_handler.go` + `agent_chat.go`
- `cmd/hermesx/main.go` + `saas.go` 初始化路径
- `internal/config/config.go` 配置扩展

### Out of Scope
- SaaS 多租户端对端联调
- MySQL 后端的集成验证
- 性能与吞吐基准

---

## 验收证据

| 证据类型 | 结果 |
|----------|------|
| 单元测试 | 25/25 PASS (`go test ./internal/evolution/... -v`) |
| 编译验证 | `go build ./...` 零错误 |
| code-reviewer 评审 | 3 HIGH + 4 MEDIUM + 2 LOW |
| security-reviewer 评审 | 2 CRITICAL + 3 HIGH + 3 MEDIUM + 1 LOW |

---

## 风险判断

### 已满足项
- 功能逻辑正确（25 个单元测试覆盖核心行为）
- 构建无编译错误
- evolution disabled 时完全零侵入（feature flag 保护）
- GeneStore 对应单测：open/close、save/query、confidence 过滤、outcome 记录

### 阻塞项（Abort Gate）

以下 4 项必须在合并到任何面向生产流量的分支之前修复：

| # | 标题 | 严重性 | 最小修复 |
|---|------|--------|----------|
| B1 | Gene insight 注入 system prompt 无清洗 | CRITICAL | PreTurnEnrich 调用 sanitizeInsight()，过滤控制字符、双向覆盖符，截断至 300 runes |
| B2 | 跨租户 gene 共享 — SaaS 模式租户隔离缺失 | CRITICAL | 最小修复：per-tenant SQLite 文件（`evolution-{tenantID}.db`）或 TaskClass 加 tenantID 前缀 |
| B3 | GeneStore.Close() 未注册到 shutdown 序列 | HIGH | saas.go shutdown block 添加 `gs.Close()`；main.go 添加 defer |
| B4 | Oris SDK OrderBy SQL injection（hermesx 侧防御） | HIGH | GeneStore.QueryTop 添加 safeOrderBy 白名单（3 行代码） |

### 可接受风险（下一 PR 修复）
- messages slice 防御性 copy（当前无 agent 复用场景）
- config struct 重复（编译安全，维护风险）
- validateInsight 长度上限（LLM 滥用路径，非外部直接攻击面）
- RecordOutcome 并发非原子（SQLite 写锁提供基本保护）
- SetEvolutionImprover atomic pointer（启动序列单线程，当前无竞态）

---

## 上线结论

**结论：❌ 不允许上线（NO-GO）**

**原因：** 2 项 CRITICAL 安全问题（prompt injection 路径未关闭、SaaS 多租户数据隔离未实现）未修复，合并到生产将直接暴露租户数据泄露风险。

**解除条件：**
1. 完成 B1–B4 四项阻塞修复
2. 修复后重跑 `go build ./...` + `go test ./internal/evolution/...` 全绿
3. security-reviewer 对 B1（sanitizeInsight）和 B2（tenant 隔离）出具复核结论

**观察重点（修复后上线初期）：**
- gene store 写入失败率（slog warn 出现频率）
- PostTurnRecord goroutine 超时率（45s timeout 触发）
- PreTurnEnrich 命中率（有策略注入 vs 无策略轮次比例）

---

## 确认记录

| 角色 | 结论 | 时间 |
|------|------|------|
| code-reviewer | WARNING — 3 HIGH 需修复 | 2026-05-16 |
| security-reviewer | NO-GO — 2 CRITICAL 阻塞合并 | 2026-05-16 |
| qa-engineer | NO-GO 确认 | 2026-05-16 |
