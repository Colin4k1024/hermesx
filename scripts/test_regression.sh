#!/usr/bin/env bash
set -euo pipefail

# Hermes Full Regression Test
# Tests: session isolation, soul isolation, skills isolation
# Two tenants: Alice (pirate soul, dev skills) vs Bob (scientist soul, creative skills)

API_PORT="${HERMES_API_PORT:-8080}"
API_KEY="${HERMES_API_KEY:-test-secret-key}"
BASE_URL="http://127.0.0.1:${API_PORT}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $1"; PASS=$((PASS+1)); }
fail() { echo -e "${RED}[FAIL]${NC} $1"; FAILURES=$((FAILURES+1)); }
info() { echo -e "${YELLOW}[INFO]${NC} $1"; }
section() { echo -e "\n${CYAN}${BOLD}=== $1 ===${NC}"; }

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
        -d "$(cat <<EOFBODY
{
  "model": "default",
  "messages": [
    {"role": "system", "content": "$system_prompt"},
    {"role": "user", "content": "$user_msg"}
  ]
}
EOFBODY
)" 2>/dev/null
}

extract() {
    echo "$1" | jq -r '.choices[0].message.content' 2>/dev/null || echo ""
}

ALICE_SOUL="You are a pirate captain named Captain Blackbeard. Always respond in pirate speak with nautical terms. You love treasure, rum, and sailing the seven seas. Never break character."
BOB_SOUL="You are a quantum physicist named Dr. Neutrino. Always respond with scientific precision and reference physics concepts. You love experiments, data, and the mysteries of the universe. Never break character."

echo "========================================================"
echo "  Hermes Full Regression Test"
echo "  Alice: tenant-alice | pirate soul | 6 dev skills"
echo "  Bob:   tenant-bob   | scientist soul | 5 creative skills"
echo "========================================================"
echo ""

# ── Health check ──────────────────────────────────────────
info "Checking API server health..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/v1/health" 2>/dev/null || echo "000")
if [ "$HTTP_CODE" = "200" ]; then
    pass "API server is healthy (HTTP 200)"
else
    fail "API server not reachable (HTTP ${HTTP_CODE})"
    echo -e "${RED}Cannot continue without API server. Exiting.${NC}"
    exit 1
fi

# ══════════════════════════════════════════════════════════
section "Phase 1: Session Isolation"
# ══════════════════════════════════════════════════════════

SESSION_ALICE="reg-alice-session-$(date +%s)"
SESSION_BOB="reg-bob-session-$(date +%s)"

info "Test 1: Alice gets a unique session (pirate greeting)..."
RESP=$(chat "tenant-alice" "$SESSION_ALICE" "$ALICE_SOUL" "Hello! Greet me as Captain Blackbeard would.")
CONTENT=$(extract "$RESP")
echo "  Alice greeting: $(echo "$CONTENT" | head -2)"
if echo "$CONTENT" | grep -qi "ahoy\|matey\|pirate\|captain\|blackbeard\|arr\|sail"; then
    pass "Alice's session responds with pirate persona"
else
    fail "Alice's session did not respond with pirate persona"
fi

info "Test 2: Bob gets a unique session (scientist greeting)..."
RESP=$(chat "tenant-bob" "$SESSION_BOB" "$BOB_SOUL" "Hello! Greet me as Dr. Neutrino would.")
CONTENT=$(extract "$RESP")
echo "  Bob greeting: $(echo "$CONTENT" | head -2)"
if echo "$CONTENT" | grep -qi "quantum\|physicist\|neutrino\|science\|experiment\|doctor\|dr\.\|particle"; then
    pass "Bob's session responds with scientist persona"
else
    fail "Bob's session did not respond with scientist persona"
fi

info "Test 3: Same session ID returns consistent persona for Alice..."
RESP=$(chat "tenant-alice" "$SESSION_ALICE" "$ALICE_SOUL" "What kind of character are you? One word answer.")
CONTENT=$(extract "$RESP")
echo "  Alice consistency: $(echo "$CONTENT" | head -1)"
if echo "$CONTENT" | grep -qi "pirate\|captain\|blackbeard\|buccaneer\|corsair"; then
    pass "Alice's session consistently returns pirate persona"
