# HermesX Roadmap Execution - Product Requirements Document

> **状态**: Confirmed  
> **创建时间**: 2026-06-25  
> **主责角色**: Tech Lead  
> **来源**: Issue #43 - HermesX 未来三个月计划

---

## 一、需求背景

HermesX 作为 Agent-first Runtime Control Plane，已经在三个月内从概念演进到了 78K+ LoC 的工程体。根据 Issue #43 的审查报告，项目已经完成了大部分 Month 1 的核心工作：

**已完成项**：
- ExecutionReceipt 实现
- Metering 聚合 API
- 安全债务修复（API Key tenant_id 越权、rand.Read 错误处理）
- GHA action digest pinning
- Grafana dashboard
- govulncheck + trivy CI 集成

**待完成项**：
- Month 1 剩余：cross-tenant 渗透测试
- Month 2：OpenAPI 规范、SDK 生成、L2 接口合约、Oris 指标上报、Eino Agent 集成验证
- Month 3：外部贡献者入口、K8s 沙箱方案、v3.0.0 发布准备

---

## 二、目标与优先级

### 核心目标

将 hermesx 从"单人可用"变成"系统可集成"，再变成"有真实用户的项目"。

### 优先级矩阵

| 优先级 | 工作项 | 时间窗口 | 依赖 |
|--------|--------|----------|------|
| P0 | cross-tenant 渗透测试 | Week 1 | 无 |
| P0 | OpenAPI 规范 | Week 2-3 | 无 |
| P0 | Go SDK + TypeScript SDK | Week 3-4 | OpenAPI |
| P1 | L2 接口合约 | Week 4 | OpenAPI |
| P1 | Oris 指标上报 | Week 5 | ExecutionReceipt |
| P1 | Eino Agent 集成验证 | Week 5-6 | 无 |
| P2 | 外部贡献者入口 | Week 7-8 | 无 |
| P2 | K8s 沙箱方案 | Week 9-10 | 无 |
| P2 | v3.0.0 发布准备 | Week 11-12 | 所有 P0/P1 |

---

## 三、关键假设与成功标准

### 关键假设

1. **假设 1**: ExecutionReceipt 已经完全实现并覆盖所有 50 个工具调用
   - 验证方式: 检查 `execution_receipts` 表和 hook 实现
   
2. **假设 2**: Metering 聚合 API 已经通过集成测试
   - 验证方式: 运行 `adminUsageAggregation` 相关测试
   
3. **假设 3**: 安全债务已经完全关闭
   - 验证方式: 检查 API Key 创建逻辑和 rand.Read 错误处理

### 成功标准

| 标准 | 量化指标 |
|------|----------|
| 安全性 | 0 个 HIGH/CRITICAL 已知漏洞 |
| 可审计性 | 所有工具调用有 ExecutionReceipt 记录 |
| 可集成性 | OpenAPI 覆盖所有 /v1/ 端点，SDK 可用 |
| 可观测性 | Grafana dashboard 覆盖 5 个关键面板 |
| 社区可见性 | ≥ 3 个 Good First Issues 有外部 PR |

---

## 四、范围定义

### In Scope（范围内）

1. **安全闭合**
   - cross-tenant 渗透测试
   - 安全扫描 CI 集成验证

2. **API 表面开放**
   - OpenAPI 3.1.0 规范编写
   - Go SDK 生成
   - TypeScript SDK 生成

3. **L2 接口合约**
   - governance client interface 定义
   - superagent-base adapter 实现

4. **Oris 集成**
   - 指标上报接口
   - ExecutionReceipt AfterExec hook 扩展

5. **Eino Agent 集成**
   - ReAct 执行路径验证
   - ToolAdapter/ModelAdapter 测试

6. **外部贡献者入口**
   - CONTRIBUTING.md 重写
   - README 核心叙事重写
   - Good First Issues 创建

7. **K8s 沙箱**
   - Job-based sandbox 实现
   - SANDBOX_MODE=k8s-job 配置

8. **v3.0.0 发布**
   - 质量门禁验证
   - Migration guide 编写

### Out of Scope（范围外）

1. Python SDK（Month 3 根据外部需求决定）
2. SIEM 集成（留到有真实客户提出合规需求）
3. GDPR soft-delete + 30 天宽限期（先解决有用户的问题）
4. gVisor/Firecracker 沙箱（v4+ 话题）
5. 双向 Oris 集成（Month 2 先做单向）

---

## 五、用户故事

### US-1: 作为安全工程师，我需要验证 cross-tenant 攻击是否被阻止

