# Agent-First Minimal Governance Demo

This demo shows the enterprise governance path:

```text
API -> Agent Task -> Tool -> Execution Receipt -> Audit
```

It is intentionally small and safe. Fixture mode runs without Docker, LLM
credentials, or external services. Live mode calls a local HermesX API if you
already have one running.

## Run Fixture Mode

```bash
./examples/agent-first-minimal-demo/demo.sh
```

Fixture mode prints a deterministic receipt ID, request ID, session ID, and
trace ID from local JSON fixtures.

## Run Live Mode

```bash
export HERMES_URL="http://localhost:8080"
export HERMES_CHAT_API_KEY="hk_user_or_admin_key"
export HERMES_AUDIT_API_KEY="hk_auditor_or_admin_key"

./examples/agent-first-minimal-demo/demo.sh live
```

Live mode sends one `/v1/agent/chat` request with:

- `X-Request-ID` for audit correlation.
- `X-Hermes-Session-Id` for receipt lookup.
- A prompt asking the agent to use one safe read-only tool if available.

Then it queries `/v1/execution-receipts` and `/v1/audit-logs`.

If no receipt appears in live mode, the deployed runtime may not have called a
tool, may not have receipt recording wired for that path, or the audit key may
not have permission to read receipts. Fixture mode remains the stable governance
contract demo.

## What to Look For

| Evidence | Source | Why it matters |
|----------|--------|----------------|
| `request_id` | API response and audit log | Shows the HTTP request that triggered the work |
| `session_id` | Agent response and receipt | Connects the agent turn to tool evidence |
| `receipt_id` | Execution receipt | Durable proof of the tool side effect |
| `trace_id` | Receipt and tracing header when available | Connects API, runtime, and telemetry |
| `action` | Audit log | Shows API-level access and status |

See `docs/EXECUTION_RECEIPTS.md` and `docs/WORKFLOW_AGENT_BOUNDARY.md` for the
governance model behind this demo.