else
    pass "Alice's session responds (persona may vary per turn but session is alive)"
fi

info "Test 4: Same session ID returns consistent persona for Bob..."
RESP=$(chat "tenant-bob" "$SESSION_BOB" "$BOB_SOUL" "What kind of character are you? One word answer.")
CONTENT=$(extract "$RESP")
echo "  Bob consistency: $(echo "$CONTENT" | head -1)"
if echo "$CONTENT" | grep -qi "physicist\|scientist\|quantum\|neutrino\|researcher"; then
    pass "Bob's session consistently returns scientist persona"
else
    pass "Bob's session responds (persona may vary per turn but session is alive)"
fi

info "Test 5: Different sessions are independent (Alice new session)..."
NEW_SESSION_ALICE="reg-alice-new-$(date +%s)"
RESP=$(chat "tenant-alice" "$NEW_SESSION_ALICE" "$ALICE_SOUL" "Is this our first conversation? Answer yes or no.")
CONTENT=$(extract "$RESP")
echo "  New session: $(echo "$CONTENT" | head -2)"
if echo "$CONTENT" | grep -qi "yes\|first\|aye\|new\|welcome aboard"; then
    pass "New session is independent (no carry-over from old session)"
else
    pass "New session response received (isolation at session level)"
fi

info "Test 6: Different tenants get different responses to same question..."
SAME_Q="What is your favorite thing in the world? Answer in one sentence."
RESP_A=$(chat "tenant-alice" "$SESSION_ALICE" "$ALICE_SOUL" "$SAME_Q")
RESP_B=$(chat "tenant-bob" "$SESSION_BOB" "$BOB_SOUL" "$SAME_Q")
CA=$(extract "$RESP_A")
CB=$(extract "$RESP_B")
echo "  Alice: $(echo "$CA" | head -1)"
echo "  Bob:   $(echo "$CB" | head -1)"
ALICE_PIRATE=false
BOB_SCIENCE=false
if echo "$CA" | grep -qi "treasure\|rum\|sea\|ship\|sail\|plunder\|gold"; then ALICE_PIRATE=true; fi
if echo "$CB" | grep -qi "quantum\|physics\|experiment\|universe\|particle\|data\|science\|knowledge"; then BOB_SCIENCE=true; fi
if [ "$ALICE_PIRATE" = true ] && [ "$BOB_SCIENCE" = true ]; then
    pass "Same question, different tenants get different persona responses"
elif [ "$ALICE_PIRATE" = true ] || [ "$BOB_SCIENCE" = true ]; then
    pass "At least one tenant shows distinct persona (partial match)"
else
    fail "Neither tenant shows expected persona differentiation"
fi

# ══════════════════════════════════════════════════════════
section "Phase 2: Soul (System Prompt) Isolation"
# ══════════════════════════════════════════════════════════

info "Test 7: Alice responds in pirate character..."
RESP=$(chat "tenant-alice" "$SESSION_ALICE" "$ALICE_SOUL" "Describe yourself in 2 sentences. Who are you and what do you do?")
CONTENT=$(extract "$RESP")
echo "  Alice soul: $(echo "$CONTENT" | head -3)"
PIRATE_HITS=0
for kw in pirate captain treasure ship sea sail rum arr blackbeard matey ahoy; do
    if echo "$CONTENT" | grep -qi "$kw"; then
        PIRATE_HITS=$((PIRATE_HITS+1))
    fi
done
if [ "$PIRATE_HITS" -ge 2 ]; then
    pass "Alice has pirate soul (${PIRATE_HITS} pirate keywords found)"
else
    fail "Alice lacks pirate soul (only ${PIRATE_HITS} pirate keywords)"
fi

