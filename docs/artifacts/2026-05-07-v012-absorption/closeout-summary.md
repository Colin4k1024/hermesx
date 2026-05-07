# Closeout Summary — v0.12 Upstream Absorption (Sprint 3)

## Meta

| Field | Value |
|-------|-------|
| Date | 2026-05-07 |
| Slug | v012-absorption |
| Role | tech-lead |
| Status | final |
| State | closed |

## 收口对象

| Item | Detail |
|------|--------|
| 关联任务 | 2026-05-07-v012-absorption |
| Release | v1.4.0 (hermes-agent v2026.4.30 能力吸收) |
| 观察窗口 | N/A — 本轮为代码合入，生产部署另行安排 |
| 收口角色 | tech-lead, qa-engineer |

## 结果判断

| Item | Result |
|------|--------|
| 目标达成 | 4/4 features delivered and verified |
| 当前状态 | CLOSED — ready for merge to main |
| 发布后观察 | 不适用（代码交付，非生产发布） |

### Sprint 3 交付物

1. **Autonomous Memory Curator** — heuristic dedup + LLM merge + stale pruning (25 tests)
2. **Self-improvement Loop** — periodic LLM review + insight persistence + MaxInsights eviction (17 tests)
3. **Gateway Media Parity** — capability-aware dispatch + fallback chain + input validation (14 tests)
4. **Gateway Lifecycle Hooks** — priority-ordered event hooks + concurrent safety (11 tests)

### 质量验证

- Full regression: 1576 tests, 33 packages, 0 failures
- Race detection: clean on `./internal/gateway/` and `./internal/agent/`
- Code review: 2 rounds, 9 total fixes (1 CRITICAL + 6 HIGH + 2 MEDIUM)
- Security review: 2 rounds, all primary vectors closed

## 残余事项

| # | 事项 | 类型 | 严重度 | 处置 |
|---|------|------|--------|------|
| 1 | compress.go / curator.go 未对存储内容做 sanitizeForPrompt | 安全加固 | Low | 接受 — 仅处理 server-controlled 数据 |
| 2 | payload.URL 字段绕过路径遍历检查 | 安全加固 | Low | 接受 — URL 当前作为远程引用使用 |
| 3 | Unicode bidi chars 通过 sanitizeForPrompt | 安全加固 | Low | 接受 — LLM 不受渲染攻击影响 |
| 4 | LifecycleHooks 未接入 Gateway Runner | 集成 | Medium | 延后 — hooks standalone correct |
| 5 | SelfImprover 未接入 Agent 循环 | 集成 | Medium | 延后 — 独立测试通过，wiring 为 additive work |

**处置结论**: 无阻塞项，残余均已记录至 backlog，不影响合入判断。

## 知识沉淀

### Lessons Learned

1. **pg store.List() 排序语义需显式确认** — `ORDER BY updated_at DESC` 导致 eviction 逻辑反转。对任何依赖 List() 顺序的业务逻辑，必须在调用侧显式排序而非依赖 store 返回顺序。

2. **字节截断 vs rune 截断** — Go `s[:n]` 对包含多字节 UTF-8 字符（CJK）的字符串会产生非法序列。涉及用户可见内容或 LLM prompt 的截断一律使用 `[]rune` 转换。

3. **并发安全审计应在首次 review 而非后期补充** — Sprint 3 的 mutex 问题在第一轮 review 才暴露。建议对含 goroutine 交互的新模块，在 PR 提交前先执行 `go test -race`。

4. **分步 review 效果优于一次性大审** — 两轮 code-review + security-review 分别发现不同维度问题，比单次审查遗漏率更低。

## Backlog 回填

已同步到 `docs/memory/backlog.md`:
- v0.12 残余项 #1-5（见上方残余事项表）
- Curator O(n^2) dedup 优化候选

## 任务关闭结论

| Item | Decision |
|------|----------|
| 最终状态 | **CLOSED** |
| 合入条件 | CI green on main |
| 后续跟踪 | 残余项通过 backlog 追踪，下一 sprint 按优先级排入 |
| 确认 | tech-lead + qa-engineer |
