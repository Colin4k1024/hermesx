# Delivery Plan: HermesX Security Enhancement

> **Slug:** security-enhancement-ironclaw  
> **State:** plan  
> **Date:** 2026-05-18  
> **Owner:** tech-lead  
> **Status:** draft

---

## 版本目标

| 项目 | 说明 |
|------|------|
| 里程碑 | HermesX v2.2.0 — Security Depth Enhancement |
| 范围 | F3 + F2 + F4（3 个 feature），F1 WASM 降级为 POC 调研 |
| 放行标准 | 零回归 + P99 < 50ms + 安全测试通过 + log_only 模式生产验证 |

---

## 需求挑战会结论

### Requirement Challenge Session Log

| # | 假设 | 质疑人 | 质疑内容 | 结论 |
|---|------|--------|----------|------|
| 1 | Prompt injection 用规则匹配可达 95% 检测率 | architect | 静态 regex 误报率高，bypass 率高于预期；arms race | **接受风险**：先上规则引擎 + log_only，收集 baseline 数据后再评估是否需要 LLM-as-judge |
| 2 | Credential isolation 需要 opaque handle | backend-engineer | 工具最终需要明文值做 HTTP auth，opaque handle 增加复杂度无实际安全收益 | **修改方案**：改为 just-in-time resolve + output leak detection 双保险 |
| 3 | WASM sandbox 适合现有 50 个工具 | architect + backend | 现有工具用 net/http、os/exec、DB driver，无法编译为 WASI；需完整 host binding | **降级**：F1 从 P2 实施降为 POC 调研，本轮不做 |
| 4 | Network allowlist 需独立于 SSRF 层 | backend-engineer | url_safety.go 已有基础，两层独立会产生维护和绕过风险 | **修改方案**：统一到 url_safety + DialContext，一个拦截点 |
| 5 | Safety check 放 HTTP middleware | architect | Injection 在 agent loop 内部，不在 API 边界 | **修改方案**：Safety interceptor 在 agent/runtime 层，不在 chain.go |

### 未决项

- Prompt injection 规则集初始来源：OWASP LLM Top 10 + HuggingFace 公开 injection dataset
- 是否需要 per-tenant 安全等级分档（T1 强制 enforce，T4 可选 log_only）：Phase 1 统一 log_only，Phase 2 按等级分

---

## Brownfield 上下文快照

| 模块 | 现状 | 本次改动影响 |
|------|------|-------------|
| `internal/middleware/chain.go` | 固定 9 层中间件栈 | **不改**，安全层不在此 |
| `internal/tools/registry.go` | 50 个工具，ToolHandler 签名 | **扩展 ToolContext**，增加 SecretResolver |
| `internal/tools/url_safety.go` | SSRF IP 黑名单 | **扩展**，增加 tenant allowlist |
| `internal/agent/` | Agent loop，LLM 调用 | **新增** safety interceptor wrap |
| `internal/api/admin/` | Admin CRUD endpoints | **新增** 3 组安全管理 API |
| `internal/store/` | PostgreSQL store layer | **新增** 3 张表 + migration |

---

## Story Slice 列表

### Phase 1: Prompt Injection Defense (F3) — Week 1-2

| Slice | 目标 | 验收标准 | Owner | 依赖 |
|-------|------|----------|-------|------|
| S1.1 | Safety interceptor 骨架 | interface 定义 + chain + noop impl + 测试 | backend-engineer | 无 |
| S1.2 | Input guard 规则引擎 | 20+ regex patterns，OWASP LLM Top 10 覆盖 | backend-engineer | S1.1 |
| S1.3 | Output guard | LLM/tool output policy enforcement | backend-engineer | S1.1 |
| S1.4 | Canary token 检测 | 系统 prompt canary 泄漏检测 | backend-engineer | S1.1 |
| S1.5 | Per-tenant safety policy | DB schema + store + Admin API | backend-engineer | S1.2 |
| S1.6 | Agent loop 集成 | Safety wrap LLM call，log_only 模式 | backend-engineer | S1.2, S1.3 |
| S1.7 | 集成测试 | 10+ injection 场景，误报率 < 5% | qa-engineer | S1.6 |

### Phase 2: Credential Isolation (F2) — Week 2-3

| Slice | 目标 | 验收标准 | Owner | 依赖 |
|-------|------|----------|-------|------|
| S2.1 | SecretResolver interface | ToolContext 扩展 + env vault 实现 | backend-engineer | 无 |
| S2.2 | Leak scanner | Aho-Corasick 多模式匹配，50+ patterns | backend-engineer | S2.1 |
| S2.3 | Tool output 泄漏检测集成 | tool dispatch 后自动扫描 | backend-engineer | S2.2 |
| S2.4 | 工具迁移 batch 1 | 高风险 10 个工具迁移到 SecretResolver | backend-engineer | S2.1 |
| S2.5 | Secret pattern 注册 API | Admin CRUD + DB migration | backend-engineer | S2.2 |
| S2.6 | Linter rule | 禁止 tools/ 包 os.Getenv 直接调用 | backend-engineer | S2.4 |
| S2.7 | 集成测试 | 泄漏检测率 ≥ 99%，hot-reload 验证 | qa-engineer | S2.3 |

### Phase 3: Network Allowlisting (F4) — Week 3-4

