# 工作流引擎使用指南

HermesX 内置固定 SOP（Standard Operating Procedure）工作流引擎，将多步骤业务流程编排为可持久化、可审计的有向无环图（DAG）。支持 Agent 任务、HTTP 服务调用、人工审批节点和条件路由，适用于审批流、多步 Agent 协作、数据管线等企业场景。

---

## 核心概念

| 概念 | 说明 |
|------|------|
| **Definition** | 工作流定义，包含名称和 Graph JSON |
| **Version** | 定义的快照版本，`publish` 后不可变 |
| **Run** | 工作流实例，基于某版本启动执行 |
| **StepRun** | 单节点执行记录，含输入/输出/状态 |
| **Graph** | DAG 结构，由 Nodes + Edges 组成 |

---

## Graph JSON Schema

```json
{
  "nodes": [
    {
      "id": "start_1",
      "type": "start",
      "name": "开始",
      "config": {}
    },
    {
      "id": "agent_review",
      "type": "agent_task",
      "name": "AI 审核",
      "config": {
        "prompt": "请审核以下申请内容：{{input.content}}",
        "model": "sonnet",
        "max_iterations": 10
      }
    },
    {
      "id": "human_approve",
      "type": "human_task",
      "name": "主管审批",
      "config": {
        "assignee": "manager",
        "instructions": "请审核 AI 审核结果，确认是否通过"
      }
    },
    {
      "id": "notify_service",
      "type": "service_task",
      "name": "发送通知",
      "config": {
        "url": "https://api.example.com/notify",
        "method": "POST",
        "headers": {"Authorization": "Bearer {{variables.api_token}}"},
        "body": {"message": "申请已通过", "applicant": "{{input.applicant}}"}
      }
    },
    {
      "id": "end_1",
      "type": "end",
      "name": "结束"
    }
  ],
  "edges": [
    {
      "from": "start_1",
      "to": "agent_review"
    },
    {
      "from": "agent_review",
      "to": "human_approve"
    },
    {
      "from": "human_approve",
      "to": "notify_service",
      "condition": {
        "outcome": "approved"
      }
    },
    {
      "from": "human_approve",
      "to": "end_1",
      "condition": {
        "outcome": "rejected"
      }
    },
    {
      "from": "notify_service",
      "to": "end_1"
    }
  ]
}
```

---

## 节点类型详解

### start

流程起点，每个 Graph 必须恰好一个。自动完成并触发下游边。

```json
{"id": "s1", "type": "start", "name": "开始"}
```

### end

流程终点，至少一个。当所有路径到达 end 节点时，实例状态变为 `completed`。

```json
{"id": "e1", "type": "end", "name": "结束"}
```

### agent_task

调用完整 Agent 循环（含 tool loop），通过 `config.prompt` 传入任务指令。

```json
{
  "id": "analyze",
  "type": "agent_task",
  "name": "数据分析",
  "config": {
    "prompt": "分析用户 {{input.user_id}} 的最近 30 天行为数据",
    "model": "sonnet",
    "max_iterations": 20
  }
}
```

**执行行为：**

- 引擎通过 `AgentExecutor` 接口调用 Agent
- 默认使用 `EinoAgentExecutor`（内置安全管线：输入拦截 + 输出脱敏 + 迭代限制）
- Agent 返回的文本写入 `stepRun.output`，供下游节点通过 `steps.{nodeID}.output` 访问
- 若 Agent 执行失败，stepRun 状态变为 `failed`，实例进入 `paused`

### service_task

HTTP 调用外部服务，响应 JSON 自动合并到步骤输出。

```json
{
  "id": "create_ticket",
  "type": "service_task",
  "name": "创建工单",
  "config": {
    "url": "https://jira.example.com/rest/api/2/issue",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json",
      "Authorization": "Bearer {{variables.jira_token}}"
    },
    "body": {
      "fields": {
        "project": {"key": "OPS"},
        "summary": "{{steps.analyze.output.title}}",
        "issuetype": {"name": "Task"}
      }
    }
  }
}
```

**执行行为：**

- 引擎调用 `HTTPExecutor` 发送请求
- HTTP 2xx → 步骤成功，响应 body 写入 output
- HTTP 4xx/5xx → 步骤失败，实例进入 `paused`（可 retry）
- 模板变量在发送前实时解析

### human_task

暂停流程等待人工操作。

```json
{
  "id": "manager_review",
  "type": "human_task",
  "name": "经理审批",
  "config": {
    "assignee": "dept_manager",
    "instructions": "请审核以下报告并决定是否通过",
    "timeout": "24h"
  }
}
```

**完成方式：**

```bash
POST /v1/workflow-tasks/{stepRunID}/complete
Content-Type: application/json

{
  "outcome": "approved",
  "output": {
    "comment": "已审核，同意执行",
    "reviewer": "张三"
  },
  "variables": {
    "approved": true,
    "approved_amount": 50000
  }
}
```

- `outcome` — 用于下游边的 outcome 匹配路由
- `output` — 写入 stepRun 输出，供下游通过 `steps.{nodeID}.output` 访问
- `variables` — 合并到实例级变量，全局可见

