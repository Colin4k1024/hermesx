# Lessons Learned

---

## 2026-04-28 — SaaS Readiness: 安全审查驱动的批量修复

**场景：** P0-P5 一次性交付 23 个新文件后，并行 code-reviewer + security-reviewer 发现 29 个安全问题（5 CRITICAL + 10 HIGH + 9 MEDIUM + 5 LOW）。

**问题：**
1. 批量交付后的安全审查修复成本高 — 5 CRITICAL + 6 HIGH 需要跨 7 个文件的协调修改。
2. Pre-existing 安全问题（CRIT-1 ACP auth bypass）在新代码审查中被发现，但修复涉及 out-of-scope 代码。
3. 新增代码虽然编译和集成测试通过，但缺少专门的单元测试，导致安全修复缺乏回归保护。

**建议：**
1. 每个 Phase 交付后立即运行安全审查，不要等全量完成 — 修复成本随积累指数增长。
2. Pre-existing 安全问题应在 intake 阶段显式列入 backlog 并评估优先级，不要等到新代码审查时才发现。
3. 新增安全关键代码（auth、RBAC、tenant isolation）应在实现阶段同步补充单元测试，不要作为"后续补充"。
4. Store interface 扩展（如 `GetByID`）应在设计阶段预判，避免安全修复时才发现缺少必要的数据访问方法。

---
