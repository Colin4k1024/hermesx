# HermesX

[English](#english) | [中文](#中文)

---

<a id="english"></a>

## English

**HermesX is an Agent-first Runtime Control Plane for governed, multi-tenant AI automation.**

It is for platform and product teams that need to run agents as production infrastructure: every agent turn, tool call, workflow step, tenant boundary, and operational signal has to be controlled, audited, and recoverable.

HermesX is not a local standalone assistant. Its supported product surface is the SaaS API service, which combines an agent runtime, a multi-tenant control plane, embedded WebUI, and fixed SOP workflows so teams can ship agentic systems without rebuilding identity, tenancy, audit, sandboxing, and observability from scratch.

### Release State

| Field | Value |
|-------|-------|
| Current docs/API baseline | `v2.4.0-dev` |
| Latest released baseline | `v2.3.0` |
| OpenAPI info.version | `2.4.0-dev` |
| Release-state rule | Features marked Unreleased are present in the current branch or changelog, but are not part of the latest released baseline until a `v2.4.0` release is cut. |

### Who It Is For

| Audience | Why HermesX fits |
|----------|------------------|
| Platform teams | Provide a shared runtime for internal agents with tenant isolation, API keys, RBAC, audit logs, and usage controls. |
| Product teams | Add AI workflows, human approvals, and tool execution to SaaS products without making every feature team own agent infrastructure. |
| Security and operations teams | Review execution receipts, sandbox policy, auth chains, audit trails, metrics, and disaster-recovery posture in one place. |

### Why HermesX

1. **Governed Agent Execution**: agents can call tools, use memory, delegate work, and stream responses while staying inside auth, policy, sandbox, and audit boundaries.
2. **Multi-Tenant SaaS Control Plane**: tenants, roles, API keys, quotas, usage records, audit logs, GDPR actions, and admin operations are first-class runtime objects.
3. **Workflow + Human-in-the-Loop Automation**: fixed SOP workflows persist definitions, immutable versions, runs, step state, retries, and human approval tasks.

### Why HermesX Over Alternatives

| Feature | HermesX | Dify | CrewAI | LangGraph |
|---------|---------|------|--------|-----------|
| Runtime | Go (single binary) | Python | Python | Python |
| Multi-tenant | PG RLS database-level isolation | None | None | None |
| Tool audit | ExecutionReceipt | None | None | None |
| Self-hosting | Native | Supported but complex | Not suitable | Not suitable |
| LLM resilience | Three-layer circuit breaker | Basic retry | None | None |
| Enterprise ready | RBAC, audit logs, GDPR | Limited | None | None |

HermesX is designed for teams that need to run agents as production infrastructure with enterprise-grade governance, not just for prototyping or local development.

### Architecture

[![HermesX technical architecture](docs/diagrams/technical-architecture.png)](docs/diagrams/technical-architecture.drawio)

Detailed one-page overview: [docs/AGENT_FIRST_ARCHITECTURE.md](docs/AGENT_FIRST_ARCHITECTURE.md).

Architecture diagrams are maintained as draw.io source files in [docs/diagrams/](docs/diagrams/):

| Diagram | Preview | Editable source |
|---------|---------|-----------------|
| Technical architecture | [PNG](docs/diagrams/technical-architecture.png) | [draw.io](docs/diagrams/technical-architecture.drawio) |
| Product architecture | [PNG](docs/diagrams/product-architecture.png) | [draw.io](docs/diagrams/product-architecture.drawio) |
| Application architecture | [PNG](docs/diagrams/application-architecture.png) | [draw.io](docs/diagrams/application-architecture.drawio) |
| Data architecture | [PNG](docs/diagrams/data-architecture.png) | [draw.io](docs/diagrams/data-architecture.drawio) |

### Minimal Demo

#### SaaS API Service

```bash
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx
docker compose -f docker-compose.prod.yml up -d

curl http://localhost:8080/health/ready
./examples/enterprise-saas-demo/demo.sh
```

#### Agent-first Governance Loop

```bash
./examples/agent-first-minimal-demo/demo.sh fixture
```

This deterministic fixture demo shows the API -> Agent Task -> Tool -> Receipt -> Audit correlation without requiring external services.

### Capability Matrix

| Capability | Latest released baseline (`v2.3.0`) | Current branch (`v2.4.0-dev`) |
|------------|--------------------------------------|--------------------------------|
| Agent runtime | OpenAI-compatible chat, native agent chat, tools, skills, memory, MCP client, context compression | Eino 0.9 main path, checkpoint resume, `include_agentic_blocks` debug output |
| SaaS control plane | Tenant isolation, PostgreSQL RLS, auth chain, API key scopes, RBAC, audit logs, GDPR export/delete, execution receipts | Admin usage aggregation API |
| Workflow automation | Fixed SOP workflow definitions, immutable versions, runs, step records, human tasks, retry/cancel API | Workflow `agent_task` default executor uses the Eino TurnLoop path |
| Sandbox and execution | Tenant-level sandbox controls with explicit Docker/K8s backends | SaaS-only default rejects implicit host execution; production uses `SANDBOX_MODE=k8s-job` |
| Observability and ops | Prometheus metrics, OpenTelemetry tracing, structured logs, production compose, PG backup/restore | Grafana dashboard, Prometheus alert rules, OTel collector compose, Redis/MinIO backup scripts |
| Distributed scheduling | SaaS cron scheduler with Redis lock, PG poll-sync, idempotent runs, result delivery | Release hardening and follow-up docs tracked in Unreleased |

### Project Signals

| Metric | Current value |
|--------|---------------|
| Go source files | 532 |
| Go test files | 156 |
| Bundled skills | 81 core + 45 optional |
| OpenAPI paths | 45 |
| Current docs/API baseline | `v2.4.0-dev` |
| Latest released baseline | `v2.3.0` |

Counts are intentionally small and evidence-oriented. The full API contract is available from `GET /v1/openapi`.

### Documentation

| Document | Purpose |
|----------|---------|
| [Agent-first architecture](docs/AGENT_FIRST_ARCHITECTURE.md) | Product positioning and layer boundaries |
| [API reference](docs/api-reference.en.md) | Endpoint-level API documentation |
| [Workflow guide](docs/workflow-guide.en.md) | Fixed SOP workflows and human tasks |
| [Execution receipts](docs/EXECUTION_RECEIPTS.md) | Receipt semantics, idempotency, and API examples |
| [Workflow/Agent boundary](docs/WORKFLOW_AGENT_BOUNDARY.md) | Where fixed SOP workflow logic ends and agent runtime logic begins |
| [Security model](docs/SECURITY_MODEL.en.md) | Threat model, auth chain, RLS, sandboxing |
| [RBAC matrix](docs/RBAC_MATRIX.en.md) | Role and resource permission matrix |
| [Enterprise readiness](docs/ENTERPRISE_READINESS.en.md) | Evidence-based enterprise readiness matrix |
| [Deployment guide](docs/deployment.en.md) | Docker, Kubernetes, HA, backup, alerting |
| [Changelog](docs/CHANGELOG.en.md) | Released vs unreleased change history |
| [Contributing](CONTRIBUTING.md) | Contribution workflow, checks, and project structure |
| [Security policy](SECURITY.md) | Private vulnerability reporting and supported versions |
| [Support](SUPPORT.md) | Where to ask for setup help, bugs, and feature requests |
| [Governance](GOVERNANCE.md) | Lightweight maintainer-led project governance |
| [Roadmap](ROADMAP.md) | Near-term direction and non-goals |

### When To Use HermesX

Use HermesX when agents must run as a governed SaaS service inside product-grade boundaries: multiple tenants, real users, sensitive tools, auditable execution, approval workflows, and operational ownership.

HermesX no longer exposes local standalone assistant or gateway deployment paths. For a single local assistant, a pure prompt prototype, or a workflow that does not need tenant isolation or auditability, use a smaller agent framework instead.

### Acknowledgements

HermesX was originally forked from [hermes-agent](https://github.com/NousResearch/hermes-agent) by [Nous Research](https://nousresearch.com). HermesX has since diverged into an independent runtime-control-plane project for enterprise agent systems.

### License

MIT

---

<a id="中文"></a>

## 中文

**HermesX 是面向 Agent 的运行时控制平面，用于受治理、多租户的 AI 自动化。**

它面向需要把 Agent 当作生产基础设施运行的平台团队和产品团队：每一次 Agent 对话、工具调用、工作流步骤、租户边界和运维信号都需要可控制、可审计、可恢复。

HermesX 不是本地单机助手。它对外支持的产品形态是 SaaS API 服务，把 Agent Runtime、多租户控制平面、内嵌 WebUI 和固定 SOP 工作流组合在一起，让团队不必从零重建身份认证、租户隔离、审计、沙箱和可观测性。

### 发布状态

| 字段 | 值 |
|------|----|
| 当前文档/API 基线 | `v2.4.0-dev` |
| 最新已发布基线 | `v2.3.0` |
| OpenAPI info.version | `2.4.0-dev` |
| 发布状态规则 | 标记为 Unreleased 的能力存在于当前分支或 changelog 中，但在 `v2.4.0` 正式发布前不属于最新稳定发布。 |

### 适用对象

| 对象 | 为什么适合 HermesX |
|------|--------------------|
| 平台团队 | 为内部 Agent 提供统一运行时，并内置租户隔离、API Key、RBAC、审计日志和用量控制。 |
| 产品团队 | 在 SaaS 产品中加入 AI 工作流、人工审批和工具执行，而不让每个业务团队都维护 Agent 基础设施。 |
| 安全与运维团队 | 在一个控制面中审查执行回执、沙箱策略、认证链、审计轨迹、指标和灾备状态。 |

### 三个支柱

1. **受治理的 Agent 执行**：Agent 可以调用工具、使用记忆、委派任务和流式响应，同时受认证、策略、沙箱和审计约束。
2. **多租户 SaaS 控制平面**：租户、角色、API Key、配额、用量、审计、GDPR 操作和管理端能力都是一等运行时对象。
3. **工作流 + 人在回路自动化**：固定 SOP 工作流持久化定义、不可变版本、实例、步骤状态、重试和人工审批任务。

### 架构

[![HermesX 技术架构图](docs/diagrams/technical-architecture.png)](docs/diagrams/technical-architecture.drawio)

一页架构说明见 [docs/AGENT_FIRST_ARCHITECTURE.md](docs/AGENT_FIRST_ARCHITECTURE.md)。

架构图统一以 draw.io 源文件维护，位于 [docs/diagrams/](docs/diagrams/)：

| 图 | 预览图 | 可编辑源文件 |
|----|--------|--------------|
| 技术架构 | [PNG](docs/diagrams/technical-architecture.png) | [draw.io](docs/diagrams/technical-architecture.drawio) |
| 产品架构 | [PNG](docs/diagrams/product-architecture.png) | [draw.io](docs/diagrams/product-architecture.drawio) |
| 应用架构 | [PNG](docs/diagrams/application-architecture.png) | [draw.io](docs/diagrams/application-architecture.drawio) |
| 数据架构 | [PNG](docs/diagrams/data-architecture.png) | [draw.io](docs/diagrams/data-architecture.drawio) |

### 最小演示

#### SaaS API 服务

```bash
git clone https://github.com/Colin4k1024/hermesx.git
cd hermesx
docker compose -f docker-compose.prod.yml up -d

curl http://localhost:8080/health/ready
./examples/enterprise-saas-demo/demo.sh
```

#### Agent-first 治理闭环

```bash
./examples/agent-first-minimal-demo/demo.sh fixture
```

这个确定性的 fixture demo 展示 API -> Agent Task -> Tool -> Receipt -> Audit 的关联链路，不依赖外部服务。

### 能力矩阵

| 能力 | 最新已发布基线（`v2.3.0`） | 当前分支（`v2.4.0-dev`） |
|------|-----------------------------|---------------------------|
| Agent Runtime | OpenAI 兼容 Chat、原生 Agent Chat、工具、技能、记忆、MCP、上下文压缩 | Eino 0.9 主链、checkpoint resume、`include_agentic_blocks` 调试输出 |
| SaaS 控制平面 | 租户隔离、PostgreSQL RLS、认证链、API Key Scope、RBAC、审计、GDPR、执行回执 | Admin usage aggregation API |
| 工作流自动化 | 固定 SOP 工作流定义、不可变版本、实例、步骤记录、人工任务、重试/取消 API | workflow `agent_task` 默认走 Eino TurnLoop |
| 沙箱与执行 | 租户级沙箱控制，显式 Docker/K8s 后端 | SaaS-only 默认拒绝隐式宿主机执行；生产使用 `SANDBOX_MODE=k8s-job` |
| 可观测与运维 | Prometheus 指标、OpenTelemetry 链路、结构化日志、生产 compose、PG 备份/恢复 | Grafana Dashboard、Prometheus 告警、OTel Collector compose、Redis/MinIO 备份脚本 |
| 分布式调度 | Redis Lock、PG 同步、幂等运行、结果投递的 SaaS cron scheduler | 未发布区跟踪发布加固和后续文档 |

### 项目信号

| 指标 | 当前值 |
|------|--------|
| Go 源文件 | 532 |
| Go 测试文件 | 156 |
| 内置技能 | 81 core + 45 optional |
| OpenAPI 路径 | 45 |
| 当前文档/API 基线 | `v2.4.0-dev` |
| 最新已发布基线 | `v2.3.0` |

这些数字只保留能帮助判断项目规模和契约状态的信号。完整 API 契约以 `GET /v1/openapi` 为准。

### 文档

| 文档 | 用途 |
|------|------|
| [Agent-first architecture](docs/AGENT_FIRST_ARCHITECTURE.md) | 产品定位与层边界 |
| [API 参考](docs/api-reference.md) | API 端点说明 |
| [工作流指南](docs/workflow-guide.md) | 固定 SOP 工作流与人工任务 |
| [执行回执](docs/EXECUTION_RECEIPTS.md) | 回执语义、幂等行为与 API 示例 |
| [工作流/Agent 边界](docs/WORKFLOW_AGENT_BOUNDARY.md) | 固定 SOP 工作流逻辑与 Agent Runtime 逻辑的边界 |
| [安全模型](docs/SECURITY_MODEL.md) | 威胁模型、认证链、RLS、沙箱 |
| [RBAC 矩阵](docs/RBAC_MATRIX.md) | 角色与资源权限 |
| [企业就绪度](docs/ENTERPRISE_READINESS.md) | 基于证据的企业能力矩阵 |
| [部署指南](docs/deployment.md) | Docker、Kubernetes、高可用、备份、告警 |
| [Changelog](docs/CHANGELOG.md) | 已发布与未发布变更 |
| [贡献指南](CONTRIBUTING.md) | 贡献流程、检查项和项目结构 |
| [安全政策](SECURITY.md) | 私密漏洞上报和支持版本 |
| [支持入口](SUPPORT.md) | 安装帮助、缺陷报告和功能请求入口 |
| [治理说明](GOVERNANCE.md) | 轻量维护者主导治理模型 |
| [路线图](ROADMAP.md) | 近期方向和非目标 |

### 何时使用 HermesX

当 Agent 需要作为受治理 SaaS 服务进入真实产品边界时使用 HermesX：多租户、真实用户、敏感工具、可审计执行、审批工作流和运维责任。

HermesX 不再对外提供本地单机助手或 Gateway 部署路径。如果只是本地助手、提示词原型，或不需要租户隔离和审计能力的简单编排，选择更小的 Agent 框架会更直接。

### 致谢

HermesX 最初 fork 自 [Nous Research](https://nousresearch.com) 的 [hermes-agent](https://github.com/NousResearch/hermes-agent)。HermesX 之后已经演进为独立的企业 Agent 运行时控制平面项目。

### 许可证

MIT