---

## 条件路由

### Outcome 匹配

最常见的路由方式，基于上游 human_task 的 outcome 值分支：

```json
{
  "from": "manager_review",
  "to": "execute_task",
  "condition": {"outcome": "approved"}
}
```

### 条件表达式

支持对工作流上下文进行复杂条件判断：

```json
{
  "from": "analyze",
  "to": "escalate",
  "condition": {
    "field": "input.amount",
    "op": "gt",
    "value": 100000
  }
}
```

**支持的运算符：**

| 运算符 | 说明 | 示例 |
|--------|------|------|
| `eq` | 等于 | `{"field":"input.status","op":"eq","value":"urgent"}` |
| `ne` | 不等于 | `{"field":"variables.retry_count","op":"ne","value":0}` |
| `gt` | 大于 | `{"field":"input.amount","op":"gt","value":10000}` |
| `gte` | 大于等于 | `{"field":"steps.score.output.confidence","op":"gte","value":0.8}` |
| `lt` | 小于 | `{"field":"input.priority","op":"lt","value":3}` |
| `lte` | 小于等于 | — |
| `exists` | 字段存在 | `{"field":"variables.override","op":"exists","value":true}` |
| `contains` | 包含 | `{"field":"input.tags","op":"contains","value":"urgent"}` |
| `startsWith` | 前缀匹配 | `{"field":"input.code","op":"startsWith","value":"ERR_"}` |
| `endsWith` | 后缀匹配 | `{"field":"input.email","op":"endsWith","value":"@company.com"}` |

**路径语法：**

- `input.field` — 实例启动时传入的输入数据
- `variables.key` — 实例级变量（可由 human_task 更新）
- `steps.{nodeID}.output.field` — 上游步骤的输出数据

路径支持点号分隔的嵌套访问，如 `steps.analyze.output.result.score`。

### 默认边（无条件）

没有 `condition` 字段的边为默认边，当所有带条件的边都不匹配时走默认边：

```json
{"from": "check", "to": "fallback"}
```

---

## 实例生命周期

```
                          ┌─────────────┐
                          │   running   │
                          └──────┬──────┘
                                 │
                    ┌────────────┼────────────┐
                    ▼            ▼            ▼
             ┌──────────┐ ┌──────────┐ ┌──────────┐
             │ waiting  │ │  paused  │ │completed │
             │(human审批)│ │(步骤失败) │ │          │
             └────┬─────┘ └────┬─────┘ └──────────┘
                  │            │
                  │ complete   │ retry
                  ▼            ▼
             ┌──────────┐ ┌──────────┐
             │  running  │ │  running  │
             └──────────┘ └──────────┘

             任何状态 ──cancel──→ cancelled
```

| 状态 | 说明 |
|------|------|
| `running` | 正在执行，引擎自动推进 |
| `waiting` | 遇到 human_task，等待人工完成 |
| `paused` | 步骤执行失败，等待 retry 或 cancel |
| `completed` | 所有路径到达 end 节点 |
| `cancelled` | 被手动取消 |

---

## 版本控制

```
draft ──publish──→ published ──archive──→ archived
```

- **draft** — 可编辑，不可启动实例
- **published** — 不可变，可启动实例
- **archived** — 归档，不可启动新实例，已有实例继续运行

每次 `publish` 会快照当前 Graph JSON 为新版本。运行中实例锁定在启动时的版本，定义后续变更不影响已有实例。

---

## API 完整示例

### 1. 创建工作流定义

```bash
curl -X POST http://localhost:8080/v1/workflow-definitions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "费用审批流",
    "description": "超过 1 万元的费用需要 AI 预审 + 主管审批",
    "graph": {
      "nodes": [
        {"id":"start","type":"start","name":"开始"},
        {"id":"ai_check","type":"agent_task","name":"AI 预审","config":{"prompt":"审核费用申请合规性：{{input.description}}，金额：{{input.amount}}"}},
        {"id":"approve","type":"human_task","name":"主管审批","config":{"assignee":"manager"}},
        {"id":"end_ok","type":"end","name":"通过"},
        {"id":"end_reject","type":"end","name":"拒绝"}
      ],
      "edges": [
        {"from":"start","to":"ai_check"},
        {"from":"ai_check","to":"approve"},
        {"from":"approve","to":"end_ok","condition":{"outcome":"approved"}},
        {"from":"approve","to":"end_reject","condition":{"outcome":"rejected"}}
      ]
    }
  }'
```

### 2. 发布版本

```bash
curl -X POST http://localhost:8080/v1/workflow-definitions/{def_id}/publish \
  -H "Authorization: Bearer $TOKEN"
```

### 3. 启动实例

```bash
curl -X POST http://localhost:8080/v1/workflow-runs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "definition_id": "{def_id}",
    "input": {
      "applicant": "李四",
      "amount": 25000,
      "description": "Q3 团建活动经费"
    }
  }'
```

### 4. 查询待处理人工任务

```bash
curl http://localhost:8080/v1/workflow-tasks \
  -H "Authorization: Bearer $TOKEN"
```

返回示例：

