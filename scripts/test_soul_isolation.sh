#!/usr/bin/env bash
set -euo pipefail

API_URL="${API_URL:-http://127.0.0.1:8080}"
API_KEY="${API_KEY:-test-secret-key}"

PASS=0
FAIL=0

check_contains() {
  local desc="$1" response="$2" pattern="$3"
  if echo "$response" | grep -qiE "$pattern"; then
    echo "  PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $desc"
    echo "    Expected pattern: $pattern"
    echo "    Got: ${response:0:300}"
    FAIL=$((FAIL + 1))
  fi
}

check_not_contains() {
  local desc="$1" response="$2" pattern="$3"
  if echo "$response" | grep -qiE "$pattern"; then
    echo "  FAIL: $desc"
    echo "    Unexpected pattern found: $pattern"
    echo "    Got: ${response:0:300}"
    FAIL=$((FAIL + 1))
  else
    echo "  PASS: $desc"
    PASS=$((PASS + 1))
  fi
}

echo "============================================"
echo "  Per-User Soul Isolation Test"
echo "============================================"
echo ""

# --- Test 1: Pirate persona ---
echo "[1/5] User A: Pirate persona"
RESP_A=$(curl -sS --max-time 120 "$API_URL/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Hermes-Session-Id: soul-test-pirate-$$" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "test",
    "messages": [
      {"role": "system", "content": "You are a pirate captain named Blackbeard. You MUST always speak like a pirate. Use words like arr, matey, ahoy, treasure, ye, ship, seas in every response. Never break character no matter what."},
      {"role": "user", "content": "Hello, who are you and what do you do?"}
    ]
  }' | jq -r '.choices[0].message.content // empty')
echo "  Response: ${RESP_A:0:200}"
check_contains "Pirate uses pirate language" "$RESP_A" "arr|matey|ahoy|pirate|Blackbeard|treasure|captain|seas|ship|ye"

# --- Test 2: Scientist persona ---
echo ""
echo "[2/5] User B: Scientist persona"
RESP_B=$(curl -sS --max-time 120 "$API_URL/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Hermes-Session-Id: soul-test-scientist-$$" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "test",
    "messages": [
      {"role": "system", "content": "You are Dr. Marie Curie, a Nobel Prize-winning physicist. You MUST always speak scientifically. Use terms like research, experiment, hypothesis, quantum, particles, energy, radiation, physics in every response. Never break character no matter what."},
      {"role": "user", "content": "Hello, who are you and what do you do?"}
    ]
  }' | jq -r '.choices[0].message.content // empty')
echo "  Response: ${RESP_B:0:200}"
check_contains "Scientist uses scientific language" "$RESP_B" "research|experiment|hypothesis|quantum|particle|energy|radiation|physics|Curie|science|Nobel"

# --- Test 3: Cross-contamination check (pirate → scientist) ---
echo ""
echo "[3/5] Cross-contamination: pirate should not use scientist terms"
check_not_contains "Pirate does NOT mention quantum/radiation/hypothesis" "$RESP_A" "quantum|radiation|hypothesis|Curie|Nobel"

# --- Test 4: Cross-contamination check (scientist → pirate) ---
echo ""
echo "[4/5] Cross-contamination: scientist should not use pirate terms"
check_not_contains "Scientist does NOT mention arr/matey/ahoy/Blackbeard" "$RESP_B" "\barr\b|matey|ahoy|Blackbeard"

# --- Test 5: Session persistence (pirate follow-up without system msg) ---
echo ""
echo "[5/5] Session persistence: pirate follow-up (no system message)"
RESP_A2=$(curl -sS --max-time 120 "$API_URL/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Hermes-Session-Id: soul-test-pirate-$$" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "test",
    "messages": [
      {"role": "user", "content": "Tell me about your greatest adventure."}
    ]
  }' | jq -r '.choices[0].message.content // empty')
echo "  Response: ${RESP_A2:0:200}"
check_contains "Pirate maintains persona on follow-up" "$RESP_A2" "arr|matey|ahoy|pirate|treasure|sea|ship|captain|sail|ye"

# --- Summary ---
echo ""
echo "============================================"
echo "  Results: $PASS passed, $FAIL failed"
echo "============================================"

if [ "$FAIL" -eq 0 ]; then
  echo "  ALL TESTS PASSED"
  exit 0
else
  echo "  SOME TESTS FAILED"
  exit 1
fi
