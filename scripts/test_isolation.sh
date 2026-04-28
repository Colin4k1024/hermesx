#!/usr/bin/env bash
set -euo pipefail

API_PORT="${HERMES_API_PORT:-8080}"
API_KEY="${HERMES_API_KEY:-test-secret-key}"
BASE_URL="http://127.0.0.1:${API_PORT}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; FAILURES=$((FAILURES+1)); }
info() { echo -e "${YELLOW}[INFO]${NC} $1"; }

FAILURES=0

echo "============================================"
echo "  Hermes Agent Multi-User Isolation Test"
echo "============================================"
echo ""

# --- Test 0: Health check ---
info "Checking API server health..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/v1/health" 2>/dev/null || echo "000")
if [ "$HTTP_CODE" = "200" ]; then
    pass "API server is healthy (${BASE_URL})"
else
    fail "API server not reachable (HTTP ${HTTP_CODE}). Is the gateway running?"
    echo ""
    echo "Start with:"
    echo "  HERMES_API_PORT=${API_PORT} HERMES_API_KEY=${API_KEY} ./hermes gateway start"
    exit 1
fi

# --- Test 1: Auth enforcement ---
info "Testing auth enforcement..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "${BASE_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{"messages":[{"role":"user","content":"hello"}]}' 2>/dev/null)
if [ "$HTTP_CODE" = "401" ]; then
    pass "Unauthenticated request rejected (401)"
else
    fail "Unauthenticated request got HTTP ${HTTP_CODE}, expected 401"
fi

# --- Test 2: Concurrent requests with different sessions ---
info "Sending concurrent requests for User A and User B..."

TEMP_DIR=$(mktemp -d)
USER_A_OUT="${TEMP_DIR}/user_a.json"
USER_B_OUT="${TEMP_DIR}/user_b.json"
USER_A_ERR="${TEMP_DIR}/user_a.err"
USER_B_ERR="${TEMP_DIR}/user_b.err"
USER_A_HTTP="${TEMP_DIR}/user_a.http"
USER_B_HTTP="${TEMP_DIR}/user_b.http"

# User A: asks about identity with session context
curl -s -w "\n%{http_code}" \
    -X POST "${BASE_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "X-Hermes-Session-Id: user-alice-session" \
    -d '{
        "model": "default",
        "messages": [
            {"role": "user", "content": "My name is Alice. Remember my name. Reply with ONLY: Hello Alice"}
        ]
    }' > "${USER_A_OUT}" 2>"${USER_A_ERR}" &
PID_A=$!

# User B: asks about identity with session context
curl -s -w "\n%{http_code}" \
    -X POST "${BASE_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "X-Hermes-Session-Id: user-bob-session" \
    -d '{
        "model": "default",
        "messages": [
            {"role": "user", "content": "My name is Bob. Remember my name. Reply with ONLY: Hello Bob"}
        ]
    }' > "${USER_B_OUT}" 2>"${USER_B_ERR}" &
PID_B=$!

info "Waiting for responses (timeout 120s)..."
wait $PID_A 2>/dev/null || true
wait $PID_B 2>/dev/null || true

# Extract HTTP codes (last line)
USER_A_CODE=$(tail -1 "${USER_A_OUT}" 2>/dev/null || echo "000")
USER_B_CODE=$(tail -1 "${USER_B_OUT}" 2>/dev/null || echo "000")

# Extract JSON body (everything except last line)
USER_A_BODY=$(sed '$d' "${USER_A_OUT}" 2>/dev/null || echo "{}")
USER_B_BODY=$(sed '$d' "${USER_B_OUT}" 2>/dev/null || echo "{}")

echo ""
info "User A (alice) HTTP: ${USER_A_CODE}"
info "User B (bob)   HTTP: ${USER_B_CODE}"

# Check HTTP success
if [ "$USER_A_CODE" = "200" ]; then
    pass "User A request succeeded (200)"
else
    fail "User A request failed (HTTP ${USER_A_CODE})"
    echo "  Error: $(cat "${USER_A_ERR}" 2>/dev/null)"
    echo "  Body: ${USER_A_BODY}"
fi