```json
[
  {
    "step_run_id": "sr_abc123",
    "run_id": "run_xyz",
    "node_id": "approve",
    "node_name": "主管审批",
    "status": "waiting",
    "created_at": "2026-05-19T10:30:00Z"
  }
]
```

### 5. 完成人工任务

```bash
curl -X POST http://localhost:8080/v1/workflow-tasks/sr_abc123/complete \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "outcome": "approved",
    "output": {"comment": "金额合理，同意"},
    "variables": {"final_amount": 25000}
  }'
```

### 6. 查询实例状态

```bash
curl http://localhost:8080/v1/workflow-runs/{run_id} \
  -H "Authorization: Bearer $TOKEN"
```

### 7. 重试失败步骤

```bash
curl -X POST http://localhost:8080/v1/workflow-runs/{run_id}/retry \
  -H "Authorization: Bearer $TOKEN"
```

### 8. 取消实例

```bash
curl -X POST http://localhost:8080/v1/workflow-runs/{run_id}/cancel \
  -H "Authorization: Bearer $TOKEN"
```

---

## Graph 校验规则

引擎在定义创建和发布时自动校验 Graph 合法性：

| 规则 | 说明 |
|------|------|
| 恰好 1 个 start 节点 | 不允许多入口或无入口 |
| 至少 1 个 end 节点 | 必须有终止点 |
| 节点类型合法 | 仅允许 `start`/`end`/`human_task`/`service_task`/`agent_task` |
| 无自环 | 边的 from ≠ to |
| 无环（DAG） | 拓扑排序检测，确保流程可终止 |
| 全连通 | 从 start 节点出发必须可达所有节点 |
| 条件运算符合法 | 仅允许已注册的运算符 |

---

## 安全与隔离

### 多租户隔离

- 工作流定义、版本、实例均绑定 `tenant_id`
- PostgreSQL RLS 确保租户间数据不可见
- API 层通过 middleware 自动注入租户上下文

### Agent 安全管线（EinoAgentExecutor）

`agent_task` 节点通过 `EinoAgentExecutor` 执行，构造函数强制要求安全参数：

```go
executor := workflow.NewEinoAgentExecutor(
    transport,       // LLM 传输层
    toolEntries,     // 可用工具集
    interceptor,     // SafetyInterceptor（输入拦截 + 输出检查）
    scanner,         // LeakScanner（凭据脱敏）
)
```

安全保障：

- **输入拦截** — Prompt Injection 检测，阻断恶意输入
- **输出检查** — 检测 Agent 输出中的不安全内容
- **凭据脱敏** — 自动识别并遮蔽 AWS Key、GitHub Token 等 20+ 种凭据模式
- **迭代限制** — 硬上限 50 次 tool loop，防止无限循环
- **流式脱敏** — chunk 级缓冲脱敏，中间 chunk 不泄漏原始凭据

### 审计

所有工作流操作（创建、发布、启动、完成、取消）均记录到审计日志，含操作人、时间、租户和变更内容。

---

## 典型场景

### 场景 1：多级审批

```
start → AI 风控预审 → 部门主管审批 → (金额>10万) → 总监审批 → end
                                    → (金额≤10万) → end
```

### 场景 2：Agent 协作管线

```
start → 数据采集 Agent → 分析 Agent → 报告生成 Agent → 人工确认 → 发送通知服务 → end
```

### 场景 3：故障处理 SOP

```
start → 故障诊断 Agent → 自动修复 Agent → (修复成功) → 通知服务 → end
                                        → (修复失败) → 人工介入 → end
```

---

## 配置参考

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `DATABASE_URL` | PostgreSQL 连接（工作流表存储） | 必填 |
| `WORKFLOW_MAX_STEPS` | 单实例最大步骤数 | 50 |
| `WORKFLOW_STEP_TIMEOUT` | 单步骤超时 | 5m |
| `WORKFLOW_HTTP_TIMEOUT` | service_task HTTP 超时 | 30s |

### 存储后端

PostgreSQL（推荐）和 MySQL 双实现，表结构自动迁移：

- `workflow_definitions` — 工作流定义
- `workflow_versions` — 版本快照
- `workflow_runs` — 实例记录
- `workflow_step_runs` — 步骤执行记录

---

## 与 Eino Agent Runtime 集成

工作流引擎的 `agent_task` 节点默认通过 `EinoAgentExecutor` 执行，利用 Eino ReAct Graph 提供：

- **完整 tool loop** — 多轮工具调用直到任务完成
- **安全管线** — 输入拦截 → Agent 执行 → 输出脱敏，全链路保护
- **上下文传递** — 工作流变量通过 context 注入 Agent，Agent 输出回写工作流

架构层次：

```
Workflow Engine
  └── AgentExecutor (interface)
        └── EinoAgentExecutor
              ├── EinoAgent (ReAct Graph)
              │     ├── ModelAdapter (llm.Transport → Eino ChatModel)
              │     └── ToolAdapter (ToolEntry → Eino InvokableTool)
              ├── SafetyInterceptor (输入/输出检查)
              └── LeakScanner (凭据脱敏)
```
