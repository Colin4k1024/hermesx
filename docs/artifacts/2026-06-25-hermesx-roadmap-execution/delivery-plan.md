# HermesX Roadmap Execution - Delivery Plan

> **状态**: Draft  
> **创建时间**: 2026-06-25  
> **主责角色**: Tech Lead  
> **来源**: PRD - HermesX Roadmap Execution

---

## 一、需求挑战会结论

### 核心假设挑战

#### 假设 1: ExecutionReceipt 已经完全实现并覆盖所有 50 个工具调用

**质疑人**: Architect  
**质疑内容**: 
- 当前只检查了 `execution_receipts` 表存在，但没有验证 before/after hook 是否真正集成到所有工具调用路径
- 工具调用路径是否包含幂等 dedup 逻辑？
- trace_id 与 OTel span 绑定是否已实现？

**替代路径**: 
- 先运行一次完整的工具调用测试，验证 receipt 是否真正生成
- 检查 `internal/tools/executor.go` 中的 hook 实现

**阻断条件**: 
- 如果 hook 没有集成到所有 50 个工具，需要先补全
- 如果幂等 dedup 没有实现，需要先补全

**结论**: 需要验证 ExecutionReceipt 的完整实现，否则会影响 Month 2 的 Oris 指标上报

---

#### 假设 2: Metering 聚合 API 已经通过集成测试

**质疑人**: QA Engineer  
**质疑内容**:
- `adminUsageAggregation` 函数存在，但测试覆盖率未知
- 是否有测试用例覆盖 daily/monthly 聚合？
- 是否有测试用例覆盖边界条件（空数据、超大范围）？

**替代路径**:
- 运行现有测试，检查覆盖率
- 补充缺失的测试用例

**阻断条件**:
- 如果测试覆盖率 < 80%，需要先补全
- 如果有未覆盖的边界条件，需要先补全

**结论**: 需要验证 Metering 聚合 API 的测试覆盖率，否则会影响 Month 1 的 exit criteria

---

#### 假设 3: 安全债务已经完全关闭

**质疑人**: Security Engineer  
**质疑内容**:
- API Key tenant_id 越权修复是否已验证？
- rand.Read 错误处理是否已验证？
- cross-tenant 渗透测试是否已添加？

**替代路径**:
- 运行现有安全测试
- 添加缺失的渗透测试用例

**阻断条件**:
- 如果渗透测试没有添加，需要先补全
- 如果安全测试覆盖率 < 100%，需要先补全

**结论**: 需要添加 cross-tenant 渗透测试，否则 Month 1 的 exit criteria 不满足

---

### 需求挑战会总结

| 假设 | 质疑人 | 结论 | 未决项 |
|------|--------|------|--------|
| ExecutionReceipt 完整实现 | Architect | 需要验证 hook 集成 | 幂等 dedup 实现 |
| Metering API 测试覆盖 | QA Engineer | 需要验证测试覆盖率 | 边界条件测试 |
| 安全债务完全关闭 | Security Engineer | 需要添加渗透测试 | cross-tenant 测试 |

---

## 二、Brownfield 上下文快照

### 现有模块边界

| 模块 | 路径 | 职责 | 外部依赖 |
|------|------|------|----------|
| API Server | `internal/api/` | HTTP 处理、路由 | 无 |
| Admin Handler | `internal/api/admin/` | 管理端 API | 无 |
| Metering | `internal/metering/` | 用量计量 | PostgreSQL/MySQL |
| Store | `internal/store/` | 数据访问层 | PostgreSQL/MySQL |
| Agent | `internal/agent/` | Agent 运行时 | 无 |
| Tools | `internal/tools/` | 工具执行 | 无 |

### 历史约束

1. **数据库兼容性**: 同时支持 PostgreSQL 和 MySQL
2. **多租户隔离**: PG RLS + 55 条 policy
3. **认证链**: Static Token → API Key → JWT → OIDC
4. **可观测性**: OTel + Prometheus + slog

### 缺失文档

1. OpenAPI 规范
2. SDK 使用文档
3. L2 接口合约文档
4. K8s 沙箱部署文档

