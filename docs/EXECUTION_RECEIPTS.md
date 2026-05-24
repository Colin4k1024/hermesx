# Execution Receipts

Execution receipts are HermesX's governance proof for tool side effects. An audit
log proves that an API request happened. A receipt proves that a specific tool
execution was attempted, with the tenant, session, user, tool name, bounded
input/output, status, duration, idempotency key, and trace correlation recorded.

中文速览：执行回执用于证明 Agent 工具副作用。审计日志回答"谁调用了哪个 API"，
执行回执回答"哪个工具在什么上下文中执行，结果是什么，是否可安全重试"。

---

## Why Receipts Exist

Agent conversations can branch, retry, call tools, and partially fail. Enterprise
governance needs a durable record at the side-effect boundary, not only at the
HTTP request boundary.

| Question | Audit log | Execution receipt |
|----------|-----------|-------------------|
| Who called the API? | Yes | Inherited through tenant/user fields |
| Which tool ran? | No | Yes |
| What input/output crossed the tool boundary? | No | Yes, truncated to the runtime limit |
| Did the side effect complete, fail, or time out? | Request-level only | Tool-level status |
| Can a retry duplicate the side effect? | No | Controlled by `idempotency_id` |
| Can traces connect API, tool, and telemetry? | Request ID | `trace_id` plus session/request context |

Receipts are not a full event-sourcing log. They are immutable evidence for a
tool call at the point where the agent runtime leaves pure language generation.

---

## Receipt Object

| Field | Type | Governance meaning |
|-------|------|--------------------|
| `id` | string | Unique receipt identifier returned to auditors and operators |
| `tenant_id` | string | Tenant isolation boundary |
| `session_id` | string | Agent or workflow session that caused the tool call |
| `user_id` | string | User or workflow actor that triggered execution |
| `tool_name` | string | Tool that crossed the side-effect boundary |
| `input` | string | Serialized tool input, truncated by runtime policy |
| `output` | string | Serialized result or error, truncated by runtime policy |
| `status` | string | `success`, `error`, or `timeout` |
| `duration_ms` | integer | Tool wall-clock duration |
| `idempotency_id` | string | Optional duplicate-call key scoped to the tenant |
| `trace_id` | string | Optional distributed trace correlation ID |
| `created_at` | timestamp | Persistence time |

Current PostgreSQL storage enforces tenant isolation with RLS and keeps a unique
index on `(tenant_id, idempotency_id)` when the idempotency value is non-empty.

---

## Creation and Query Contract

Receipts are created by the runtime/store path when HermesX records a tool
execution. In this worktree, the public tenant HTTP API exposes read/query
endpoints:

- `GET /v1/execution-receipts`
- `GET /v1/execution-receipts/{id}`

Do not design external integrations around `POST /v1/execution-receipts` unless
that route is present in the deployed server you are targeting. The safer
contract is: trigger a governed agent or workflow action, then query receipts by
session, tool, status, or receipt ID.

---

## API Examples

### 1. Start an Agent Task

Use a stable `X-Request-ID` and `X-Hermes-Session-Id` so the API audit log,
conversation, and receipt query can be correlated.

```bash
REQUEST_ID="demo-agent-first-001"
SESSION_ID="sess_agent_first_minimal_demo"

curl -sS -D /tmp/hermesx-agent.headers \
  -X POST "http://localhost:8080/v1/agent/chat" \
  -H "Authorization: Bearer $HERMES_CHAT_API_KEY" \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: $REQUEST_ID" \
  -H "X-Hermes-Session-Id: $SESSION_ID" \
  -d '{
    "model": "mock",
    "messages": [
      {
        "role": "user",
        "content": "Use one safe read-only tool if available, then answer in one sentence."
      }
    ],
    "include_agentic_blocks": true
  }'
```

The response headers include `X-Request-ID` and, when tracing is active,
`X-Trace-ID`.

### 2. List Receipts for the Session

```bash
curl -sS "http://localhost:8080/v1/execution-receipts?session_id=$SESSION_ID&limit=20" \
  -H "Authorization: Bearer $HERMES_AUDIT_API_KEY"
```

Example response:

```json
{
  "execution_receipts": [
    {
      "id": "rcpt_demo_001",
      "tenant_id": "tenant_demo",
      "session_id": "sess_agent_first_minimal_demo",
      "user_id": "user_demo",
      "tool_name": "read_file",
      "input": "{\"path\":\"docs/SECURITY_MODEL.md\"}",
      "output": "{\"bytes\":512,\"redacted\":false}",
      "status": "success",
      "duration_ms": 17,
      "idempotency_id": "demo-agent-first-001/read_file/1",
      "trace_id": "0af7651916cd43dd8448eb211c80319c",
      "created_at": "2026-05-24T04:00:00Z"
    }
  ],
  "total": 1
}
```

### 3. Fetch the Receipt by ID

```bash
RECEIPT_ID="rcpt_demo_001"

curl -sS "http://localhost:8080/v1/execution-receipts/$RECEIPT_ID" \
  -H "Authorization: Bearer $HERMES_AUDIT_API_KEY"
```

### 4. Correlate the API Audit Log

```bash
curl -sS "http://localhost:8080/v1/audit-logs?limit=50" \
  -H "Authorization: Bearer $HERMES_AUDIT_API_KEY"
```

Find the audit entry whose `request_id` matches the `X-Request-ID` used for the
agent call. Then use `session_id` and `trace_id` from receipts to connect the API
request, agent session, tool execution, and distributed trace.

---

## Idempotency and Duplicate Calls

`idempotency_id` is the receipt-level duplicate-call key. It is scoped by tenant,
so two tenants may reuse the same value without conflict.

Runtime semantics:

1. If `idempotency_id` is absent, every tool execution may create a separate
   receipt.
2. If `idempotency_id` is present and no prior receipt exists for the same
   tenant, the runtime executes the tool and stores the receipt.
3. If the same `(tenant_id, idempotency_id)` appears again, the runtime should
   return the prior receipt/output and skip re-execution.
4. If a retry carries different input for an existing idempotency key, the first
   receipt remains authoritative. Treat the key as naming one logical side
   effect, not a mutable request body.

Implication: clients should generate the idempotency key before attempting a
tool side effect and reuse it only for retries of that same logical operation.

---

## Workflow Relationship

Free-form `/v1/agent/chat` can produce receipts when the agent calls tools, but
the conversation itself is not a fixed SOP. A workflow `agent_task` is different:
the published workflow version owns the order, retry surface, human approvals,
and step state, while the agent runtime owns the language/tool loop inside that
single governed node.

See [Workflow and Agent Runtime Boundary](WORKFLOW_AGENT_BOUNDARY.md) for the
full boundary contract.

