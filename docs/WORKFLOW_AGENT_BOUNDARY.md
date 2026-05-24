# Workflow and Agent Runtime Boundary

HermesX has two related but different execution surfaces:

- The Agent Runtime handles open-ended conversation and tool use.
- The Workflow Engine handles fixed SOP execution through a persisted DAG.

They share parts of the Eino tool-loop implementation, but they do not have the
same governance contract.

中文速览：自由对话适合探索式任务，固定工作流适合可审计 SOP。`agent_task`
是工作流中的受治理节点，不是把整段自由聊天自动升级为流程。

---

## Boundary Summary

| Dimension | Free Agent Runtime | Fixed SOP Workflow Engine |
|-----------|--------------------|---------------------------|
| Primary API | `POST /v1/agent/chat` | `/v1/workflow-definitions`, `/v1/workflow-runs`, `/v1/workflow-tasks` |
| Shape | Conversation turn plus tool loop | Published DAG version plus step runs |
| Best for | Exploration, support, one-off agent assistance | Repeatable enterprise procedure, approvals, handoff |
| Control plane | Session, memory, tools, safety, checkpoint resume | Definition, immutable version, run, step state, retry/cancel |
| Ordering | Model-driven within runtime limits | Graph-driven by nodes and edges |
| Human approval | A tool or external policy may ask | First-class `human_task` node |
| Retry | Request/session resume semantics | Failed step retry through workflow state |
| Evidence | Audit log, messages, tool receipts | Audit log, workflow run/step state, tool receipts from agent nodes |

---

## Agent Runtime

The Agent Runtime is the open-ended agent surface. It receives user intent,
builds a prompt from tenant/user context, runs the Eino TurnLoop, calls allowed
tools, applies safety and leak scanning, and persists the successful
conversation turn.

It owns:

- Natural-language task interpretation.
- Tool selection inside the configured toolset.
- Tool-loop iteration limits.
- Safety interception and secret redaction.
- Session history, memory, and checkpoint resume.
- Tool-level execution receipts when runtime receipt recording is wired.

It does not own:

- A fixed business process graph.
- A published SOP version.
- Workflow step status or human task queues.
- Cross-step deterministic routing.

Use this surface when the user is asking a broad question or when the exact path
cannot be known before the agent starts reasoning.

---

## Workflow Engine

The Workflow Engine is the fixed SOP surface. It executes a tenant-scoped,
validated DAG made of `start`, `agent_task`, `service_task`, `human_task`, and
`end` nodes. Published versions are immutable so a running instance keeps the
same procedure even if the draft definition changes later.

It owns:

- Graph validation: one start node, end nodes, no cycles, valid conditions.
- Versioning and run state.
- Step input/output and status.
- Conditional routing from explicit workflow data.
- Human task assignment and completion.
- Retry, pause, cancel, and completion semantics.

It does not own:

- The model's internal reasoning.
- The detailed tool loop inside an `agent_task`.
- Free-form conversational memory as workflow state.

Use this surface when the organization needs the same business procedure to run
repeatedly with predictable governance and replayable state.

---

## `agent_task` as a Governed Node

`agent_task` is the bridge between the two surfaces.

The workflow owns:

- When the agent is invoked.
- Which published version and run caused the invocation.
- The prompt template and configured node limits.
- What happens after success, failure, human approval, retry, or cancel.
- The step output consumed by downstream nodes.

The agent runtime owns:

- How the prompt is interpreted.
- Which allowed tools are called inside the node.
- Tool-loop safety and iteration limits.
- Redacted final response generation.
- Tool receipts for side effects inside that node.

This means an `agent_task` can use agentic reasoning without making the whole
workflow non-deterministic. The graph remains the source of truth for process
state; the agent's final output is one step result inside that graph.

---

## Free Chat vs Fixed SOP

Choose free chat when:

- The user is exploring a problem.
- The task path is not known in advance.
- The main value is interactive assistance.
- Governance can be satisfied by auth, RBAC, safety controls, audit logs, and
  tool receipts.

Choose workflow when:

- The process must follow a known sequence.
- Human approval or segregation of duties is required.
- A failed step needs controlled retry.
- Auditors need immutable version, run, and step evidence.
- Downstream systems need structured step outputs.

Do not treat a successful free-form chat as evidence that a fixed SOP ran. If
the business question is "Was the approved procedure followed?", use a workflow
run.

---

## Governance Invariants

These invariants hold across both surfaces:

- Tenant context comes from authentication, not caller-supplied body fields.
- PostgreSQL RLS remains the data isolation boundary for tenant-scoped tables.
- RBAC decides who can call chat, workflow, audit, and receipt endpoints.
- Audit logs record API-level access and status.
- Execution receipts record tool-level side-effect evidence.
- `trace_id`, `request_id`, and `session_id` are correlation fields, not access
  control fields.

Additional workflow invariants:

- Published workflow versions are immutable.
- A run is bound to the version used at start time.
- Conversation history is not a substitute for workflow step state.
- An agent cannot rewrite the active workflow graph during a run unless an
  explicit product feature and authorization model are added.

---

## Minimal Flow

```text
API request
  -> Workflow run starts from immutable version
  -> agent_task step invokes Agent Runtime
  -> Agent Runtime may call tools
  -> Each governed tool side effect can produce an ExecutionReceipt
  -> Workflow stores step output and advances by graph rules
  -> Audit logs record API access across the path
```

For receipt fields, duplicate-call semantics, and API examples, see
[Execution Receipts](EXECUTION_RECEIPTS.md).

