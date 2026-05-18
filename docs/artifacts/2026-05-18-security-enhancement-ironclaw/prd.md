# PRD: HermesX Security Enhancement — Borrowing from IronClaw

> **Slug:** security-enhancement-ironclaw  
> **State:** intake  
> **Date:** 2026-05-18  
> **Owner:** tech-lead  
> **Status:** draft

---

## 背景

基于对 [IronClaw](https://github.com/Colin4k1024/ironclaw)（Rust Agent OS，privacy-first 个人 AI 助手）安全模型的竞品分析，识别出 4 个 HermesX 当前缺失且具备高企业价值的安全增强方向。

**触发原因：**
- HermesX 当前工具隔离依赖 Docker sandbox，粒度为"代码执行级"而非"工具调用级"
- 凭证管理直接通过环境变量暴露给工具运行时，无泄漏检测
- 无 prompt injection 防护层，agent 可被用户输入/文档/工具结果劫持
- 工具 HTTP 出口无管控，存在数据外泄风险

**当前约束：**
- HermesX v2.1.1，Go 1.25，单二进制部署
- 已有 Docker sandbox、多租户 RLS、4 层认证链、审计日志
- 生产环境已有 3 副本 + Redis + PostgreSQL + MinIO + OTel

---

## 目标与成功标准

### 业务目标

为 HermesX 补齐 agent 安全纵深防御能力，使其达到企业级 AI agent 的安全基线，防止 prompt injection、credential leak、data exfiltration 三大攻击面。

### 用户价值

- 企业管理员：对 agent 行为边界有可审计、可配置的控制力
- 终端用户：agent 不会因恶意输入被劫持，个人凭证不会泄漏
- 安全团队：网络出口可白名单管控，满足合规审计要求

### 成功指标

| 指标 | 目标 |
|------|------|
| Prompt injection 检测率 | 已知模式 ≥ 95% 拦截 |
| Credential leak 检测率 | 已注册 secret pattern ≥ 99% 检出 |
| 网络白名单拦截准确率 | 100%（无漏放） |
| 安全层性能开销 | P99 < 50ms |
| WASM sandbox 工具启动延迟 | < 10ms |
| 零回归 | 现有 1585 tests 全部通过 |

---

## 用户故事

### US-1: Prompt Injection Defense (P0)

**作为** 企业安全管理员  
**我希望** agent 在处理用户输入前自动检测和拦截 prompt injection 攻击  
**以便** 防止 agent 被劫持执行未授权操作

**验收标准：**
- [ ] 用户输入经过 input pattern detection（已知注入模式 regex + heuristic）
- [ ] 工具返回结果经过 content sanitization
- [ ] LLM 输出经过 output policy enforcement
- [ ] 支持 canary token 检测（context leakage detection）
- [ ] 安全策略 per-tenant 可配置（Admin API CRUD）
- [ ] 检测到注入时：拦截 + 审计记录 + 可选告警
- [ ] 性能开销 P99 < 20ms

### US-2: Credential Isolation & Leak Detection (P1)

**作为** 平台运维工程师  
**我希望** 工具运行时永远无法直接读取明文 secret  
**以便** 即使工具代码有漏洞或 LLM 被诱导，凭证也不会泄漏

**验收标准：**
- [ ] Secret 通过 opaque handle 传递给工具，工具侧无法读取原文
- [ ] Host boundary injection：只在 HTTP 请求发出瞬间注入真实值
- [ ] LLM output scanner：检测输出中是否包含已注册 secret pattern
- [ ] Tool output scanner：工具返回值泄漏检测
- [ ] 检测到泄漏时：自动 mask + 审计记录 + 告警
- [ ] 支持 secret rotation 不重启服务（热加载）
- [ ] 与现有 API Key（hk_ prefix）和 env var 管理兼容

### US-3: Network Endpoint Allowlisting (P1)

**作为** 企业安全管理员  
**我希望** 限制 agent 工具只能访问经过审批的外部服务  
**以便** 防止数据通过工具调用外泄到未授权目标

**验收标准：**
- [ ] Per-tenant endpoint allowlist 存储在 PostgreSQL
- [ ] 支持 host + path prefix 粒度（如 `api.openai.com/v1/*`）
- [ ] 支持 CIDR 范围和通配符域名（`*.internal.corp`）
- [ ] 未授权请求自动拦截，返回明确错误给 agent
- [ ] 所有拦截和放行记录写入审计日志
- [ ] Admin API: CRUD allowlist rules
- [ ] 默认策略可配置：deny-all（严格）/ allow-all（宽松）/ log-only（观察）
- [ ] 不影响 LLM provider 调用（内置白名单）

### US-4: WASM Tool Sandbox (P2)

**作为** 平台架构师  
**我希望** 第三方工具在 WASM 沙箱中运行，每个工具有独立的资源和权限边界  
**以便** 恶意或有缺陷的工具无法影响宿主进程和其他租户

**验收标准：**
- [ ] 使用 wazero（Go native WASM runtime，零 CGO 依赖）
- [ ] 每个工具调用在独立 WASM instance 中执行
- [ ] Capability-based 权限：fs_read, fs_write, net_http, env_read 等
- [ ] 资源限制：内存上限、执行时间上限、CPU 配额
- [ ] 工具 panic/OOM 不影响宿主进程
- [ ] 工具启动延迟 < 10ms（预编译 AOT）
- [ ] 兼容现有 tool interface（adapter pattern）
- [ ] Docker sandbox 保留为 code execution fallback

---

## 范围

### In Scope

- F3: Prompt Injection Defense Layer（safety middleware）
- F2: Credential Isolation & Leak Detection（secret manager 重构）
- F4: Network Endpoint Allowlisting（egress policy engine）
- F1: WASM Tool Sandbox（wazero integration）
- Admin API 扩展（安全策略 CRUD）
- 审计日志扩展（安全事件类型）
- per-tenant 安全配置存储
- 集成测试 + 安全测试

### Out of Scope

- 前端 UI（管理界面后续独立任务）
- 多通道扩展（不在本次范围）
- LLM provider 替换或新增
- 现有 Docker sandbox 移除（保留为 fallback）
- 性能压测重做（仅验证开销在预算内）

---

## 风险与依赖

### 关键依赖

| 依赖 | 说明 | 状态 |
|------|------|------|
| wazero v2.x | Go native WASM runtime | 稳定，已有 production 案例 |
| 现有 tool interface | 需要 adapter 兼容 | 已有 `Tool` interface 定义 |
| PostgreSQL | 安全策略存储 | 已有，需新增 migration |
| Redis | 缓存 allowlist 规则 | 已有 |
| 审计系统 | 安全事件记录 | 已有 `audit_logs` 表 |

### 风险

| 风险 | 影响 | 缓解 |
|------|------|------|
| Prompt injection 误报率过高 | 正常用户请求被拦截 | 先上 log-only 模式，收集 baseline |
| WASM 生态 Go 工具链不成熟 | 开发周期延长 | P2 优先级，可延后；Docker 兜底 |
| Secret handle 机制对现有工具侵入大 | 大量工具需适配 | Adapter pattern + 渐进式迁移 |
| 性能开销超预算 | 影响 SLA | 每层独立 benchmark，超标则降级 |
| wazero 对 WASI 支持不完整 | 部分工具无法迁移 | 分类：简单工具走 WASM，复杂工具留 Docker |

### 待确认项

- [ ] Prompt injection 规则集初始来源（OWASP LLM Top 10? 自建?）
- [ ] Secret pattern 注册机制（手动 vs 自动发现）
- [ ] 网络白名单初始策略（deny-all 还是 allow-all + log）
- [ ] WASM 工具接口标准（WIT? custom ABI?）
- [ ] 安全事件告警通道（webhook? email? Slack?）
- [ ] 是否需要 per-tenant 安全等级（T1 强制全部，T4 可选）

---

## 企业治理待确认项

| 维度 | 判断 |
|------|------|
| 应用等级 | T2（多租户 SaaS，含企业数据） |
| 技术架构等级 | T2（多副本，高可用要求） |
| 数据/合规风险 | 是 — 涉及凭证管理和网络出口管控 |
| 集团组件约束 | 无（独立产品） |
| 私有 overlay 需求 | 无 |

---

## 参与角色清单

| 角色 | 职责 |
|------|------|
| `tech-lead` | Intake 收口、优先级仲裁、设计评审 |
| `architect` | 安全架构设计、WASM 集成方案、接口契约 |
| `backend-engineer` | 4 个 feature 实现、migration、测试 |
| `qa-engineer` | 安全测试用例、渗透测试场景、放行建议 |
| `devops-engineer` | 部署验证、性能验证、监控扩展 |

---

## 领域技能包启用建议

| 技能 | 理由 |
|------|------|
| `security-review` | 每个 feature 完成后必须安全审查 |
| `golang-patterns` | Go 实现模式参考 |
| `api-design` | Admin API 扩展设计 |
| `database-migrations` | 新增安全策略表 |

---

## UI 范围

本次无前端变更。Admin API 先行，管理 UI 后续独立任务。

---

## 需求挑战会候选分组

### Group 1: Safety Architecture (F3 + F2)

**参与:** architect, backend-engineer, tech-lead

**挑战焦点:**
- Prompt injection 检测精度与误报率平衡
- Credential isolation 对现有 50 个工具的侵入度
- 安全层在中间件栈中的位置（Tracing → Metrics → RequestID → Auth → **Safety?** → Tenant → ...）

### Group 2: Network & Isolation (F4 + F1)

**参与:** architect, backend-engineer, devops-engineer

**挑战焦点:**
- 网络白名单在 Go HTTP client 层的拦截点（Transport? Middleware?）
- WASM sandbox 与现有 Docker sandbox 的共存策略
- wazero 性能 benchmark 与限制评估

---

## 实施节奏建议

| Phase | Feature | 目标周 |
|-------|---------|--------|
| Phase 1 | F3: Prompt Injection Defense | Week 1-2 |
| Phase 2 | F2: Credential Isolation | Week 2-3 |
| Phase 3 | F4: Network Allowlisting | Week 3-4 |
| Phase 4 | F1: WASM Sandbox | Week 5-7 |

---

## 关键假设

1. IronClaw 的安全模式可以在 Go 生态中等价实现（wazero 替代 wasmtime）
2. 现有 50 个工具中 80%+ 可以在不修改工具代码的情况下适配新安全层
3. Prompt injection 防御可以先从规则匹配开始，后续再引入 ML classifier
4. 企业客户更看重"可控"（白名单、审计）而非"智能"（自动判断）

## 非目标

- 不做 AI-based prompt injection detection（先做规则）
- 不做完整的 WAF（只做 agent 层面的安全增强）
- 不做前端安全策略管理 UI（本期）
- 不替换现有 Docker sandbox（共存）