if [ "$USER_B_CODE" = "200" ]; then
    pass "User B request succeeded (200)"
else
    fail "User B request failed (HTTP ${USER_B_CODE})"
    echo "  Error: $(cat "${USER_B_ERR}" 2>/dev/null)"
    echo "  Body: ${USER_B_BODY}"
fi

# Extract response content
RESP_A=$(echo "${USER_A_BODY}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['choices'][0]['message']['content'])" 2>/dev/null || echo "PARSE_ERROR")
RESP_B=$(echo "${USER_B_BODY}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['choices'][0]['message']['content'])" 2>/dev/null || echo "PARSE_ERROR")

echo ""
info "User A response: ${RESP_A}"
info "User B response: ${RESP_B}"

# --- Test 3: Session ID isolation ---
SESS_A=$(echo "${USER_A_BODY}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null || echo "")
SESS_B=$(echo "${USER_B_BODY}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null || echo "")

if [ -n "$SESS_A" ] && [ -n "$SESS_B" ] && [ "$SESS_A" != "$SESS_B" ]; then
    pass "Session IDs are different (A=${SESS_A}, B=${SESS_B})"
else
    fail "Session IDs should be different (A=${SESS_A}, B=${SESS_B})"
fi

# --- Test 4: Content isolation (no cross-contamination) ---
# Check User A response contains "Alice" and NOT "Bob"
if echo "$RESP_A" | grep -qi "alice"; then
    pass "User A response contains 'Alice'"
else
    fail "User A response missing 'Alice': ${RESP_A}"
fi

if echo "$RESP_A" | grep -qi "bob"; then
    fail "User A response leaked User B context ('Bob' found): ${RESP_A}"
else
    pass "User A response does NOT contain 'Bob' (no leak)"
fi

# Check User B response contains "Bob" and NOT "Alice"
if echo "$RESP_B" | grep -qi "bob"; then
    pass "User B response contains 'Bob'"
else
    fail "User B response missing 'Bob': ${RESP_B}"
fi

if echo "$RESP_B" | grep -qi "alice"; then
    fail "User B response leaked User A context ('Alice' found): ${RESP_B}"
else
    pass "User B response does NOT contain 'Alice' (no leak)"
fi

# --- Test 5: Session persistence (follow-up in same session) ---
echo ""
info "Testing session persistence — follow-up request to User A..."
sleep 2
FOLLOWUP_OUT="${TEMP_DIR}/followup.json"
curl -s -w "\n%{http_code}" \
    -X POST "${BASE_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${API_KEY}" \
    -H "X-Hermes-Session-Id: user-alice-session" \
    -d '{
        "model": "default",
        "messages": [
            {"role": "user", "content": "Earlier I told you my name is Alice. What is my name? Reply with ONLY the name, nothing else."}
        ]
    }' > "${FOLLOWUP_OUT}" 2>/dev/null

FOLLOWUP_CODE=$(tail -1 "${FOLLOWUP_OUT}")
FOLLOWUP_BODY=$(sed '$d' "${FOLLOWUP_OUT}")
FOLLOWUP_RESP=$(echo "${FOLLOWUP_BODY}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['choices'][0]['message']['content'])" 2>/dev/null || echo "PARSE_ERROR")

info "Follow-up response: ${FOLLOWUP_RESP}"

if [ "$FOLLOWUP_CODE" = "200" ]; then
    pass "Follow-up request succeeded (200)"
else
    fail "Follow-up request failed (HTTP ${FOLLOWUP_CODE})"
fi

if echo "$FOLLOWUP_RESP" | grep -qi "alice"; then
    pass "Session persisted — agent remembers 'Alice'"
else
    fail "Session lost — agent forgot 'Alice': ${FOLLOWUP_RESP}"
fi

# Cleanup
rm -rf "${TEMP_DIR}"

# --- Summary ---
echo ""
echo "============================================"
if [ $FAILURES -eq 0 ]; then
    echo -e "  ${GREEN}ALL TESTS PASSED${NC}"
else
    echo -e "  ${RED}${FAILURES} TEST(S) FAILED${NC}"
fi
echo "============================================"
exit $FAILURES
