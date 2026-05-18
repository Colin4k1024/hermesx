# ADR-006: Safety Layer 位置决策 & WASM Sandbox 降级

## 决策信息

| 项目 | 内容 |
|------|------|
| 编号 | ADR-006 |
| 标题 | Safety Layer 放置在 Agent Loop 内 + WASM Sandbox 降级为 POC |
| 状态 | Accepted |
| 日期 | 2026-05-18 |
| Owner | tech-lead |
| 关联需求 | security-enhancement-ironclaw |

---

## 背景与约束

HermesX 计划从 IronClaw 借鉴安全增强能力，涉及两个关键架构决策：

1. **Safety Layer 放在哪里？** 选项包括 HTTP middleware stack 或 Agent runtime loop 内部。
2. **WASM Sandbox 是否本轮实施？** 原计划 P2 优先级实施 wazero-based 工具沙箱。

**约束条件：**
- 现有中间件栈已固定（chain.go），变更影响全局请求路径
- Prompt injection 发生在 agent loop 内部（用户消息 → LLM → 工具结果），不在 HTTP API 边界
- 50 个现有工具使用 net/http、os/exec、database driver，无法编译为 WASI
- 保持 single binary 部署模型
- P99 < 50ms 性能预算

---

## 备选方案

### 决策 1: Safety Layer 位置

| 选项 | 适用条件 | 优点 | 风险 |
|------|----------|------|------|
| A: HTTP Middleware | API 边界攻击 | 统一拦截点 | **无法检测 agent 内部注入**（工具结果回注、多轮对话注入） |
| B: Agent Loop 内部 | LLM 上下游消息流 | 覆盖真实攻击面 | 需要理解 agent 内部消息格式 |
| C: 两者都做 | 全面覆盖 | 最安全 | 复杂度高，维护两套规则 |

### 决策 2: WASM Sandbox

| 选项 | 适用条件 | 优点 | 风险 |
|------|----------|------|------|
| A: 本轮实施 | 新工具 only | ms 级隔离 | 50 个工具无法迁移，双 SDK 分裂 |
| B: 降级为 POC | 评估可行性 | 不阻塞核心交付 | 未来可能需要重新规划 |
| C: 不做 | — | 零开销 | 丢失长期架构方向 |

---

## 决策结果

### D1: Safety Layer → Agent Loop 内部（选项 B）

**原因：**
- HTTP middleware 看到的是 API client 请求体，不是 agent 内部的消息流
- Prompt injection 的真实攻击面在工具结果回注和多轮对话累积，必须在 LLM call 前后检查
- chain.go 不变，零影响现有请求路径

**影响范围：** `internal/agent/` 新增 safety interceptor，wrap LLM 调用

### D2: WASM Sandbox → 降级为 POC（选项 B）

**原因：**
- 50 个工具使用 net/http、os/exec、database/sql，无法编译为 WASI
- wazero 需要 host function binding（文件、网络、DB），等价于重建 tool SDK
- Docker sandbox + seccomp 已覆盖代码执行隔离需求
- 挑战会共识：ROI 不支持本轮实施

**影响范围：** F1 从交付计划移出，architect Week 5 做 1 个工具的 wazero POC + 可行性报告

**回退路径：** 如果未来新工具设计时直接基于 WASI 接口，可在 v2.3.0 重新评估

---

## 企业内控补充

| 维度 | 内容 |
|------|------|
| 应用等级 | T2 |
| 技术架构等级 | T2 |
| 关键组件偏离 | 无 — 纯 Go 实现，不引入新外部依赖 |
| 资产文档入口 | `docs/artifacts/2026-05-18-security-enhancement-ironclaw/` |

---

## 后续动作

| 动作 | 同步文档 | Owner | 完成条件 |
|------|----------|-------|----------|
| arch-design 中标注 Safety 位置 | arch-design.md | architect | 已完成 |
| delivery-plan 中 F1 降级为 POC | delivery-plan.md | tech-lead | 已完成 |
| wazero POC Week 5 | — | architect | 可行性报告产出 |
| 通知 qa-engineer F1 不进入本轮测试 | test-plan.md | tech-lead | 计划更新 |