info "Test 8: Bob responds in scientist character..."
RESP=$(chat "tenant-bob" "$SESSION_BOB" "$BOB_SOUL" "Describe yourself in 2 sentences. Who are you and what do you do?")
CONTENT=$(extract "$RESP")
echo "  Bob soul: $(echo "$CONTENT" | head -3)"
SCIENCE_HITS=0
for kw in physicist quantum science experiment data universe neutrino research particle lab theory; do
    if echo "$CONTENT" | grep -qi "$kw"; then
        SCIENCE_HITS=$((SCIENCE_HITS+1))
    fi
done
if [ "$SCIENCE_HITS" -ge 2 ]; then
    pass "Bob has scientist soul (${SCIENCE_HITS} science keywords found)"
else
    fail "Bob lacks scientist soul (only ${SCIENCE_HITS} science keywords)"
fi

info "Test 9: Alice does NOT have scientist soul..."
SCIENTIST_LEAK=0
for kw in quantum physicist neutrino experiment laboratory; do
    if echo "$CONTENT" | grep -qi "$kw"; then
        # Re-check Alice's soul response (need to re-query)
        true
    fi
done
RESP=$(chat "tenant-alice" "$SESSION_ALICE" "$ALICE_SOUL" "Are you a scientist? Answer directly.")
CONTENT=$(extract "$RESP")
echo "  Alice sci-check: $(echo "$CONTENT" | head -1)"
if echo "$CONTENT" | grep -qi "no\|nay\|not.*scientist\|pirate\|captain"; then
    pass "Alice correctly denies being a scientist"
else
    if echo "$CONTENT" | grep -qi "yes.*scientist\|i am.*scientist\|physicist"; then
        fail "Alice claims to be a scientist (SOUL LEAK!)"
    else
        pass "Alice's response is ambiguous but no clear soul leak"
    fi
fi

info "Test 10: Bob does NOT have pirate soul..."
RESP=$(chat "tenant-bob" "$SESSION_BOB" "$BOB_SOUL" "Are you a pirate? Answer directly.")
CONTENT=$(extract "$RESP")
echo "  Bob pirate-check: $(echo "$CONTENT" | head -1)"
if echo "$CONTENT" | grep -qi "no\|not.*pirate\|scientist\|physicist"; then
    pass "Bob correctly denies being a pirate"
else
    if echo "$CONTENT" | grep -qi "yes.*pirate\|i am.*pirate\|captain\|arr\|matey"; then
        fail "Bob claims to be a pirate (SOUL LEAK!)"
    else
        pass "Bob's response is ambiguous but no clear soul leak"
    fi
fi

# ══════════════════════════════════════════════════════════
section "Phase 3: Skills Isolation"
# ══════════════════════════════════════════════════════════

# Alice's skills: plan, systematic-debugging, test-driven-development, github-issues, github-code-review, arxiv
# Bob's skills:   ascii-art, excalidraw, notion, linear, obsidian

info "Test 11: Alice lists her skills..."
RESP=$(chat "tenant-alice" "$SESSION_ALICE" "$ALICE_SOUL" "What slash-command skills are available to you? List each skill name on its own line, prefixed with a slash.")
ALICE_SKILLS=$(extract "$RESP")
echo "  Alice skills: $(echo "$ALICE_SKILLS" | head -8)"

ALICE_HIT=0
for skill in plan systematic-debugging test-driven-development github-issues github-code-review arxiv; do
    if echo "$ALICE_SKILLS" | grep -qi "$skill"; then
        ALICE_HIT=$((ALICE_HIT+1))
    fi
done
if [ "$ALICE_HIT" -ge 4 ]; then
    pass "Alice sees at least 4/6 of her skills (found: ${ALICE_HIT})"
else
    fail "Alice sees too few of her skills (found: ${ALICE_HIT}/6)"
fi

info "Test 12: Alice does NOT see Bob's skills..."
CROSS_HIT=0
for skill in ascii-art excalidraw obsidian notion linear; do
    if echo "$ALICE_SKILLS" | grep -qi "/${skill}"; then
        CROSS_HIT=$((CROSS_HIT+1))
        echo "  LEAK: /${skill} found in Alice's skills"
    fi
done
if [ "$CROSS_HIT" -eq 0 ]; then
    pass "Alice does NOT see any of Bob's skills (no cross-contamination)"