| Slice | 目标 | 验收标准 | Owner | 依赖 |
|-------|------|----------|-------|------|
| S3.1 | Egress policy engine | allowlist rules 存储 + 加载 + 匹配 | backend-engineer | 无 |
| S3.2 | Secure Transport | DialContext hook 统一 SSRF + allowlist | backend-engineer | S3.1 |
| S3.3 | url_safety 统一 | 现有 SSRF 逻辑合并到新 Transport | backend-engineer | S3.2 |
| S3.4 | Admin API | egress rules CRUD + 审计日志 | backend-engineer | S3.1 |
| S3.5 | Redis 缓存 | allowlist 热缓存，减少 DB 查询 | backend-engineer | S3.1 |
| S3.6 | DNS rebinding 防护 | DialContext 连接级 IP 验证 | backend-engineer | S3.2 |
| S3.7 | 集成测试 | allowlist 100% 准确，DNS rebinding 防护 | qa-engineer | S3.6 |

### Phase 4: WASM POC (F1) — Week 5 (仅调研)

| Slice | 目标 | 验收标准 | Owner | 依赖 |
|-------|------|----------|-------|------|
| S4.1 | wazero POC | 1 个简单工具在 WASM 中运行 | architect | 无 |
| S4.2 | 可行性报告 | 性能 benchmark + 限制评估 + 决策建议 | architect | S4.1 |

---

## 工作拆解

| 工作项 | 主责角色 | 依赖 | 计划周 |
|--------|----------|------|--------|
| Safety interceptor 设计 + 实现 | backend-engineer | arch-design 冻结 | W1-2 |
| Credential isolation 设计 + 实现 | backend-engineer | Safety 骨架完成 | W2-3 |
| Egress policy 设计 + 实现 | backend-engineer | url_safety 理解 | W3-4 |
| DB migration 编写 | backend-engineer | schema 冻结 | W1 |
| Admin API 实现 | backend-engineer | migration 就绪 | W2-4 |
| 安全测试用例设计 | qa-engineer | 各 feature 接口冻结 | W1-4 |
| 渗透测试场景 | qa-engineer | 实现完成 | W4 |
| 性能 benchmark | devops-engineer | 各 feature 集成完成 | W4 |
| WASM POC | architect | 无 | W5 |
| 部署验证 | devops-engineer | 全部集成完成 | W4-5 |

---

## 风险与缓解

| 风险 | 影响 | 缓解措施 | Owner |
|------|------|----------|-------|
| Injection 误报率超 5% | 用户体验劣化 | log_only 先行，收集 2 周数据 | backend-engineer |
| 50 个工具迁移 Secret 周期长 | 安全窗口期 | 分批：高风险 10 个先行 | backend-engineer |
| DialContext 性能开销 | P99 上升 | Redis 缓存 allowlist | backend-engineer |
| DNS rebinding 绕过 | allowlist 失效 | 连接级 IP 重验证 | backend-engineer |
| 规则集维护成本 | 检测率下降 | 社区规则集 + 定期更新机制 | tech-lead |

---

## 节点检查

| 检查点 | 时间 | 条件 |
|--------|------|------|
| 方案评审通过 | W1 D1 | arch-design 冻结，挑战会结论落盘 |
| F3 Safety interceptor 可 demo | W2 D5 | log_only 模式跑通完整 agent loop |
| F2 Leak scanner 可 demo | W3 D3 | 输出泄漏检测 + mask 工作 |
| F4 Egress policy 可 demo | W4 D3 | allowlist 拦截 + 审计日志 |
| 全量集成测试通过 | W4 D5 | 1585 existing + 30+ new tests |
| 性能验证通过 | W5 D2 | P99 < 50ms overhead confirmed |
| 发布准备 | W5 D5 | release-plan 就绪 |

---

## 角色分工

| 角色 | 职责 | 交接节点 |
|------|------|----------|
| tech-lead | 冲突仲裁、放行决策、挑战会收口 | Plan → Execute handoff |
| architect | 安全架构设计、WASM POC、接口契约 | Design → Execute |
| backend-engineer | 3 feature 实现、migration、Admin API | Execute → Review |
| qa-engineer | 安全测试、渗透场景、放行建议 | Review → Release |
| devops-engineer | 性能验证、部署、监控扩展 | Release → Operate |

---

## 应用等级 / 技术架构等级

| 维度 | 判断 |
|------|------|
| 应用等级 | T2 |
| 技术架构等级 | T2 |
| 关键组件偏离 | 无（Go 原生，无新外部依赖） |
| ADR 需求 | 是 — D4 WASM 降级决策需记录 |

---

## 是否需要 ADR

**是。** 需要记录：
- ADR: WASM Sandbox 降级为 POC 的决策（F1 scope change）
- ADR: Safety Layer 位置决策（agent loop 内而非 HTTP middleware）

---

## 技能装配清单

| 技能 | 触发原因 | 主责角色 |
|------|----------|----------|
| `security-review` | 每个 feature 安全审查 | qa-engineer |
| `golang-patterns` | Go 实现模式 | backend-engineer |
| `api-design` | Admin API 设计 | architect |
| `database-migrations` | 3 张新表 | backend-engineer |
| `golang-testing` | 集成测试 | qa-engineer |

---

## 前端交付物与检查点

**本次无前端变更。** Admin 管理 UI 后续独立任务。

---

## Implementation Readiness 结论

| 维度 | 状态 | 说明 |
|------|------|------|
| Challenge 完成 | ✅ | 5 条核心假设已挑战并决议 |
| Design review | ✅ | arch-design.md 含系统边界、组件拆分、数据流、接口 |
| Handoff 可执行 | ✅ | story slice 粒度可直接进入 /team-execute |
| 阻塞项 | 无 | — |

**结论：** Plan 阶段完成，ready for `/team-execute`。