---

## 三、应用等级 / 技术架构等级

### 应用等级

- **等级**: L1（治理/基础设施）
- **说明**: hermesx 定位为 Agent-first Runtime Control Plane，是 L1 层的治理基础设施

### 技术架构等级

- **等级**: Enterprise
- **说明**: 支持多租户、高可用、安全审计、合规要求

### 关键组件偏离

| 组件 | 偏离 | 影响 |
|------|------|------|
| ExecutionReceipt | 已实现但需要验证完整性 | Month 2 的 Oris 指标上报 |
| Metering API | 已实现但需要验证测试覆盖 | Month 1 的 exit criteria |
| 安全债务 | 已修复但需要添加渗透测试 | Month 1 的 exit criteria |

---

## 四、Story Slice 列表

### Story 1: 验证 ExecutionReceipt 完整实现

**目标**: 验证 ExecutionReceipt 是否真正覆盖所有 50 个工具调用  
**验收标准**:
- [ ] 运行一次完整的工具调用测试
- [ ] 验证 receipt 是否真正生成
- [ ] 验证 trace_id 与 OTel span 绑定
- [ ] 验证幂等 dedup 逻辑

**依赖**: 无  
**Owner**: Backend Engineer  
**Handoff**: Tech Lead 验证

---

### Story 2: 验证 Metering 聚合 API 测试覆盖

**目标**: 验证 Metering 聚合 API 的测试覆盖率  
**验收标准**:
- [ ] 运行现有测试，检查覆盖率
- [ ] 补充缺失的测试用例
- [ ] 测试覆盖率 ≥ 80%

**依赖**: 无  
**Owner**: QA Engineer  
**Handoff**: Tech Lead 验证

---

### Story 3: 添加 cross-tenant 渗透测试

**目标**: 添加 cross-tenant 渗透测试用例  
**验收标准**:
- [ ] `TestCrossTenantAPIKeyCreation` 测试通过
- [ ] `TestRLSBypassViaSubquery` 测试通过
- [ ] `TestAdminEndpointTenantLeakage` 测试通过

**依赖**: 无  
**Owner**: Security Engineer  
**Handoff**: Tech Lead 验证

---

### Story 4: 编写 OpenAPI 规范

**目标**: 编写 OpenAPI 3.1.0 规范，覆盖所有 /v1/ 端点  
**验收标准**:
- [ ] `docs/openapi.yaml` 覆盖所有 /v1/ 端点
- [ ] 包含 auth scheme、请求/响应 schema、error codes
- [ ] OpenAPI 版本为 3.1.0

**依赖**: 无  
**Owner**: Backend Engineer  
**Handoff**: Architect 验证

---

### Story 5: 生成 Go SDK

**目标**: 从 OpenAPI 规范生成 Go SDK  
**验收标准**:
- [ ] `sdk/go/` 目录包含生成的客户端代码
- [ ] 包含所有 /v1/ 端点的类型定义和方法
- [ ] 可以通过 `go get` 安装

**依赖**: Story 4  
**Owner**: Backend Engineer  
**Handoff**: Tech Lead 验证

---

### Story 6: 生成 TypeScript SDK

**目标**: 从 OpenAPI 规范生成 TypeScript SDK  
**验收标准**:
- [ ] `sdk/typescript/` 目录包含生成的客户端代码
- [ ] 包含 TypeScript 类型定义
- [ ] 可以通过 `npm install` 安装

**依赖**: Story 4  
**Owner**: Frontend Engineer  
**Handoff**: Tech Lead 验证

---

### Story 7: 定义 L2 接口合约

**目标**: 定义 governance client interface  
**验收标准**:
- [ ] `pkg/governance/client.go` 定义接口
- [ ] 接口包含 GetTenant、GetExecutionReceipts、GetTenantQuota、GetSandboxPolicy
- [ ] superagent-base 有 adapter 实现

**依赖**: Story 4  
**Owner**: Backend Engineer  
**Handoff**: Architect 验证

---