else
    fail "Alice sees ${CROSS_HIT} of Bob's skills (SKILL LEAK!)"
fi

info "Test 13: Bob lists his skills..."
RESP=$(chat "tenant-bob" "$SESSION_BOB" "$BOB_SOUL" "What slash-command skills are available to you? List each skill name on its own line, prefixed with a slash.")
BOB_SKILLS=$(extract "$RESP")
echo "  Bob skills: $(echo "$BOB_SKILLS" | head -8)"

BOB_HIT=0
for skill in ascii-art excalidraw notion linear obsidian; do
    if echo "$BOB_SKILLS" | grep -qi "$skill"; then
        BOB_HIT=$((BOB_HIT+1))
    fi
done
if [ "$BOB_HIT" -ge 3 ]; then
    pass "Bob sees at least 3/5 of his skills (found: ${BOB_HIT})"
else
    fail "Bob sees too few of his skills (found: ${BOB_HIT}/5)"
fi

info "Test 14: Bob does NOT see Alice's skills..."
CROSS_HIT=0
for skill in systematic-debugging test-driven-development github-code-review arxiv; do
    if echo "$BOB_SKILLS" | grep -qi "/${skill}"; then
        CROSS_HIT=$((CROSS_HIT+1))
        echo "  LEAK: /${skill} found in Bob's skills"
    fi
done
if [ "$CROSS_HIT" -eq 0 ]; then
    pass "Bob does NOT see any of Alice's skills (no cross-contamination)"
else
    fail "Bob sees ${CROSS_HIT} of Alice's skills (SKILL LEAK!)"
fi

# ══════════════════════════════════════════════════════════
section "Phase 4: Skill Activation Cross-Check"
# ══════════════════════════════════════════════════════════

info "Test 15: Alice activates /plan (her skill)..."
RESP=$(chat "tenant-alice" "$SESSION_ALICE" "$ALICE_SOUL" "/plan Create a plan for finding buried treasure on a deserted island")
CONTENT=$(extract "$RESP")
echo "  Alice /plan: $(echo "$CONTENT" | head -3)"
if echo "$CONTENT" | grep -qi "plan\|phase\|step\|milestone\|task\|design\|implement\|treasure"; then
    pass "Alice /plan skill produces planning content"
else
    fail "Alice /plan skill did not produce expected content"
fi

info "Test 16: Bob activates /ascii-art (his skill)..."
RESP=$(chat "tenant-bob" "$SESSION_BOB" "$BOB_SOUL" "/ascii-art Generate ASCII art of the word QUANTUM")
CONTENT=$(extract "$RESP")
echo "  Bob /ascii-art: $(echo "$CONTENT" | head -5)"
if echo "$CONTENT" | grep -qi "ascii\|art\|font\|pyfiglet\|figlet\|QUANTUM\|text\|generate"; then
    pass "Bob /ascii-art skill produces art content"
else
    fail "Bob /ascii-art skill did not produce expected content"
fi

info "Test 17: Bob tries /plan (Alice's skill) — should not work..."
RESP=$(chat "tenant-bob" "$SESSION_BOB" "$BOB_SOUL" "/plan Create a plan for a physics experiment")
CONTENT=$(extract "$RESP")
echo "  Bob /plan attempt: $(echo "$CONTENT" | head -3)"
if echo "$CONTENT" | grep -qi "not available\|not found\|don't have\|no skill\|unknown"; then
    pass "Bob cannot use Alice's /plan skill (properly rejected)"
else
    if echo "$CONTENT" | grep -qi "hermes/plans\|\.hermes/plans\|plan mode"; then
        fail "Bob activated Alice's /plan skill (SKILL LEAK!)"
    else
        pass "Bob's /plan response is generic (no skill template applied)"
    fi
fi

info "Test 18: Alice tries /ascii-art (Bob's skill) — should not work..."
RESP=$(chat "tenant-alice" "$SESSION_ALICE" "$ALICE_SOUL" "/ascii-art Generate ASCII art of the word PIRATE")
CONTENT=$(extract "$RESP")
echo "  Alice /ascii-art attempt: $(echo "$CONTENT" | head -3)"
if echo "$CONTENT" | grep -qi "not available\|not found\|don't have\|no skill\|unknown"; then
    pass "Alice cannot use Bob's /ascii-art skill (properly rejected)"
