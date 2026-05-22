# Workflow Engine Guide

HermesX includes a built-in Fixed SOP (Standard Operating Procedure) workflow engine that orchestrates multi-step business processes as persistent, auditable directed acyclic graphs (DAGs). Supports agent tasks, HTTP service calls, human approval nodes, and conditional routing — suitable for approval flows, multi-agent collaboration, data pipelines, and other enterprise scenarios.

---

## Core Concepts

| Concept | Description |
|---------|-------------|
| **Definition** | Workflow definition containing name and Graph JSON |
| **Version** | Immutable snapshot of a definition after `publish` |
| **Run** | Workflow instance, launched from a specific version |
| **StepRun** | Single node execution record with input/output/status |
| **Graph** | DAG structure composed of Nodes + Edges |

---

## Graph JSON Schema

```json
{
  "nodes": [
    {
      "id": "start_1",
      "type": "start",
      "name": "Start",
      "config": {}
    },
    {
      "id": "agent_review",
      "type": "agent_task",
      "name": "AI Review",
      "config": {
        "prompt": "Please review the following application: {{input.content}}",
        "model": "sonnet",
        "max_iterations": 10
      }
    },
    {
      "id": "human_approve",
      "type": "human_task",
      "name": "Manager Approval",
      "config": {
        "assignee": "manager",
        "instructions": "Review the AI assessment and decide whether to approve"
      }
    },
    {
      "id": "notify_service",
      "type": "service_task",
      "name": "Send Notification",
      "config": {
        "url": "https://api.example.com/notify",
        "method": "POST",
        "headers": {"Authorization": "Bearer {{variables.api_token}}"},
        "body": {"message": "Application approved", "applicant": "{{input.applicant}}"}
      }
    },
    {
      "id": "end_1",
      "type": "end",
      "name": "End"
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

## Node Types

### start

Flow entry point. Exactly one per graph. Auto-completes and triggers downstream edges.

```json
{"id": "s1", "type": "start", "name": "Start"}
```

### end

Flow exit point. At least one required. Instance transitions to `completed` when all paths reach an end node.

```json
{"id": "e1", "type": "end", "name": "End"}
```

### agent_task

Invokes a full agent loop (including tool calls) via `config.prompt`.

```json
{
  "id": "analyze",
  "type": "agent_task",
  "name": "Data Analysis",
  "config": {
    "prompt": "Analyze the last 30 days of behavior data for user {{input.user_id}}",
    "model": "sonnet",
    "max_iterations": 20
  }
}
```

**Execution behavior:**

- Engine delegates to `AgentExecutor` interface
- Default implementation is `EinoAgentExecutor` (built-in safety pipeline: input interception + output redaction + iteration limits)
- Agent's text response writes to `stepRun.output`, accessible downstream via `steps.{nodeID}.output`
- If agent execution fails, stepRun status becomes `failed`, instance enters `paused`

### service_task

HTTP call to external services. JSON response auto-merges into step output.

```json
{
  "id": "create_ticket",
  "type": "service_task",
  "name": "Create Ticket",
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

**Execution behavior:**

- Engine calls `HTTPExecutor` to send the request
- HTTP 2xx → step succeeds, response body written to output
- HTTP 4xx/5xx → step fails, instance enters `paused` (retryable)
- Template variables resolved at send time

### human_task

Pauses the flow awaiting human action.

```json
{
  "id": "manager_review",
  "type": "human_task",
  "name": "Manager Review",
  "config": {
    "assignee": "dept_manager",
    "instructions": "Please review the report and decide whether to proceed",
    "timeout": "24h"
  }
}
```

**Completion:**

```bash
POST /v1/workflow-tasks/{stepRunID}/complete
Content-Type: application/json

{
  "outcome": "approved",
  "output": {
    "comment": "Reviewed and approved",
    "reviewer": "John"
  },
  "variables": {
    "approved": true,
    "approved_amount": 50000
  }
}
```

- `outcome` — used for downstream edge outcome matching
- `output` — written to stepRun output, accessible downstream via `steps.{nodeID}.output`
- `variables` — merged into instance-level variables, globally visible

---

## Conditional Routing

### Outcome Matching

The most common routing method, branching based on upstream human_task outcome:

```json
{
  "from": "manager_review",
  "to": "execute_task",
  "condition": {"outcome": "approved"}
}
```

### Condition Expressions

Complex condition evaluation against workflow context:

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

**Supported operators:**

| Operator | Description | Example |
|----------|-------------|---------|
| `eq` | Equals | `{"field":"input.status","op":"eq","value":"urgent"}` |
| `ne` | Not equals | `{"field":"variables.retry_count","op":"ne","value":0}` |
| `gt` | Greater than | `{"field":"input.amount","op":"gt","value":10000}` |
| `gte` | Greater than or equal | `{"field":"steps.score.output.confidence","op":"gte","value":0.8}` |
| `lt` | Less than | `{"field":"input.priority","op":"lt","value":3}` |
| `lte` | Less than or equal | — |
| `exists` | Field exists | `{"field":"variables.override","op":"exists","value":true}` |
| `contains` | Contains | `{"field":"input.tags","op":"contains","value":"urgent"}` |
| `startsWith` | Prefix match | `{"field":"input.code","op":"startsWith","value":"ERR_"}` |
| `endsWith` | Suffix match | `{"field":"input.email","op":"endsWith","value":"@company.com"}` |

**Path syntax:**

- `input.field` — input data passed when starting the instance
- `variables.key` — instance-level variables (updatable by human_task)
- `steps.{nodeID}.output.field` — output data from upstream steps

Paths support dot-separated nested access, e.g., `steps.analyze.output.result.score`.

### Default Edge (unconditional)

Edges without a `condition` field are default edges — taken when all conditional edges fail to match:

```json
{"from": "check", "to": "fallback"}
```

---

## Instance Lifecycle

```
                          ┌─────────────┐
                          │   running   │
                          └──────┬──────┘
                                 │
                    ┌────────────┼────────────┐
                    ▼            ▼            ▼
             ┌──────────┐ ┌──────────┐ ┌──────────┐
             │ waiting  │ │  paused  │ │completed │
             │(human)   │ │(failed)  │ │          │
             └────┬─────┘ └────┬─────┘ └──────────┘
                  │            │
                  │ complete   │ retry
                  ▼            ▼
             ┌──────────┐ ┌──────────┐
             │  running  │ │  running  │
             └──────────┘ └──────────┘

             Any state ──cancel──→ cancelled
```

| Status | Description |
|--------|-------------|
| `running` | Actively executing, engine auto-advances |
| `waiting` | Encountered human_task, awaiting completion |
| `paused` | Step failed, awaiting retry or cancel |
| `completed` | All paths reached end nodes |
| `cancelled` | Manually cancelled |

---

## Version Control

```
draft ──publish──→ published ──archive──→ archived
```

- **draft** — editable, cannot start instances
- **published** — immutable, can start instances
- **archived** — archived, cannot start new instances; existing instances continue running

Each `publish` snapshots the current Graph JSON as a new version. Running instances are pinned to the version at launch time — subsequent definition changes never affect them.

---

## Full API Examples

### 1. Create Workflow Definition

```bash
curl -X POST http://localhost:8080/v1/workflow-definitions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Expense Approval",
    "description": "Expenses over $10k require AI pre-review + manager approval",
    "graph": {
      "nodes": [
        {"id":"start","type":"start","name":"Start"},
        {"id":"ai_check","type":"agent_task","name":"AI Review","config":{"prompt":"Review expense compliance: {{input.description}}, amount: {{input.amount}}"}},
        {"id":"approve","type":"human_task","name":"Manager Approval","config":{"assignee":"manager"}},
        {"id":"end_ok","type":"end","name":"Approved"},
        {"id":"end_reject","type":"end","name":"Rejected"}
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

### 2. Publish Version

```bash
curl -X POST http://localhost:8080/v1/workflow-definitions/{def_id}/publish \
  -H "Authorization: Bearer $TOKEN"
```

### 3. Start Instance

```bash
curl -X POST http://localhost:8080/v1/workflow-runs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "definition_id": "{def_id}",
    "input": {
      "applicant": "Alice",
      "amount": 25000,
      "description": "Q3 team building budget"
    }
  }'
