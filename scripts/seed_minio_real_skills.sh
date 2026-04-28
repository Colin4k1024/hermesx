#!/usr/bin/env bash
set -euo pipefail

MINIO_ENDPOINT="${MINIO_ENDPOINT:-http://127.0.0.1:9000}"
MINIO_ACCESS_KEY="${MINIO_ACCESS_KEY:-hermes}"
MINIO_SECRET_KEY="${MINIO_SECRET_KEY:-hermespass}"
MINIO_BUCKET="${MINIO_BUCKET:-hermes-skills}"
MC_ALIAS="hermes-local"
SKILLS_DIR="$(cd "$(dirname "$0")/../skills" && pwd)"

echo "=== Seeding MinIO with real skills from project ==="
echo "  Skills source: ${SKILLS_DIR}"

mc alias set "$MC_ALIAS" "$MINIO_ENDPOINT" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" 2>/dev/null || {
  echo "ERROR: mc (MinIO Client) not found. Install: brew install minio/stable/mc"
  exit 1
}

mc mb --ignore-existing "${MC_ALIAS}/${MINIO_BUCKET}"

# Clean previous test data
echo "  Cleaning previous tenant data..."
mc rm --recursive --force "${MC_ALIAS}/${MINIO_BUCKET}/tenant-alice/" 2>/dev/null || true
mc rm --recursive --force "${MC_ALIAS}/${MINIO_BUCKET}/tenant-bob/" 2>/dev/null || true

upload_skill() {
  local tenant="$1"
  local skill_name="$2"
  local skill_path="$3"

  if [ ! -f "$skill_path" ]; then
    echo "    WARN: ${skill_path} not found, skipping"
    return 1
  fi

  mc pipe "${MC_ALIAS}/${MINIO_BUCKET}/${tenant}/${skill_name}/SKILL.md" < "$skill_path"
  echo "    ✓ ${tenant}/${skill_name}"
}

# --- tenant-alice: 6 skills (developer tools + research) ---
echo ""
echo "  Uploading tenant-alice skills (6 skills)..."
upload_skill "tenant-alice" "plan" \
  "${SKILLS_DIR}/software-development/plan/SKILL.md"
upload_skill "tenant-alice" "systematic-debugging" \
  "${SKILLS_DIR}/software-development/systematic-debugging/SKILL.md"
upload_skill "tenant-alice" "test-driven-development" \
  "${SKILLS_DIR}/software-development/test-driven-development/SKILL.md"
upload_skill "tenant-alice" "github-issues" \
  "${SKILLS_DIR}/github/github-issues/SKILL.md"
upload_skill "tenant-alice" "github-code-review" \
  "${SKILLS_DIR}/github/github-code-review/SKILL.md"
upload_skill "tenant-alice" "arxiv" \
  "${SKILLS_DIR}/research/arxiv/SKILL.md"

# --- tenant-bob: 5 skills (creative + productivity) ---
echo ""
echo "  Uploading tenant-bob skills (5 skills)..."
upload_skill "tenant-bob" "ascii-art" \
  "${SKILLS_DIR}/creative/ascii-art/SKILL.md"
upload_skill "tenant-bob" "excalidraw" \
  "${SKILLS_DIR}/creative/excalidraw/SKILL.md"
upload_skill "tenant-bob" "notion" \
  "${SKILLS_DIR}/productivity/notion/SKILL.md"
upload_skill "tenant-bob" "linear" \
  "${SKILLS_DIR}/productivity/linear/SKILL.md"
upload_skill "tenant-bob" "obsidian" \
  "${SKILLS_DIR}/note-taking/obsidian/SKILL.md"

echo ""
echo "=== Verifying uploaded skills ==="
echo ""
echo "  tenant-alice:"
mc ls "${MC_ALIAS}/${MINIO_BUCKET}/tenant-alice/" | awk '{print "    " $NF}'
echo ""
echo "  tenant-bob:"
mc ls "${MC_ALIAS}/${MINIO_BUCKET}/tenant-bob/" | awk '{print "    " $NF}'

ALICE_COUNT=$(mc ls --recursive "${MC_ALIAS}/${MINIO_BUCKET}/tenant-alice/" | wc -l | tr -d ' ')
BOB_COUNT=$(mc ls --recursive "${MC_ALIAS}/${MINIO_BUCKET}/tenant-bob/" | wc -l | tr -d ' ')

echo ""
echo "=== Seed complete ==="
echo "  tenant-alice: ${ALICE_COUNT} skills (plan, systematic-debugging, test-driven-development, github-issues, github-code-review, arxiv)"
echo "  tenant-bob:   ${BOB_COUNT} skills (ascii-art, excalidraw, notion, linear, obsidian)"