**验收标准**:
- [ ] `TestCrossTenantAPIKeyCreation` 测试通过
- [ ] `TestRLSBypassViaSubquery` 测试通过
- [ ] `TestAdminEndpointTenantLeakage` 测试通过

### US-2: 作为平台集成者，我需要 OpenAPI 规范来生成客户端

**验收标准**:
- [ ] `docs/openapi.yaml` 覆盖所有 /v1/ 端点
- [ ] 包含 auth scheme、请求/响应 schema、error codes
- [ ] OpenAPI 版本为 3.1.0

### US-3: 作为开发者，我需要 Go SDK 来调用 hermesx API

**验收标准**:
- [ ] `sdk/go/` 目录包含生成的客户端代码
- [ ] 包含所有 /v1/ 端点的类型定义和方法
- [ ] 可以通过 `go get` 安装

### US-4: 作为前端开发者，我需要 TypeScript SDK 来集成 hermesx

**验收标准**:
- [ ] `sdk/typescript/` 目录包含生成的客户端代码
- [ ] 包含 TypeScript 类型定义
- [ ] 可以通过 `npm install` 安装

### US-5: 作为 L2 集成者，我需要 governance client interface

**验收标准**:
- [ ] `pkg/governance/client.go` 定义接口
- [ ] 接口包含 GetTenant、GetExecutionReceipts、GetTenantQuota、GetSandboxPolicy
- [ ] superagent-base 有 adapter 实现

### US-6: 作为 Oris 集成者，我需要 hermesx 上报执行指标

**验收标准**:
- [ ] ExecutionReceipt AfterExec hook 上报 OrisMetricEvent
- [ ] 包含 AgentID、ToolName、SuccessRate、AvgLatencyMs、ErrorTypes、TraceID
- [ ] 通过 OTel custom span attributes 或 HTTP 上报

### US-7: 作为新贡献者，我需要清晰的贡献指南

**验收标准**:
- [ ] CONTRIBUTING.md 包含本地环境 setup
- [ ] 包含新 tool 实现步骤
- [ ] 包含新 platform adapter 实现步骤
- [ ] README 包含 Why HermesX 章节

### US-8: 作为运维工程师，我需要 K8s Job sandbox

**验收标准**:
- [ ] SANDBOX_MODE=k8s-job 配置可用
- [ ] Agent 可以通过 K8s Job 执行工具调用
- [ ] 无需 DinD（Docker-in-Docker）

---

## 六、风险与缓解

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| OpenAPI 规范与实现漂移 | 高 | 中 | 使用 oapi-codegen 从 OpenAPI 生成 Go 服务端 stub |
| Eino Agent 集成不完整 | 高 | 中 | 先跑一次真实的 ReAct 执行，覆盖完整路径 |
| 外部贡献者为零 | 中 | 高 | 创建真实的 Good First Issues，重写 CONTRIBUTING.md |
| K8s Job sandbox 冷启动延迟 | 低 | 高 | 先实现 Job-based 方案，sidecar 方案留到 v3.x |

---

## 七、已确认决策

1. **决策 1**: hermesx 的目标用户是谁？
   - ✅ **已确认**: 企业自部署
   - **影响**: K8s Job sandbox + 多副本 HA + Helm chart 是 P0

2. **决策 2**: Oris 集成是双向的还是单向的？
   - ✅ **已确认**: 单向（hermesx → Oris）
   - **影响**: Month 2 先做单向，验证了再扩展

3. **决策 3**: SDK 语言优先级
   - ✅ **已确认**: Go + TypeScript（Month 2）
   - **影响**: Python SDK 留到 Month 3 根据外部需求决定

---

## 八、企业治理待确认项

1. **应用等级**: 需要确认 hermesx 的应用等级（L1-L4）
2. **技术架构等级**: 需要确认技术架构等级
3. **数据/合规风险**: 需要确认是否有数据/合规风险
4. **私有 enterprise overlay**: 需要确认是否需要私有 enterprise overlay

---

## 九、领域技能包启用建议

| 技能包 | 启用原因 | 优先级 |
|--------|----------|--------|
| security-review | cross-tenant 渗透测试 | P0 |
| api-design | OpenAPI 规范编写 | P0 |
| golang-patterns | Go SDK 生成 | P0 |
| frontend-patterns | TypeScript SDK 生成 | P0 |
| backend-patterns | L2 接口合约 | P1 |
| devops-engineer | K8s 沙箱实现 | P2 |

---

## 十、UI 范围、终端假设与质量门禁

### UI 范围