### Story 8: 实现 Oris 指标上报

**目标**: 在 ExecutionReceipt AfterExec hook 中上报 OrisMetricEvent  
**验收标准**:
- [ ] ExecutionReceipt AfterExec hook 上报 OrisMetricEvent
- [ ] 包含 AgentID、ToolName、SuccessRate、AvgLatencyMs、ErrorTypes、TraceID
- [ ] 通过 OTel custom span attributes 或 HTTP 上报

**依赖**: Story 1  
**Owner**: Backend Engineer  
**Handoff**: Tech Lead 验证

---

### Story 9: 验证 Eino Agent 集成

**目标**: 跑一次真实的 Eino ReAct 执行  
**验收标准**:
- [ ] 覆盖：工具调用 → Safety Pipeline 过滤 → ExecutionReceipt 写入 → OTel trace
- [ ] 验证 ToolAdapter 和 ModelAdapter 在多 LLM provider 下的行为

**依赖**: Story 1  
**Owner**: Backend Engineer  
**Handoff**: Tech Lead 验证

---

### Story 10: 重写 CONTRIBUTING.md

**目标**: 重写贡献指南，包含完整的本地 setup + tool/adapter 贡献指南  
**验收标准**:
- [ ] CONTRIBUTING.md 包含本地环境 setup
- [ ] 包含新 tool 实现步骤
- [ ] 包含新 platform adapter 实现步骤
- [ ] 包含 PR 提交规范和 review 周期承诺

**依赖**: 无  
**Owner**: Tech Lead  
**Handoff**: 无

---

### Story 11: 重写 README 核心叙事

**目标**: 重写 README，包含 Why HermesX 章节  
**验收标准**:
- [ ] README 包含 Why HermesX 章节
- [ ] 包含与 Dify/CrewAI/LangGraph 的对比
- [ ] 包含架构图链接

**依赖**: 无  
**Owner**: Tech Lead  
**Handoff**: 无

---

### Story 12: 创建 Good First Issues

**目标**: 创建真实的 Good First Issues  
**验收标准**:
- [ ] "Add a new platform adapter for Notion"
- [ ] "Write integration test for rate limiter under concurrent load"
- [ ] "Add govulncheck baseline suppression file"
- [ ] "Document the ExecutionReceipt API in openapi.yaml"

**依赖**: Story 4  
**Owner**: Tech Lead  
**Handoff**: 无

---

### Story 13: 实现 K8s Job Sandbox

**目标**: 实现 Job-based sandbox  
**验收标准**:
- [ ] SANDBOX_MODE=k8s-job 配置可用
- [ ] Agent 可以通过 K8s Job 执行工具调用
- [ ] 无需 DinD（Docker-in-Docker）

**依赖**: 无  
**Owner**: DevOps Engineer  
**Handoff**: Security Engineer 验证

---

### Story 14: 准备 v3.0.0 发布

**目标**: 准备 v3.0.0 发布  
**验收标准**:
- [ ] 测试覆盖率 ≥ 75%
- [ ] 0 个 HIGH/CRITICAL 已知漏洞
- [ ] Docker compose quickstart 在全新机器上 < 5 分钟跑通
- [ ] enterprise-saas-demo 所有 11 步通过
- [ ] Migration guide 编写完成

**依赖**: 所有 Story  
**Owner**: Tech Lead  
**Handoff**: 无

---

## 五、角色分工

| 角色 | 职责 | Story |
|------|------|-------|
| Tech Lead | 需求 intake、任务拆解、角色分派、冲突决策与最终交付收口 | Story 10, 11, 12, 14 |
| Backend Engineer | OpenAPI 规范编写、SDK 生成、L2 接口合约、Oris 指标上报 | Story 1, 4, 5, 7, 8, 9 |
| Frontend Engineer | TypeScript SDK 生成 | Story 6 |
| Security Engineer | cross-tenant 渗透测试、安全评审 | Story 3, 13 |
| QA Engineer | 测试用例验证、质量门禁验证 | Story 2 |
| DevOps Engineer | K8s 沙箱实现、本地环境 setup | Story 13 |
| Architect | 接口设计评审、集成方案评审 | Story 4, 7 |

