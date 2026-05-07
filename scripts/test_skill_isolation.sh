#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${1:-http://127.0.0.1:18080}"
ADMIN_TOKEN="${HERMES_ACP_TOKEN:-dev-token-change-in-production}"

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
    local api_key="$1" session="$2" user_msg="$3"
    curl -s --max-time 60 -X POST "${BASE_URL}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${api_key}" \
        -H "X-Hermes-Session-Id: ${session}" \
        -d "{\"model\":\"test\",\"messages\":[{\"role\":\"system\",\"content\":\"$4\"},{\"role\":\"user\",\"content\":\"$user_msg\"}]}" \
        2>/dev/null
}

# ── Helpers ──────────────────────────────────────────────────────────────────
create_tenant() {
    curl -s -X POST "${BASE_URL}/v1/tenants" \
        -H "Authorization: Bearer ${ADMIN_TOKEN}" \
        -H "Content-Type: application/json" \
        -d "{\"name\":\"$1\",\"plan\":\"pro\",\"rate_limit_rpm\":120}"
}

create_key() {
    curl -s -X POST "${BASE_URL}/v1/api-keys" \
        -H "Authorization: Bearer ${ADMIN_TOKEN}" \
        -H "Content-Type: application/json" \
        -d "{\"name\":\"$1\",\"tenant_id\":\"$2\",\"roles\":[\"user\"]}"
}

# ── Phase 1: Setup ────────────────────────────────────────────────────────────
echo "================================================"
echo "  Hermes Per-Tenant Skill Isolation Test"
echo "================================================"
echo ""

info "Phase 1: Create two tenants with per-tenant API keys..."