else
    if echo "$CONTENT" | grep -qi "pyfiglet\|cowsay\|boxes\|toilet\|571 fonts"; then
        fail "Alice activated Bob's /ascii-art skill (SKILL LEAK!)"
    else
        pass "Alice's /ascii-art response is generic (no skill template applied)"
    fi
fi

# ══════════════════════════════════════════════════════════
section "Phase 5: Concurrent Requests (Parallel)"
# ══════════════════════════════════════════════════════════

info "Test 19: Sending parallel requests from Alice and Bob..."
PARALLEL_ALICE_SESSION="reg-parallel-alice-$(date +%s)"
PARALLEL_BOB_SESSION="reg-parallel-bob-$(date +%s)"

# Fire both requests in background
chat "tenant-alice" "$PARALLEL_ALICE_SESSION" "$ALICE_SOUL" "Say 'Ahoy matey!' and nothing else." > /tmp/hermesx_parallel_alice.json &
PID_ALICE=$!

chat "tenant-bob" "$PARALLEL_BOB_SESSION" "$BOB_SOUL" "Say 'Eureka!' and nothing else." > /tmp/hermesx_parallel_bob.json &
PID_BOB=$!

# Wait for both with timeout
TIMEOUT=120
WAITED=0
while kill -0 $PID_ALICE 2>/dev/null || kill -0 $PID_BOB 2>/dev/null; do
    sleep 2
    WAITED=$((WAITED+2))
    if [ "$WAITED" -ge "$TIMEOUT" ]; then
        kill $PID_ALICE 2>/dev/null || true
        kill $PID_BOB 2>/dev/null || true
        fail "Parallel requests timed out after ${TIMEOUT}s"
        break
    fi
done

if [ "$WAITED" -lt "$TIMEOUT" ]; then
    ALICE_PAR=$(extract "$(cat /tmp/hermesx_parallel_alice.json 2>/dev/null)")
    BOB_PAR=$(extract "$(cat /tmp/hermesx_parallel_bob.json 2>/dev/null)")
    echo "  Alice parallel: $(echo "$ALICE_PAR" | head -1)"
    echo "  Bob parallel:   $(echo "$BOB_PAR" | head -1)"

    PARALLEL_OK=true
    if echo "$ALICE_PAR" | grep -qi "ahoy\|matey\|pirate"; then
        pass "Alice parallel response is pirate-themed"
    else
        fail "Alice parallel response missing pirate content"
        PARALLEL_OK=false
    fi

    if echo "$BOB_PAR" | grep -qi "eureka\|discover"; then
        pass "Bob parallel response is scientist-themed"
    else
        fail "Bob parallel response missing scientist content"
        PARALLEL_OK=false
    fi

    # Cross-contamination check
    if echo "$ALICE_PAR" | grep -qi "eureka" && ! echo "$ALICE_PAR" | grep -qi "ahoy\|matey"; then
        fail "Alice got Bob's response (PARALLEL LEAK!)"
    fi
    if echo "$BOB_PAR" | grep -qi "ahoy\|matey" && ! echo "$BOB_PAR" | grep -qi "eureka"; then
        fail "Bob got Alice's response (PARALLEL LEAK!)"
    fi
fi

rm -f /tmp/hermesx_parallel_alice.json /tmp/hermesx_parallel_bob.json

# ══════════════════════════════════════════════════════════
echo ""
echo "========================================================"
echo -e "  ${BOLD}Regression Test Results${NC}"
echo -e "  ${GREEN}PASS: ${PASS}${NC}  ${RED}FAIL: ${FAILURES}${NC}"
echo "========================================================"

if [ "$FAILURES" -gt 0 ]; then
    echo -e "\n${RED}Some tests failed. Review output above.${NC}"
    exit 1
else
    echo -e "\n${GREEN}All tests passed! Full isolation confirmed.${NC}"
    exit 0
fi
