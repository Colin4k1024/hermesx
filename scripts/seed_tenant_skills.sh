#!/usr/bin/env bash
set -euo pipefail

# Seed all bundled skills into MinIO for a given tenant.
# Usage: ./scripts/seed_tenant_skills.sh <tenant-id>
# Example: ./scripts/seed_tenant_skills.sh 0c8b983e-bbdb-456e-9a18-0a7ba8c5014c

TENANT_ID="${1:?Usage: $0 <tenant-id>}"
MINIO_ENDPOINT="${MINIO_ENDPOINT:-http://127.0.0.1:9000}"
MINIO_ACCESS_KEY="${MINIO_ACCESS_KEY:-hermes}"
MINIO_SECRET_KEY="${MINIO_SECRET_KEY:-hermespass}"
MINIO_BUCKET="${MINIO_BUCKET:-hermes-skills}"
MC_ALIAS="hermes-seed"
SKILLS_DIR="$(cd "$(dirname "$0")/../skills" && pwd)"

echo "=== Seeding skills for tenant: ${TENANT_ID} ==="
echo "  Source: ${SKILLS_DIR}"

mc alias set "$MC_ALIAS" "$MINIO_ENDPOINT" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" 2>/dev/null || {
  echo "ERROR: mc not found. Install: brew install minio/stable/mc"
  exit 1
}

mc mb --ignore-existing "${MC_ALIAS}/${MINIO_BUCKET}" 2>/dev/null

UPLOADED=0
FAILED=0
while IFS= read -r skill_path; do
  rel="${skill_path#${SKILLS_DIR}/}"
  if mc pipe "${MC_ALIAS}/${MINIO_BUCKET}/${TENANT_ID}/${rel}" < "$skill_path" 2>/dev/null; then
    echo "  ✓ ${rel}"
    UPLOADED=$((UPLOADED + 1))
  else
    echo "  ✗ ${rel}"
    FAILED=$((FAILED + 1))
  fi
done < <(find "$SKILLS_DIR" -name "SKILL.md" -maxdepth 3)

echo ""
echo "=== Done: ${UPLOADED} uploaded, ${FAILED} failed ==="