```

### 4. List Pending Human Tasks

```bash
curl http://localhost:8080/v1/workflow-tasks \
  -H "Authorization: Bearer $TOKEN"
```

Response example:

```json
[
  {
    "step_run_id": "sr_abc123",
    "run_id": "run_xyz",
    "node_id": "approve",
    "node_name": "Manager Approval",
    "status": "waiting",
    "created_at": "2026-05-19T10:30:00Z"
  }
]
```

### 5. Complete Human Task

```bash
curl -X POST http://localhost:8080/v1/workflow-tasks/sr_abc123/complete \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "outcome": "approved",
    "output": {"comment": "Amount is reasonable, approved"},
    "variables": {"final_amount": 25000}
  }'
```

### 6. Check Instance Status

```bash
curl http://localhost:8080/v1/workflow-runs/{run_id} \
  -H "Authorization: Bearer $TOKEN"
```

### 7. Retry Failed Step

```bash
curl -X POST http://localhost:8080/v1/workflow-runs/{run_id}/retry \
  -H "Authorization: Bearer $TOKEN"
```

### 8. Cancel Instance

```bash
curl -X POST http://localhost:8080/v1/workflow-runs/{run_id}/cancel \
  -H "Authorization: Bearer $TOKEN"
```

---

## Graph Validation Rules

The engine automatically validates graph correctness when creating and publishing definitions:

| Rule | Description |
|------|-------------|
| Exactly 1 start node | No multiple or missing entry points |
| At least 1 end node | Must have a termination point |
| Valid node types | Only `start`/`end`/`human_task`/`service_task`/`agent_task` |
| No self-loops | Edge from ≠ to |
| Acyclic (DAG) | Topological sort detection ensures the flow can terminate |
| Fully connected | All nodes must be reachable from start |
| Valid condition operators | Only registered operators allowed |

---

## Security & Isolation

### Multi-Tenant Isolation

- Workflow definitions, versions, and instances are all bound to `tenant_id`
- PostgreSQL RLS ensures cross-tenant data invisibility
- API layer auto-injects tenant context via middleware

### Agent Safety Pipeline (EinoAgentExecutor)

`agent_task` nodes execute through `EinoAgentExecutor`, whose constructor mandates safety parameters:

```go
executor := workflow.NewEinoAgentExecutor(
    transport,       // LLM transport layer
    toolEntries,     // available tool set
    interceptor,     // SafetyInterceptor (input interception + output checking)
    scanner,         // LeakScanner (credential redaction)
)
```

Safety guarantees:

- **Input interception** — prompt injection detection, blocks malicious input
- **Output checking** — detects unsafe content in agent output
- **Credential redaction** — auto-identifies and masks AWS Keys, GitHub Tokens, and 20+ credential patterns
- **Iteration limit** — hard cap of 50 tool loop iterations, prevents infinite loops
- **Stream redaction** — chunk-level buffered redaction, intermediate chunks never leak raw credentials
- **TurnLoop main path** — workflow `agent_task` and `/v1/agent/chat` share Eino TurnLoop execution semantics; workflow recovery is persisted through step retry, while API conversations resume interrupted requests through the checkpoint store

### Audit

All workflow operations (create, publish, start, complete, cancel) are recorded in the audit log with operator, timestamp, tenant, and change details.

---

## Common Scenarios

### Scenario 1: Multi-Level Approval

```
start → AI risk review → Dept manager approval → (amount > 100k) → Director approval → end
                                                → (amount ≤ 100k) → end
