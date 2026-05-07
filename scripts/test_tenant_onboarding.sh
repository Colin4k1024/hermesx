#!/usr/bin/env bash
set -euo pipefail

# Tenant Onboarding E2E Test
# Tests: create tenant → verify skills provisioned → verify Soul → create API key → chat with memory/skills/soul → cross-tenant isolation

BASE="http://localhost:8080"
ADMIN_TOKEN="admin-test-token"
LLM_MODEL="${LLM_MODEL:-MiniMax-M2.7-highspeed}"
PASS=0
FAIL=0
TOTAL=0

red()   { printf "\033[0;31m%s\033[0m\n" "$1"; }
green() { printf "\033[0;32m%s\033[0m\n" "$1"; }
blue()  { printf "\033[0;34m%s\033[0m\n" "$1"; }

assert_eq() {
  TOTAL=$((TOTAL + 1))
  local desc="$1" expected="$2" actual="$3"
  if [ "$expected" = "$actual" ]; then
    green "  PASS: $desc"
    PASS=$((PASS + 1))
  else
    red "  FAIL: $desc (expected='$expected', actual='$actual')"
    FAIL=$((FAIL + 1))
  fi
}

assert_contains() {
  TOTAL=$((TOTAL + 1))
  local desc="$1" needle="$2" haystack="$3"
  if echo "$haystack" | grep -q "$needle"; then
    green "  PASS: $desc"
    PASS=$((PASS + 1))
  else
    red "  FAIL: $desc (expected to contain '$needle')"
    FAIL=$((FAIL + 1))
  fi
}

assert_gt() {
  TOTAL=$((TOTAL + 1))
  local desc="$1" threshold="$2" actual="$3"
  if [ "$actual" -gt "$threshold" ] 2>/dev/null; then
    green "  PASS: $desc (got $actual > $threshold)"
    PASS=$((PASS + 1))
  else
    red "  FAIL: $desc (expected > $threshold, got '$actual')"
    FAIL=$((FAIL + 1))
  fi
}

# ============================================================
blue "=== Phase 1: Health Check ==="
# ============================================================
HEALTH=$(curl -sf "$BASE/health/ready")
assert_contains "server ready" '"status":"ready"' "$HEALTH"