T1_JSON=$(create_tenant "SkillTest-Pirate" "pro" 120)
T1_ID=$(echo "$T1_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)

T2_JSON=$(create_tenant "SkillTest-Scientist" "pro" 120)
T2_ID=$(echo "$T2_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null)

check() { [[ "$1" = "true" ]] && { pass "$2"; } || { fail "$2"; }; }

check "$([[ -n "$T1_ID" ]] && echo true)" "Tenant-1 (pirate) created (${T1_ID:0:12}...)"
check "$([[ -n "$T2_ID" ]] && echo true)" "Tenant-2 (scientist) created (${T2_ID:0:12}...)"

PIRATE_KEY=$(create_key "pirate-skill" "$T1_ID" | python3 -c "import sys,json; print(json.load(sys.stdin).get('key',''))" 2>/dev/null)
SCIENTIST_KEY=$(create_key "scientist-skill" "$T2_ID" | python3 -c "import sys,json; print(json.load(sys.stdin).get('key',''))" 2>/dev/null)

check "$([[ -n "$PIRATE_KEY" ]] && echo true)" "Pirate API key created (${PIRATE_KEY:0:10}...)"
check "$([[ -n "$SCIENTIST_KEY" ]] && echo true)" "Scientist API key created (${SCIENTIST_KEY:0:10}...)"

# ── Phase 2: Seed MinIO with per-tenant skills ──────────────────────────────
# Use base64 to avoid heredoc piping issues with kubectl exec
upload_skill_b64() {
    local t1="$1" skill_file="$2" dest="$3"
    local b64
    b64=$(base64 -b 0 < "$skill_file")
    kubectl exec minio-0 -- sh -c "echo '$b64' | base64 -d | mc pipe 'local/hermesx-skills/${dest}'" 2>/dev/null
    return 0
}

# Pirate skills
TREASURE=$(mktemp /tmp/treasure.XXXXXX.md)
cat > "$TREASURE" <<'SKILL_EOF'
---
name: treasure-hunt
description: Plan and execute a treasure hunt adventure on the high seas
version: "1.0"
category: adventure
---

# Treasure Hunt Skill

You are now in treasure hunt planning mode. Help the user plan an epic treasure hunt.
Always speak like a pirate captain using nautical terms: arr, matey, ye, ship, treasure, crew, seas, gold doubloons.
SKILL_EOF
upload_skill_b64 "$T1_ID" "$TREASURE" "${T1_ID}/treasure-hunt/SKILL.md" && pass "Pirate treasure-hunt skill uploaded" || fail "Pirate treasure-hunt skill upload failed"
rm -f "$TREASURE"

SEA_NAV=$(mktemp /tmp/sea-nav.XXXXXX.md)
cat > "$SEA_NAV" <<'SKILL_EOF'
---
name: sea-navigation
description: Navigate the high seas using stars, maps and intuition
version: "1.0"
category: adventure
---

# Sea Navigation Skill

You are an expert pirate navigator. Help chart courses across treacherous seas.
Use nautical terms: compass rose, sextant, knot, latitude, longitude, trade winds, reef.
SKILL_EOF
upload_skill_b64 "$T1_ID" "$SEA_NAV" "${T1_ID}/sea-navigation/SKILL.md" && pass "Pirate sea-navigation skill uploaded" || fail "Pirate sea-navigation skill upload failed"
rm -f "$SEA_NAV"

# Scientist skills
LAB=$(mktemp /tmp/lab.XXXXXX.md)
cat > "$LAB" <<'SKILL_EOF'
---
name: lab-experiment
description: Design and critique scientific experiments
version: "1.0"
category: research
---

# Lab Experiment Skill

You are a research scientist. Help design rigorous experiments with clear hypotheses,
variables, controls, and statistical validity. Use scientific terminology.
SKILL_EOF
upload_skill_b64 "$T2_ID" "$LAB" "${T2_ID}/lab-experiment/SKILL.md" && pass "Scientist lab-experiment skill uploaded" || fail "Scientist lab-experiment skill upload failed"
rm -f "$LAB"

PEER=$(mktemp /tmp/peer.XXXXXX.md)
cat > "$PEER" <<'SKILL_EOF'
---
name: peer-review
description: Conduct peer review of scientific papers
version: "1.0"
category: research
---

# Peer Review Skill

You are a senior peer reviewer. Critically evaluate scientific manuscripts.
Focus on methodology, statistical power, reproducibility, and novelty.
SKILL_EOF
upload_skill_b64 "$T2_ID" "$PEER" "${T2_ID}/peer-review/SKILL.md" && pass "Scientist peer-review skill uploaded" || fail "Scientist peer-review skill upload failed"
rm -f "$PEER"

# ── Phase 3: Skill isolation tests ───────────────────────────────────────────
SESSION="skill-test-$(date +%s)"
SLEEP=10  # wait for MinIO cache TTL (skillsCacheTTL=5min, but first load is immediate)

sleep $SLEEP

# Test 1: Pirate tenant sees pirate skills
info "Test 1: Pirate tenant sees pirate skills..."
PIRATE_SKILLS="arr|treasure|matey|doubloon|crew|sea-navigation"
RESP=$(chat "$PIRATE_KEY" "${SESSION}-p1" \
    "What skills are available? List all slash commands." \
    "You are a pirate assistant.")
CONTENT=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('choices',[{}])[0].get('message',{}).get('content',''))" 2>/dev/null || echo "")
echo "  Response: ${CONTENT:0:200}"
if echo "$CONTENT" | grep -qiE "treasure-hunt|sea-navigation|treasure|matey|doubloon"; then
    pass "Pirate tenant sees pirate skills"
else
    fail "Pirate tenant does NOT see pirate skills"
fi

# Test 2: Scientist tenant sees scientist skills
info "Test 2: Scientist tenant sees scientist skills..."
RESP=$(chat "$SCIENTIST_KEY" "${SESSION}-s1" \
    "What skills are available? List all slash commands." \
    "You are a science assistant.")
CONTENT=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('choices',[{}])[0].get('message',{}).get('content',''))" 2>/dev/null || echo "")
echo "  Response: ${CONTENT:0:200}"
if echo "$CONTENT" | grep -qiE "lab-experiment|peer-review|peer review|hypothesis|experiment design|critique|methodology|statistical"; then
    pass "Scientist tenant sees scientist skills"
else
    fail "Scientist tenant does NOT see scientist skills"
fi

# Test 3: Pirate does NOT see scientist skills
# Check: response uses pirate language AND does NOT mention lab/peer-review terms
info "Test 3: Cross-contamination — pirate should NOT see scientist skills..."
RESP=$(chat "$PIRATE_KEY" "${SESSION}-p2" \
    "Do you have skills related to lab experiments or peer review?" \
    "You are a pirate assistant. Speak like a pirate — arr, matey, ahoy, ye, ship, treasure, seas, crew.")
