# PRD — Code Review Fixes (2026-06-04)

**slug:** code-review-fixes  
**状态:** intake → plan  
**主责角色:** tech-lead  
**日期:** 2026-06-04

---

## 背景

代码审查发现架构批量修复（commit a6833f4）遗留 3 个 MEDIUM + 1 个 LOW 问题，需在本迭代内修复。

## 目标与成功标准

- 消除常量语义复用带来的维护歧义
- 统一错误包装约定，保证上层 `errors.Is(err, store.ErrNotFound)` 可正确工作
- 修复 GDPR AlertEvents 导出被静默截断 50 条的隐性合规漏洞（pg 层 guard `>100→50`）
- 收敛 `StartCleanupLoop` 可见性，避免外部误用

**成功标准:** 4 个问题全部修复，现有测试通过，无新增 CRITICAL/HIGH 问题。

---

## 用户故事与验收标准

| # | 问题 | 验收标准 |
|---|------|---------|
| F1 | `gdprExportMaxSessions` 复用于 AlertEvents | 新增独立常量 `gdprExportMaxAlertEvents`，`gdpr.go:148` 使用新常量 |
| F2 | `pgAlertRuleStore.Get/Update/Delete` 暴露 `pgx.ErrNoRows` | 3 处改为 `store.ErrNotFound`，上层 `errors.Is` 可正常工作 |
| F3 | GDPR AlertEvents 最多 50 条（pg guard 将 >100 截为 50）| `ListByTenant` 支持 limit=0 表示无限制；GDPR 导出传入 0；pg 层移除上限 guard |
| F4 | `StartCleanupLoop` 是 exported 方法 | 改为 `startCleanupLoop`，Run 内部调用同步更新，测试调整 |

---

## 范围

**In Scope:**
- `internal/api/gdpr.go`
- `internal/store/pg/pg_alert_store.go`
- `internal/metering/alerts.go`
- `internal/metering/alerts_test.go`（F4 接口调整）

**Out of Scope:**
- 其他 store 实现（mysql/sqlite）的 `ListByTenant` 行为变更
- AlertEventStore 接口签名变更（limit 语义调整在 pg 层内部完成）

---

## 风险与依赖

| 风险 | 影响 | 缓解 |
|------|------|------|
| F3 全量导出大租户 OOM | GDPR 导出内存压增 | pg 层单次 SQL `LIMIT ALL`，结果集受实际数据量控制；后续可加流式分页 |
| F4 rename 影响测试 | 编译失败 | rename 后同步更新所有引用 |