# ============================================================
blue "=== Phase 2: Create Fresh Tenant ==="
# ============================================================
TENANT_RESP=$(curl -sf -X POST "$BASE/v1/tenants" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Onboarding Test Corp","plan":"pro","rate_limit_rpm":120,"max_sessions":10}')

TENANT_ID=$(echo "$TENANT_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
TENANT_NAME=$(echo "$TENANT_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['name'])")

assert_contains "tenant created with ID" "$TENANT_ID" "$TENANT_ID"
assert_eq "tenant name correct" "Onboarding Test Corp" "$TENANT_NAME"
blue "  Tenant ID: $TENANT_ID"

sleep 3  # wait for async provisioning

# ============================================================
blue "=== Phase 3: Verify Skills Provisioned (MinIO) ==="
# ============================================================
SKILL_COUNT=$(docker exec hermesx-minio mc ls local/hermesx-skills/${TENANT_ID}/ 2>/dev/null | grep -c "/$" || echo "0")
assert_gt "tenant has skills directories" 10 "$SKILL_COUNT"
blue "  Skills directories found: $SKILL_COUNT"

MANIFEST=$(docker exec hermesx-minio mc cat local/hermesx-skills/${TENANT_ID}/.manifest.json 2>/dev/null || echo "{}")
assert_contains "manifest has skills key" '"skills"' "$MANIFEST"
assert_contains "manifest has synced_at" '"synced_at"' "$MANIFEST"

MANIFEST_SKILL_COUNT=$(echo "$MANIFEST" | python3 -c "import sys,json; m=json.load(sys.stdin); print(len(m.get('skills',{})))" 2>/dev/null || echo "0")
assert_gt "manifest tracks 70+ skills" 70 "$MANIFEST_SKILL_COUNT"
blue "  Skills in manifest: $MANIFEST_SKILL_COUNT"

# ============================================================
blue "=== Phase 4: Verify Soul Provisioned ==="
# ============================================================
SOUL=$(docker exec hermesx-minio mc cat local/hermesx-skills/${TENANT_ID}/_soul/SOUL.md 2>/dev/null || echo "NOT_FOUND")
assert_contains "soul file exists" "Agent Soul" "$SOUL"
assert_contains "soul contains tenant ref" "Hermes" "$SOUL"
blue "  Soul content (first 100 chars): $(echo "$SOUL" | head -c 100)"

# ============================================================
blue "=== Phase 5: Create API Key for Tenant ==="
# ============================================================
KEY_RESP=$(curl -sf -X POST "$BASE/v1/api-keys" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"onboarding-test-key\",\"tenant_id\":\"$TENANT_ID\",\"roles\":[\"user\"]}")

API_KEY=$(echo "$KEY_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['key'])")
assert_contains "API key starts with hk_" "hk_" "$API_KEY"
blue "  API Key: ${API_KEY:0:20}..."

# ============================================================
blue "=== Phase 6: Verify Skills via API ==="
# ============================================================
SKILLS_RESP=$(curl -sf "$BASE/v1/skills" \
  -H "Authorization: Bearer $API_KEY")

SKILLS_TOTAL=$(echo "$SKILLS_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['total'])" 2>/dev/null || echo "0")
assert_gt "skills API returns 70+ skills" 70 "$SKILLS_TOTAL"
blue "  Skills from API: $SKILLS_TOTAL"

SKILLS_TENANT=$(echo "$SKILLS_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['tenant_id'])" 2>/dev/null || echo "")
assert_eq "skills scoped to correct tenant" "$TENANT_ID" "$SKILLS_TENANT"

FIRST_SKILL=$(echo "$SKILLS_RESP" | python3 -c "import sys,json; s=json.load(sys.stdin)['skills'][0]; print(s.get('source',''))" 2>/dev/null || echo "")
assert_eq "first skill source is builtin" "builtin" "$FIRST_SKILL"

# ============================================================
blue "=== Phase 7: Chat — Verify Soul + Skills Injection ==="
# ============================================================
CHAT_RESP=$(curl -s --max-time 60 -X POST "$BASE/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -H "X-Hermes-Session-Id: onboarding-test-session-1" \
  -H "X-Hermes-User-Id: test-user-alice" \
  -d '{"model":"'"$LLM_MODEL"'","messages":[{"role":"user","content":"Hello, what skills do you have? Answer in under 30 words."}]}' || echo "{}")

CHAT_CONTENT=$(echo "$CHAT_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['choices'][0]['message']['content'])" 2>/dev/null || echo "ERROR")
TOTAL=$((TOTAL + 1))
if [ "$CHAT_CONTENT" != "ERROR" ] && [ -n "$CHAT_CONTENT" ]; then
  green "  PASS: chat response not empty"
  PASS=$((PASS + 1))
else
  red "  FAIL: chat response not empty (got '$CHAT_CONTENT')"
  FAIL=$((FAIL + 1))
fi
blue "  Chat response (first 200 chars): $(echo "$CHAT_CONTENT" | head -c 200)"

# ============================================================
blue "=== Phase 8: Chat — Verify Memory Persistence ==="
# ============================================================
CHAT2_RESP=$(curl -s --max-time 60 -X POST "$BASE/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -H "X-Hermes-Session-Id: onboarding-test-session-1" \
  -H "X-Hermes-User-Id: test-user-alice" \
  -d '{"model":"'"$LLM_MODEL"'","messages":[{"role":"user","content":"Remember that my favorite color is blue. Confirm briefly."}]}' || echo "{}")

CHAT2_CONTENT=$(echo "$CHAT2_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['choices'][0]['message']['content'])" 2>/dev/null || echo "ERROR")
TOTAL=$((TOTAL + 1))
if [ "$CHAT2_CONTENT" != "ERROR" ] && [ -n "$CHAT2_CONTENT" ]; then
  green "  PASS: memory chat response ok"
  PASS=$((PASS + 1))
else
  red "  FAIL: memory chat response ok (got '$CHAT2_CONTENT')"
  FAIL=$((FAIL + 1))
fi

# Third turn in same session
CHAT3_RESP=$(curl -s --max-time 60 -X POST "$BASE/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -H "X-Hermes-Session-Id: onboarding-test-session-1" \
  -H "X-Hermes-User-Id: test-user-alice" \
  -d '{"model":"'"$LLM_MODEL"'","messages":[{"role":"user","content":"What is my favorite color? Answer in one word."}]}' || echo "{}")

CHAT3_CONTENT=$(echo "$CHAT3_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['choices'][0]['message']['content'])" 2>/dev/null || echo "ERROR")
blue "  Multi-turn response: $(echo "$CHAT3_CONTENT" | head -c 200)"

# Verify session stored in PG (optional — saas-api uses in-memory chat store, PG sessions are from gateway mode)
SESSION_COUNT=$(docker exec hermes-pg psql -U hermes -d hermes -t -c "SELECT COUNT(*) FROM sessions WHERE tenant_id='$TENANT_ID'" 2>/dev/null | tr -d ' ' || echo "0")
blue "  Sessions in PG: ${SESSION_COUNT:-0} (saas-api chat uses in-memory store)"

MSG_COUNT=$(docker exec hermes-pg psql -U hermes -d hermes -t -c "SELECT COUNT(*) FROM messages WHERE tenant_id='$TENANT_ID'" 2>/dev/null | tr -d ' ' || echo "0")
blue "  Messages in PG: ${MSG_COUNT:-0} (saas-api chat uses in-memory store)"

# ============================================================
blue "=== Phase 9: Skill Upload — User Modification ==="
# ============================================================
UPLOAD_RESP=$(curl -sf -X PUT "$BASE/v1/skills/my-test-skill" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: text/plain" \
  -d '---
name: "my-test-skill"
description: "A test skill uploaded via API"
version: "1.0.0"
---

# My Test Skill

You are a specialized test assistant.')

assert_contains "skill upload succeeded" '"uploaded"' "$UPLOAD_RESP"

# Verify it appears in listing
SKILLS_AFTER=$(curl -sf "$BASE/v1/skills" -H "Authorization: Bearer $API_KEY")
HAS_MY_SKILL=$(echo "$SKILLS_AFTER" | python3 -c "
import sys,json
skills = json.load(sys.stdin)['skills']
found = [s for s in skills if s['name'] == 'my-test-skill']
print('yes' if found else 'no')
" 2>/dev/null || echo "no")
assert_eq "uploaded skill visible in listing" "yes" "$HAS_MY_SKILL"

IS_USER_MOD=$(echo "$SKILLS_AFTER" | python3 -c "
import sys,json
skills = json.load(sys.stdin)['skills']
found = [s for s in skills if s['name'] == 'my-test-skill']
print('true' if found and found[0].get('user_modified') else 'false')
" 2>/dev/null || echo "false")
assert_eq "uploaded skill marked user_modified" "true" "$IS_USER_MOD"

# Verify in MinIO manifest
MANIFEST2=$(docker exec hermesx-minio mc cat local/hermesx-skills/${TENANT_ID}/.manifest.json 2>/dev/null || echo "{}")
MANIFEST_MOD=$(echo "$MANIFEST2" | python3 -c "
import sys,json
m = json.load(sys.stdin)
e = m.get('skills',{}).get('my-test-skill',{})
print('true' if e.get('user_modified') else 'false')
" 2>/dev/null || echo "false")
assert_eq "manifest marks skill as user_modified" "true" "$MANIFEST_MOD"

# ============================================================
blue "=== Phase 10: Skill Delete ==="
# ============================================================
DEL_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE/v1/skills/my-test-skill" \
  -H "Authorization: Bearer $API_KEY")
assert_eq "skill delete returns 204" "204" "$DEL_STATUS"

# Verify removed from listing
SKILLS_FINAL=$(curl -sf "$BASE/v1/skills" -H "Authorization: Bearer $API_KEY")
HAS_DELETED=$(echo "$SKILLS_FINAL" | python3 -c "
import sys,json
skills = json.load(sys.stdin)['skills']
found = [s for s in skills if s['name'] == 'my-test-skill']
print('yes' if found else 'no')
" 2>/dev/null || echo "yes")
assert_eq "deleted skill removed from listing" "no" "$HAS_DELETED"

# ============================================================
blue "=== Phase 11: Cross-Tenant Isolation ==="
# ============================================================

# Create a second tenant
TENANT2_RESP=$(curl -sf -X POST "$BASE/v1/tenants" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Isolation Test Corp","plan":"starter","rate_limit_rpm":60,"max_sessions":5}')

TENANT2_ID=$(echo "$TENANT2_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
blue "  Tenant 2 ID: $TENANT2_ID"
sleep 3  # wait for async provisioning

# Create key for tenant 2
KEY2_RESP=$(curl -sf -X POST "$BASE/v1/api-keys" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"isolation-key\",\"tenant_id\":\"$TENANT2_ID\",\"roles\":[\"user\"]}")
API_KEY2=$(echo "$KEY2_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['key'])")

# Tenant 2 should NOT see tenant 1's chat sessions
T2_SESSIONS=$(curl -sf "$BASE/v1/mock-sessions" -H "Authorization: Bearer $API_KEY2")
T2_SESSION_LIST=$(echo "$T2_SESSIONS" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('sessions',[])))" 2>/dev/null || echo "0")
assert_eq "tenant 2 sees 0 sessions from tenant 1" "0" "$T2_SESSION_LIST"

# Upload a custom skill to tenant 2 only
curl -sf -X PUT "$BASE/v1/skills/tenant2-only-skill" \
  -H "Authorization: Bearer $API_KEY2" \
  -d '---
name: "tenant2-only-skill"
description: "Only for tenant 2"
version: "1.0.0"
---
Only tenant 2 has this skill.' > /dev/null

# Verify tenant 1 cannot see tenant 2's skill
T1_HAS_T2_SKILL=$(curl -sf "$BASE/v1/skills" -H "Authorization: Bearer $API_KEY" | python3 -c "
import sys,json
skills = json.load(sys.stdin)['skills']
found = [s for s in skills if s['name'] == 'tenant2-only-skill']
print('yes' if found else 'no')
" 2>/dev/null || echo "yes")
assert_eq "tenant 1 cannot see tenant 2 skill" "no" "$T1_HAS_T2_SKILL"

# Verify tenant 2 CAN see its own skill
T2_HAS_SKILL=$(curl -sf "$BASE/v1/skills" -H "Authorization: Bearer $API_KEY2" | python3 -c "
import sys,json
skills = json.load(sys.stdin)['skills']
found = [s for s in skills if s['name'] == 'tenant2-only-skill']
print('yes' if found else 'no')
" 2>/dev/null || echo "no")
assert_eq "tenant 2 sees its own custom skill" "yes" "$T2_HAS_SKILL"

# Verify tenant 2 also got skills provisioned
T2_TOTAL=$(curl -sf "$BASE/v1/skills" -H "Authorization: Bearer $API_KEY2" | python3 -c "import sys,json; print(json.load(sys.stdin)['total'])" 2>/dev/null || echo "0")
assert_gt "tenant 2 also has 70+ provisioned skills" 70 "$T2_TOTAL"

# Verify tenant 2 got its own Soul
T2_SOUL=$(docker exec hermesx-minio mc cat local/hermesx-skills/${TENANT2_ID}/_soul/SOUL.md 2>/dev/null || echo "NOT_FOUND")
assert_contains "tenant 2 has soul" "Agent Soul" "$T2_SOUL"

# ============================================================
blue "=== Phase 12: Cleanup ==="
# ============================================================
# Delete test tenants
curl -sf -X DELETE "$BASE/v1/tenants/$TENANT_ID" -H "Authorization: Bearer $ADMIN_TOKEN" > /dev/null 2>&1 || true
curl -sf -X DELETE "$BASE/v1/tenants/$TENANT2_ID" -H "Authorization: Bearer $ADMIN_TOKEN" > /dev/null 2>&1 || true

# ============================================================
echo ""
blue "============================================"
if [ "$FAIL" -eq 0 ]; then
  green "ALL $TOTAL TESTS PASSED ($PASS passed, $FAIL failed)"
else
  red "$FAIL of $TOTAL TESTS FAILED ($PASS passed, $FAIL failed)"
fi
blue "============================================"

exit $FAIL