- **WebUI**: 无新增 UI 变更，现有 WebUI 已覆盖管理功能
- **CLI**: 无新增 CLI 变更

### 终端假设

- **浏览器**: Chrome 90+, Firefox 88+, Safari 14+
- **Node.js**: 20+（用于 SDK 构建）
- **Go**: 1.25+（用于 SDK 生成）

### 质量门禁

| 门禁 | 标准 |
|------|------|
| 测试覆盖率 | ≥ 75%（当前 1,828 tests，估算需要补到 2,200+） |
| 安全扫描 | 0 个 HIGH/CRITICAL 已知漏洞（govulncheck + trivy 全绿） |
| 快速启动 | Docker compose quickstart 在全新机器上 < 5 分钟跑通 |
| 集成测试 | enterprise-saas-demo 所有 11 步通过 |

---

## 十一、需求挑战会候选分组

### 分组 1: 安全闭合（Week 1）

**参与角色**:
- Security Engineer: cross-tenant 渗透测试
- QA Engineer: 测试用例验证
- Tech Lead: 风险评估

**讨论焦点**:
- cross-tenant 攻击向量是否完整
- 测试覆盖率是否足够

### 分组 2: API 表面开放（Week 2-4）

**参与角色**:
- Backend Engineer: OpenAPI 规范编写
- Frontend Engineer: TypeScript SDK 生成
- Tech Lead: 接口设计评审

**讨论焦点**:
- OpenAPI 规范是否完整
- SDK 生成工具选择

### 分组 3: L2 接口合约（Week 4-5）

**参与角色**:
- Backend Engineer: governance client interface 定义
- Architect: 接口设计评审
- Tech Lead: 集成方案确认

**讨论焦点**:
- 接口是否满足 L2 需求
- 是否需要 proto/OpenAPI 合约

### 分组 4: Oris 集成（Week 5-6）

**参与角色**:
- Backend Engineer: 指标上报实现
- Architect: 集成方案评审
- Tech Lead: 数据格式确认

**讨论焦点**:
- 指标上报方式（OTel vs HTTP）
- 数据格式是否满足 Oris 需求

### 分组 5: 外部贡献者入口（Week 7-8）

**参与角色**:
- Tech Lead: 文档重写
- QA Engineer: Good First Issues 创建
- DevOps Engineer: 本地环境 setup

**讨论焦点**:
- CONTRIBUTING.md 是否清晰
- Good First Issues 是否真实可完成

### 分组 6: K8s 沙箱（Week 9-10）

**参与角色**:
- DevOps Engineer: K8s Job 实现
- Security Engineer: 安全评审
- Tech Lead: 方案确认

**讨论焦点**:
- Job-based vs sidecar 方案选择
- 冷启动延迟是否可接受

### 分组 7: v3.0.0 发布（Week 11-12）

**参与角色**:
- Tech Lead: 发布准备
- QA Engineer: 质量门禁验证
- DevOps Engineer: 部署验证

**讨论焦点**:
- 质量门禁是否全部通过
- Migration guide 是否完整

---

## 十二、参与角色清单

| 角色 | 职责 | 输入缺口 |
|------|------|----------|
| Tech Lead | 需求 intake、任务拆解、角色分派、冲突决策与最终交付收口 | 无 |
| Backend Engineer | OpenAPI 规范编写、SDK 生成、L2 接口合约、Oris 指标上报 | 需要确认接口设计 |
| Frontend Engineer | TypeScript SDK 生成 | 需要确认类型定义 |
| Security Engineer | cross-tenant 渗透测试、安全评审 | 需要确认测试用例 |
| QA Engineer | 测试用例验证、质量门禁验证 | 需要确认测试覆盖率 |
| DevOps Engineer | K8s 沙箱实现、本地环境 setup | 需要确认部署方案 |
| Architect | 接口设计评审、集成方案评审 | 需要确认架构决策 |

---

## 十三、时间线

```
Week 1:  cross-tenant 渗透测试
Week 2-3: OpenAPI 规范编写
Week 3-4: Go SDK + TypeScript SDK 生成
Week 4-5: L2 接口合约
Week 5-6: Oris 指标上报 + Eino Agent 集成验证
Week 7-8: 外部贡献者入口
Week 9-10: K8s 沙箱方案
Week 11-12: v3.0.0 发布准备
```

---

## 十四、下一步行动

1. **立即**: 确认三个决策点（目标用户、Oris 集成方向、SDK 语言优先级）
2. **本周**: 开始 cross-tenant 渗透测试
3. **下周**: 开始 OpenAPI 规范编写
4. **本月**: 完成 Month 1 所有 exit criteria