---

## 六、风险与依赖

### 风险

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| ExecutionReceipt hook 未完全集成 | 高 | 中 | 先验证完整实现，再进行 Oris 指标上报 |
| Metering API 测试覆盖率不足 | 高 | 中 | 先验证测试覆盖率，再进行 Month 1 exit criteria |
| OpenAPI 规范与实现漂移 | 高 | 中 | 使用 oapi-codegen 从 OpenAPI 生成 Go 服务端 stub |
| Eino Agent 集成不完整 | 高 | 中 | 先跑一次真实的 ReAct 执行，覆盖完整路径 |
| 外部贡献者为零 | 中 | 高 | 创建真实的 Good First Issues，重写 CONTRIBUTING.md |
| K8s Job sandbox 冷启动延迟 | 低 | 高 | 先实现 Job-based 方案，sidecar 方案留到 v3.x |

### 依赖

| Story | 依赖 Story | 说明 |
|-------|------------|------|
| Story 5 | Story 4 | Go SDK 依赖 OpenAPI 规范 |
| Story 6 | Story 4 | TypeScript SDK 依赖 OpenAPI 规范 |
| Story 7 | Story 4 | L2 接口合约依赖 OpenAPI 规范 |
| Story 8 | Story 1 | Oris 指标上报依赖 ExecutionReceipt |
| Story 9 | Story 1 | Eino Agent 集成依赖 ExecutionReceipt |
| Story 12 | Story 4 | Good First Issues 依赖 OpenAPI 规范 |
| Story 14 | 所有 Story | v3.0.0 发布依赖所有 Story |

---

## 七、技能装配清单

| 技能包 | 启用原因 | 优先级 |
|--------|----------|--------|
| security-review | cross-tenant 渗透测试 | P0 |
| api-design | OpenAPI 规范编写 | P0 |
| golang-patterns | Go SDK 生成 | P0 |
| frontend-patterns | TypeScript SDK 生成 | P0 |
| backend-patterns | L2 接口合约 | P1 |
| devops-engineer | K8s 沙箱实现 | P2 |

---

## 八、Implementation Readiness 结论

### 执行前提

1. **ExecutionReceipt 完整实现**: 需要验证 before/after hook 是否真正集成到所有工具调用路径
2. **Metering API 测试覆盖**: 需要验证测试覆盖率 ≥ 80%
3. **安全债务完全关闭**: 需要添加 cross-tenant 渗透测试

### 阻塞项

| 阻塞项 | 影响 Story | 解决方案 |
|--------|------------|----------|
| ExecutionReceipt hook 未验证 | Story 8, 9 | 先运行完整工具调用测试 |
| Metering API 测试覆盖不足 | Month 1 exit criteria | 先验证测试覆盖率 |
| cross-tenant 渗透测试未添加 | Month 1 exit criteria | 先添加测试用例 |

### 可执行性判断

- **当前状态**: NOT READY
- **原因**: 需要先验证 ExecutionReceipt 完整实现、Metering API 测试覆盖、添加 cross-tenant 渗透测试
- **预计就绪时间**: Week 1 结束

---

## 九、时间线

```
Week 1:  Story 1, 2, 3（验证 + 渗透测试）
Week 2-3: Story 4（OpenAPI 规范）
Week 3-4: Story 5, 6（Go SDK + TypeScript SDK）
Week 4-5: Story 7（L2 接口合约）
Week 5-6: Story 8, 9（Oris 指标上报 + Eino Agent 集成）
Week 7-8: Story 10, 11, 12（外部贡献者入口）
Week 9-10: Story 13（K8s 沙箱）
Week 11-12: Story 14（v3.0.0 发布准备）
```

---

## 十、下一步行动

1. **立即**: 验证 ExecutionReceipt 完整实现
2. **本周**: 验证 Metering API 测试覆盖
3. **本周**: 添加 cross-tenant 渗透测试
4. **下周**: 开始 OpenAPI 规范编写