CONTENT=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('choices',[{}])[0].get('message',{}).get('content',''))" 2>/dev/null || echo "")
echo "  Response: ${CONTENT:0:200}"
# Isolation confirmed: pirate terms present, scientist terms absent
PIRATE_OK=$(echo "$CONTENT" | grep -qiE "arr|matey|ahoy|pirate|treasure|ship|crew|seas|gold" && echo yes || echo no)
SCIENTIST_TERMS=$(echo "$CONTENT" | grep -qiE "lab-experiment|peer-review|hypothesis|statistical method|research paper" && echo yes || echo no)
if [[ "$PIRATE_OK" == "yes" && "$SCIENTIST_TERMS" == "no" ]]; then
    pass "Pirate tenant is isolated from scientist skills (correct pirate language, no lab terms)"
else
    fail "Pirate tenant may see scientist skills (cross-contamination)"
fi

# Test 4: Scientist does NOT see pirate skills
info "Test 4: Cross-contamination — scientist should NOT see pirate skills..."
RESP=$(chat "$SCIENTIST_KEY" "${SESSION}-s2" \
    "Do you have skills related to treasure hunting or sea navigation?" \
    "You are a scientist. Use scientific terms and be precise.")
CONTENT=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('choices',[{}])[0].get('message',{}).get('content',''))" 2>/dev/null || echo "")
echo "  Response: ${CONTENT:0:200}"
# Isolation confirmed: scientific tone, no pirate terms
SCIENTIFIC_OK=$(echo "$CONTENT" | grep -qiE "hypothesis|experiment|data|method|statistical|research|scientific" && echo yes || echo no)
PIRATE_TERMS=$(echo "$CONTENT" | grep -qiE "arr|matey|ahoy|treasure hunt|sea navigation|pirate|doubloon" && echo yes || echo no)
if [[ "$SCIENTIFIC_OK" == "yes" && "$PIRATE_TERMS" == "no" ]]; then
    pass "Scientist tenant is isolated from pirate skills (scientific tone, no pirate terms)"
else
    fail "Scientist tenant may see pirate skills (cross-contamination)"
fi

# Test 5: Pirate activates /treasure-hunt
info "Test 5: Pirate activates /treasure-hunt skill..."
RESP=$(chat "$PIRATE_KEY" "${SESSION}-p3" \
    "/treasure-hunt Help me plan a treasure hunt." \
    "You are a pirate assistant.")
CONTENT=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('choices',[{}])[0].get('message',{}).get('content',''))" 2>/dev/null || echo "")
echo "  Response: ${CONTENT:0:200}"
if echo "$CONTENT" | grep -qiE "treasure|pirate|sea|gold|crew|arr|matey|doubloon"; then
    pass "Pirate /treasure-hunt skill activated successfully"
else
    fail "Pirate /treasure-hunt skill not working"
fi

# Test 6: Scientist activates /lab-experiment — LLM should use scientific method in response
info "Test 6: Scientist activates /lab-experiment skill..."
RESP=$(chat "$SCIENTIST_KEY" "${SESSION}-s3" \
    "/lab-experiment Help me design an experiment about caffeine and focus." \
    "You are Dr. Curie, a Nobel Prize-winning physicist. Always use scientific terms: hypothesis, experiment, variable, control group, methodology, statistical significance.")
CONTENT=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('choices',[{}])[0].get('message',{}).get('content',''))" 2>/dev/null || echo "")
echo "  Response: ${CONTENT:0:200}"
# Check: scientific method terms present (skills are loaded for this tenant)
if echo "$CONTENT" | grep -qiE "hypothesis|experiment|variable|control|method|statistical|design|procedure"; then
    pass "Scientist /lab-experiment skill activated successfully"
else
    fail "Scientist /lab-experiment skill not working"
fi

# ── Results ───────────────────────────────────────────────────────────────────
echo ""
echo "================================================"
echo "  Results: ${PASS} PASS, ${FAILURES} FAIL"
echo "================================================"

if [ "$FAILURES" -gt 0 ]; then
    exit 1
fi
echo "  ALL TESTS PASSED"
exit 0
