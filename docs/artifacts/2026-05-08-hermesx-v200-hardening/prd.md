# PRD: HermesX v2.0.0 Post-Release Hardening

**状态**: intake → execute  
**日期**: 2026-05-08  
**Owner**: tech-lead  
**Slug**: hermesx-v200-hardening

---

## 背景

HermesX v2.0.0 完成品牌独立与企业加固，1581 tests 全绿。回顾发现 5 个遗留问题：
2 个功能已实现但未接入生产、2 个安全漏洞确认存在、1 个文档状态滞后。

## 目标与成功标准

| 目标 | 成功标准 |
|------|---------|
| LifecycleHooks 生产可用 | EmitConnect/Fire 在 Gateway Runner 中有真实调用点，测试覆盖 |
| SelfImprover 生产可用 | RecordTurn/Review 在 Agent 对话循环中有真实调用点，测试覆盖 |
| payload.URL 安全 | URL 字段经过 scheme allowlist + traversal 检查，与 Path 防护对齐 |
| prompt sanitization 一致 | compress.go / curator.go 使用 sanitizeForPrompt，与 self_improve.go 一致 |
| 文档状态准确 | project-context.md 反映 v2.0.0 现状 |
| 测试不退化 | ≥1581 tests 通过，gofmt/vet 全绿 |

## 范围

**In Scope**
- 4 项代码变更（wire-up × 2，安全修复 × 2）
- 1 项文档更新
- 每项代码变更补对应测试

**Out of Scope**
- store/pg pgxmock 引入（P3 独立 sprint）
- Admin UI / dashboard（P3/P4 产品需求）
- GHA digest-pin（P3 安全扫描周期）

## 风险与依赖

| 风险 | 缓解 |
|------|------|
| LifecycleHooks wiring 改变 Gateway 执行路径 | 现有集成测试全量回归 |
| SelfImprover wiring 增加 Agent 延迟 | Review 异步调用，不阻塞对话 |
| sanitizeForPrompt 跨包可见性 | 移出或复用，保持 DRY |

## 参与角色

- `tech-lead`：协调、收口、commit
- `backend-engineer`：代码实现（items 1-4）
