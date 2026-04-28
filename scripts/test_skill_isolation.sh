#!/usr/bin/env bash
set -euo pipefail

API_PORT="${HERMES_API_PORT:-8080}"
API_KEY="${HERMES_API_KEY:-test-secret-key}"
BASE_URL="http://127.0.0.1:${API_PORT}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $1"; PASS=$((PASS+1)); }
fail() { echo -e "${RED}[FAIL]${NC} $1"; FAILURES=$((FAILURES+1)); }
info() { echo -e "${YELLOW}[INFO]${NC} $1"; }

PASS=0
FAILURES=0

chat() {
    local tenant="$1"
    local session="$2"
    local system_prompt="$3"
    local user_msg="$4"

    curl -s -X POST "${BASE_URL}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${API_KEY}" \
        -H "X-Hermes-Session-Id: ${session}" \
        -H "X-Hermes-Tenant-Id: ${tenant}" \
        -d "$(cat <<EOF
{
  "model": "default",
  "messages": [
    {"role": "system", "content": "$system_prompt"},
    {"role": "user", "content": "$user_msg"}
  ]
}
EOF
)" 2>/dev/null
}

echo "================================================"
echo "  Hermes Per-User Skill Isolation Test"
echo "================================================"
echo ""

# --- Test 0: Health check ---
info "Checking API server health..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/v1/health" 2>/dev/null || echo "000")
if [ "$HTTP_CODE" = "200" ]; then
    pass "API server is healthy"
else
    fail "API server not reachable (HTTP ${HTTP_CODE})"
    echo "  Start with: docker compose -f docker-compose.dev.yml up -d"
    exit 1
fi

# --- Test 1: Pirate tenant sees pirate skills ---
info "Test 1: Pirate tenant should see pirate skills in system prompt..."
RESP=$(chat "tenant-pirate" "skill-test-pirate-1" \
    "You are a pirate assistant." \
    "What skills are available to me? List only the slash commands.")
echo "  Response: $(echo "$RESP" | jq -r '.choices[0].message.content' 2>/dev/null | head -3)"

CONTENT=$(echo "$RESP" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
if echo "$CONTENT" | grep -qi "treasure-hunt\|sea-navigation"; then
    pass "Pirate tenant sees pirate skills"
else
    fail "Pirate tenant does NOT see pirate skills"
fi

# --- Test 2: Scientist tenant sees scientist skills ---
info "Test 2: Scientist tenant should see scientist skills..."
RESP=$(chat "tenant-scientist" "skill-test-scientist-1" \
    "You are a science assistant." \
    "What skills are available to me? List only the slash commands.")
echo "  Response: $(echo "$RESP" | jq -r '.choices[0].message.content' 2>/dev/null | head -3)"

CONTENT=$(echo "$RESP" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
if echo "$CONTENT" | grep -qi "lab-experiment\|peer-review"; then
    pass "Scientist tenant sees scientist skills"
else
    fail "Scientist tenant does NOT see scientist skills"
fi

# --- Test 3: Pirate does NOT see scientist skills ---
info "Test 3: Cross-contamination check — pirate should NOT see scientist skills..."
RESP=$(chat "tenant-pirate" "skill-test-pirate-2" \
    "You are a pirate assistant." \
    "Do you have any skills related to lab experiments or peer review? Answer yes or no.")
echo "  Response: $(echo "$RESP" | jq -r '.choices[0].message.content' 2>/dev/null | head -3)"

CONTENT=$(echo "$RESP" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
if echo "$CONTENT" | grep -qi "no\|don't\|do not\|not available"; then
    pass "Pirate tenant does NOT see scientist skills"
else
    fail "Pirate tenant may see scientist skills (cross-contamination)"
fi

# --- Test 4: Scientist does NOT see pirate skills ---
info "Test 4: Cross-contamination check — scientist should NOT see pirate skills..."
RESP=$(chat "tenant-scientist" "skill-test-scientist-2" \
    "You are a science assistant." \
    "Do you have any skills related to treasure hunting or sea navigation? Answer yes or no.")
echo "  Response: $(echo "$RESP" | jq -r '.choices[0].message.content' 2>/dev/null | head -3)"

CONTENT=$(echo "$RESP" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
if echo "$CONTENT" | grep -qi "no\|don't\|do not\|not available"; then
    pass "Scientist tenant does NOT see pirate skills"
else
    fail "Scientist tenant may see pirate skills (cross-contamination)"
fi

# --- Test 5: Pirate activates /treasure-hunt ---
info "Test 5: Pirate activates /treasure-hunt skill..."
RESP=$(chat "tenant-pirate" "skill-test-pirate-3" \
    "You are a pirate assistant." \
    "/treasure-hunt Help me plan a treasure hunt.")
echo "  Response: $(echo "$RESP" | jq -r '.choices[0].message.content' 2>/dev/null | head -3)"

CONTENT=$(echo "$RESP" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
if echo "$CONTENT" | grep -qi "treasure\|pirate\|sea\|gold\|crew\|adventure"; then
    pass "Pirate /treasure-hunt skill activated successfully"
else
    fail "Pirate /treasure-hunt skill not working"
fi

# --- Test 6: Scientist activates /lab-experiment ---
info "Test 6: Scientist activates /lab-experiment skill..."
RESP=$(chat "tenant-scientist" "skill-test-scientist-3" \
    "You are a science assistant." \
    "/lab-experiment Help me design an experiment.")
echo "  Response: $(echo "$RESP" | jq -r '.choices[0].message.content' 2>/dev/null | head -3)"

CONTENT=$(echo "$RESP" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
if echo "$CONTENT" | grep -qi "hypothesis\|experiment\|variable\|method\|control\|sample"; then
    pass "Scientist /lab-experiment skill activated successfully"
else
    fail "Scientist /lab-experiment skill not working"
fi

# --- Test 7: Default tenant has no MinIO skills ---
info "Test 7: Default tenant (no X-Hermes-Tenant-Id) should have no tenant-specific skills..."
RESP=$(chat "default" "skill-test-default-1" \
    "You are a helpful assistant." \
    "What skills or slash commands do you have? List them all.")
echo "  Response: $(echo "$RESP" | jq -r '.choices[0].message.content' 2>/dev/null | head -3)"

CONTENT=$(echo "$RESP" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
if echo "$CONTENT" | grep -qi "treasure-hunt\|lab-experiment"; then
    fail "Default tenant sees tenant-specific skills (should not)"
else
    pass "Default tenant has no tenant-specific skills"
fi

echo ""
echo "================================================"
echo "  Results: ${PASS} PASS, ${FAILURES} FAIL"
echo "================================================"

if [ "$FAILURES" -gt 0 ]; then
    exit 1
fi
