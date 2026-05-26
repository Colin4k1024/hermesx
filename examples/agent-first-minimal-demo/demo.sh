#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-fixture}"
BASE_URL="${HERMES_URL:-http://localhost:8080}"
CHAT_API_KEY="${HERMES_CHAT_API_KEY:-${HERMES_API_KEY:-}}"
AUDIT_API_KEY="${HERMES_AUDIT_API_KEY:-${HERMES_API_KEY:-}}"
SESSION_ID="${HERMES_SESSION_ID:-sess_agent_first_minimal_demo}"
REQUEST_ID="${HERMES_REQUEST_ID:-demo-agent-first-001}"
LIVE_TMPDIR=""

DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURE_DIR="${DEMO_DIR}/fixtures"

need_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    echo "jq is required for this demo." >&2
    exit 1
  fi
}

section() {
  printf "\n== %s ==\n" "$1"
}

fixture_mode() {
  need_jq

  local agent_json="${FIXTURE_DIR}/agent-chat-response.json"
  local receipts_json="${FIXTURE_DIR}/execution-receipts.json"
  local audit_json="${FIXTURE_DIR}/audit-logs.json"

  section "1. API -> Agent Task"
  jq -r '"session_id=" + .id + "\nrequest_id=" + .headers["X-Request-ID"] + "\ntrace_id=" + .headers["X-Trace-ID"]' "$agent_json"

  section "2. Agent Task -> Tool -> Receipt"
  jq -r '.execution_receipts[0] | "receipt_id=" + .id + "\ntool_name=" + .tool_name + "\nidempotency_id=" + .idempotency_id + "\nreceipt_trace_id=" + .trace_id' "$receipts_json"

  section "3. Receipt -> Audit Correlation"
  jq -r --arg request_id "$REQUEST_ID" '.audit_logs[] | select(.request_id == $request_id) | "audit_action=" + .action + "\naudit_request_id=" + .request_id + "\naudit_status=" + (.status_code | tostring)' "$audit_json"

  section "Result"
  echo "Fixture demo complete: request, receipt, trace, and audit evidence correlate locally."
}

live_mode() {
  need_jq

  if [ -z "$CHAT_API_KEY" ]; then
    echo "Set HERMES_CHAT_API_KEY or HERMES_API_KEY for live mode." >&2
    exit 1
  fi
  if [ -z "$AUDIT_API_KEY" ]; then
    echo "Set HERMES_AUDIT_API_KEY or HERMES_API_KEY for live mode." >&2
    exit 1
  fi

  LIVE_TMPDIR="$(mktemp -d)"
  trap 'rm -rf "$LIVE_TMPDIR"' EXIT

  section "1. API -> Agent Task"
  local status
  status="$(curl -sS -o "${LIVE_TMPDIR}/agent.json" -D "${LIVE_TMPDIR}/headers" -w "%{http_code}" \
    -X POST "${BASE_URL}/v1/agent/chat" \
    -H "Authorization: Bearer ${CHAT_API_KEY}" \
    -H "Content-Type: application/json" \
    -H "X-Request-ID: ${REQUEST_ID}" \
    -H "X-Hermes-Session-Id: ${SESSION_ID}" \
    -d '{
      "model": "mock",
      "messages": [
        {
          "role": "user",
          "content": "Use one safe read-only tool if available, then answer in one sentence."
        }
      ],
      "include_agentic_blocks": true
    }')"

  if [ "${status#2}" = "$status" ]; then
    echo "Agent request failed with HTTP ${status}." >&2
    cat "${LIVE_TMPDIR}/agent.json" >&2
    exit 1
  fi

  local trace_id
  trace_id="$(awk 'tolower($1)=="x-trace-id:" {print $2}' "${LIVE_TMPDIR}/headers" | tr -d '\r' | tail -n 1)"
  echo "session_id=${SESSION_ID}"
  echo "request_id=${REQUEST_ID}"
  echo "trace_id=${trace_id:-not-returned}"

  section "2. Tool -> Execution Receipt"
  curl -sS "${BASE_URL}/v1/execution-receipts?session_id=${SESSION_ID}&limit=20" \
    -H "Authorization: Bearer ${AUDIT_API_KEY}" > "${LIVE_TMPDIR}/receipts.json"

  local receipt_id
  receipt_id="$(jq -r '.execution_receipts[0].id // empty' "${LIVE_TMPDIR}/receipts.json")"
  if [ -z "$receipt_id" ]; then
    echo "No receipt found for session ${SESSION_ID}." >&2
    echo "The agent may not have called a tool, or receipt recording may not be wired in this runtime path." >&2
    exit 2
  fi
  jq -r '.execution_receipts[0] | "receipt_id=" + .id + "\ntool_name=" + .tool_name + "\nidempotency_id=" + (.idempotency_id // "") + "\nreceipt_trace_id=" + (.trace_id // "")' "${LIVE_TMPDIR}/receipts.json"

  section "3. Audit Correlation"
  curl -sS "${BASE_URL}/v1/audit-logs?limit=50" \
    -H "Authorization: Bearer ${AUDIT_API_KEY}" > "${LIVE_TMPDIR}/audit.json"

  local audit_line
  audit_line="$(jq -r --arg request_id "$REQUEST_ID" '.audit_logs[]? | select(.request_id == $request_id) | "audit_action=" + .action + "\naudit_request_id=" + .request_id + "\naudit_status=" + (.status_code | tostring)' "${LIVE_TMPDIR}/audit.json")"
  if [ -z "$audit_line" ]; then
    echo "No audit entry found for request_id=${REQUEST_ID}." >&2
    exit 3
  fi
  echo "$audit_line"

  section "Result"
  echo "Live demo complete: receipt_id=${receipt_id}"
}

case "$MODE" in
  fixture|"")
    fixture_mode
    ;;
  live)
    live_mode
    ;;
  *)
    echo "Usage: $0 [fixture|live]" >&2
    exit 64
    ;;
esac