```

### Scenario 2: Agent Collaboration Pipeline

```
start → Data Collection Agent → Analysis Agent → Report Generation Agent → Human Review → Notification Service → end
```

### Scenario 3: Incident Response SOP

```
start → Diagnosis Agent → Auto-Fix Agent → (fix successful) → Notification Service → end
                                          → (fix failed) → Human Intervention → end
```

---

## Configuration Reference

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection (workflow table storage) | Required |
| `WORKFLOW_MAX_STEPS` | Maximum steps per instance | 50 |
| `WORKFLOW_STEP_TIMEOUT` | Per-step timeout | 5m |
| `WORKFLOW_HTTP_TIMEOUT` | service_task HTTP timeout | 30s |

### Storage Backends

PostgreSQL (recommended) and MySQL dual implementation with auto-migration:

- `workflow_definitions` — workflow definitions
- `workflow_versions` — version snapshots
- `workflow_runs` — instance records
- `workflow_step_runs` — step execution records

---

## Eino Agent Runtime Integration

The workflow engine's `agent_task` nodes execute by default through `EinoAgentExecutor`, leveraging the Eino ReAct Graph to provide:

- **Full tool loop** — multi-round tool calls until task completion
- **Safety pipeline** — input interception → agent execution → output redaction, full-chain protection
- **Context passing** — workflow variables injected into agent via context, agent output written back to workflow
- **Unified main path** — shares `RunConversationTurnLoopSafe` with `/v1/agent/chat`, reducing behavior drift between API and workflow execution

Architecture layers:

```
Workflow Engine
  └── AgentExecutor (interface)
        └── EinoAgentExecutor
              ├── EinoAgent (ReAct Graph)
              │     ├── ModelAdapter (llm.Transport → Eino ChatModel)
              │     └── ToolAdapter (ToolEntry → Eino InvokableTool)
              ├── SafetyInterceptor (input/output checking)
              └── LeakScanner (credential redaction)
```
