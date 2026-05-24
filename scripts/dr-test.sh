#!/usr/bin/env bash
# Disaster recovery test — verifies backups can be restored.
# Usage: ./dr-test.sh
#
# This script:
#   1. Verifies the latest PostgreSQL dump backup exists and is gzip-valid
#   2. Tests Redis restore prerequisites from latest backup
#   3. Tests MinIO restore prerequisites from latest backup
#   4. Verifies live data integrity where credentials/tools are available
#   5. Outputs pass/fail report
#
# Environment variables:
#   POSTGRES_BACKUP_DIR - PostgreSQL dump backup directory (default: /backup/postgres)
#   REDIS_HOST         - Redis hostname (default: localhost)
#   REDIS_PORT         - Redis port (default: 6379)
#   REDIS_PASSWORD     - Redis auth password (default: empty)
#   REDIS_BACKUP_DIR   - Redis backup directory (default: /backup/redis)
#   MINIO_ENDPOINT     - MinIO endpoint (default: http://localhost:9000)
#   MINIO_ACCESS_KEY   - MinIO access key (required for MinIO tests)
#   MINIO_SECRET_KEY   - MinIO secret key (required for MinIO tests)
#   MINIO_BACKUP_DIR   - MinIO backup directory (default: /backup/minio)
#   SOURCE_BUCKET      - MinIO source bucket (default: hermes-skills)
#   DR_TEMP_DIR        - Temp directory for DR tests (default: /tmp/hermesx-dr-test)

set -euo pipefail

POSTGRES_BACKUP_DIR="${POSTGRES_BACKUP_DIR:-/backup/postgres}"
REDIS_HOST="${REDIS_HOST:-localhost}"
REDIS_PORT="${REDIS_PORT:-6379}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"
REDIS_BACKUP_DIR="${REDIS_BACKUP_DIR:-/backup/redis}"
MINIO_ENDPOINT="${MINIO_ENDPOINT:-http://localhost:9000}"
MINIO_ACCESS_KEY="${MINIO_ACCESS_KEY:-}"
MINIO_SECRET_KEY="${MINIO_SECRET_KEY:-}"
MINIO_BACKUP_DIR="${MINIO_BACKUP_DIR:-/backup/minio}"
SOURCE_BUCKET="${SOURCE_BUCKET:-hermes-skills}"
DR_TEMP_DIR="${DR_TEMP_DIR:-/tmp/hermesx-dr-test}"

# Test results
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0
REPORT=""

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

record_pass() {
  TESTS_RUN=$((TESTS_RUN + 1))
  TESTS_PASSED=$((TESTS_PASSED + 1))
  REPORT="${REPORT}\n  [PASS] $1"
  log "PASS: $1"
}

record_fail() {
  TESTS_RUN=$((TESTS_RUN + 1))
  TESTS_FAILED=$((TESTS_FAILED + 1))
  REPORT="${REPORT}\n  [FAIL] $1 — $2"
  log "FAIL: $1 — $2"
}

record_skip() {
  REPORT="${REPORT}\n  [SKIP] $1 — $2"
  log "SKIP: $1 — $2"
}

# Build redis-cli command with optional auth
redis_cmd() {
  local cmd=(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT")
  if [[ -n "$REDIS_PASSWORD" ]]; then
    cmd+=(-a "$REDIS_PASSWORD" --no-auth-warning)
  fi
  "${cmd[@]}" "$@"
}

cleanup() {
  log "Cleaning up temp directory..."
  rm -rf "$DR_TEMP_DIR"
}
trap cleanup EXIT

mkdir -p "$DR_TEMP_DIR"

log "============================================"
log "  HermesX Disaster Recovery Test"
log "============================================"
log ""

# ─────────────────────────────────────────────
# Test 0: PostgreSQL dump backup tests
# ─────────────────────────────────────────────
log "--- PostgreSQL Backup Tests ---"

LATEST_PG_BACKUP=""
if [[ -d "$POSTGRES_BACKUP_DIR" ]]; then
  LATEST_PG_BACKUP=$(find "$POSTGRES_BACKUP_DIR" -name "hermes_*.sql.gz" -type f -printf '%T@ %p\n' 2>/dev/null | sort -rn | head -1 | cut -d' ' -f2-)
  # Fallback for macOS (no -printf)
  if [[ -z "$LATEST_PG_BACKUP" ]]; then
    LATEST_PG_BACKUP=$(ls -t "$POSTGRES_BACKUP_DIR"/hermes_*.sql.gz 2>/dev/null | head -1)
  fi
fi

if [[ -n "$LATEST_PG_BACKUP" && -f "$LATEST_PG_BACKUP" ]]; then
  record_pass "PostgreSQL backup file exists: $(basename "$LATEST_PG_BACKUP")"

  PG_FILE_SIZE=$(stat -f%z "$LATEST_PG_BACKUP" 2>/dev/null || stat -c%s "$LATEST_PG_BACKUP" 2>/dev/null || echo "0")
  if [[ "$PG_FILE_SIZE" -gt 100 ]]; then
    record_pass "PostgreSQL backup file size is valid (${PG_FILE_SIZE} bytes)"
  else
    record_fail "PostgreSQL backup file size" "File too small (${PG_FILE_SIZE} bytes)"
  fi

  if command -v gzip >/dev/null 2>&1; then
    if gzip -t "$LATEST_PG_BACKUP" >/dev/null 2>&1; then
      record_pass "PostgreSQL backup gzip integrity check passed"
    else
      record_fail "PostgreSQL backup gzip integrity" "gzip -t failed for $(basename "$LATEST_PG_BACKUP")"
    fi
  else
    record_skip "PostgreSQL gzip integrity check" "gzip not found"
  fi
else
  record_fail "PostgreSQL backup file exists" "No hermes_*.sql.gz backup found in ${POSTGRES_BACKUP_DIR}"
fi

# ─────────────────────────────────────────────
# Test 1: Redis backup file existence
# ─────────────────────────────────────────────
log ""
log "--- Redis Backup Tests ---"

LATEST_REDIS_BACKUP=""
if [[ -d "$REDIS_BACKUP_DIR" ]]; then
  LATEST_REDIS_BACKUP=$(find "$REDIS_BACKUP_DIR" -name "redis-*.rdb" -type f -printf '%T@ %p\n' 2>/dev/null | sort -rn | head -1 | cut -d' ' -f2-)
  # Fallback for macOS (no -printf)
  if [[ -z "$LATEST_REDIS_BACKUP" ]]; then
    LATEST_REDIS_BACKUP=$(ls -t "$REDIS_BACKUP_DIR"/redis-*.rdb 2>/dev/null | head -1)
  fi
fi

if [[ -n "$LATEST_REDIS_BACKUP" && -f "$LATEST_REDIS_BACKUP" ]]; then
  record_pass "Redis backup file exists: $(basename "$LATEST_REDIS_BACKUP")"

  # Check file is non-empty and valid RDB header
  FILE_SIZE=$(stat -f%z "$LATEST_REDIS_BACKUP" 2>/dev/null || stat -c%s "$LATEST_REDIS_BACKUP" 2>/dev/null || echo "0")
  if [[ "$FILE_SIZE" -gt 100 ]]; then
    record_pass "Redis backup file size is valid (${FILE_SIZE} bytes)"
  else
    record_fail "Redis backup file size" "File too small (${FILE_SIZE} bytes)"
  fi

  # Verify RDB magic header (REDIS)
  HEADER=$(head -c 5 "$LATEST_REDIS_BACKUP" 2>/dev/null || true)
  if [[ "$HEADER" == "REDIS" ]]; then
    record_pass "Redis backup has valid RDB header"
  else
    record_fail "Redis backup RDB header" "Invalid header: ${HEADER}"
  fi
else
  record_fail "Redis backup file exists" "No backup found in ${REDIS_BACKUP_DIR}"
fi

# Test 2: Redis connectivity and data check
if command -v redis-cli >/dev/null 2>&1; then
  if redis_cmd PING >/dev/null 2>&1; then
    record_pass "Redis is reachable at ${REDIS_HOST}:${REDIS_PORT}"

    # Check DBSIZE to verify data exists
    DB_SIZE=$(redis_cmd DBSIZE 2>/dev/null | grep -oE '[0-9]+' || echo "0")
    if [[ "$DB_SIZE" -gt 0 ]]; then
      record_pass "Redis contains ${DB_SIZE} keys"
    else
      record_fail "Redis data check" "Database is empty (0 keys)"
    fi

    # Sample key check — get a random key and verify it's readable
    SAMPLE_KEY=$(redis_cmd RANDOMKEY 2>/dev/null || true)
    if [[ -n "$SAMPLE_KEY" && "$SAMPLE_KEY" != "(nil)" ]]; then
      redis_cmd TYPE "$SAMPLE_KEY" >/dev/null 2>&1 && \
        record_pass "Redis sample key '${SAMPLE_KEY}' is readable" || \
        record_fail "Redis sample key read" "Cannot read key '${SAMPLE_KEY}'"
    fi
  else
    record_fail "Redis connectivity" "Cannot connect to ${REDIS_HOST}:${REDIS_PORT}"
  fi
else
  record_skip "Redis connectivity test" "redis-cli not found"
fi

# ─────────────────────────────────────────────
# Test 3: MinIO backup tests
# ─────────────────────────────────────────────
log ""
log "--- MinIO Backup Tests ---"

LATEST_MINIO_BACKUP=""
if [[ -d "$MINIO_BACKUP_DIR" ]]; then
  LATEST_MINIO_BACKUP=$(ls -td "$MINIO_BACKUP_DIR"/${SOURCE_BUCKET}-* 2>/dev/null | head -1)
fi

if [[ -n "$LATEST_MINIO_BACKUP" && -d "$LATEST_MINIO_BACKUP" ]]; then
  record_pass "MinIO backup directory exists: $(basename "$LATEST_MINIO_BACKUP")"

  # Count files in backup
  BACKUP_FILE_COUNT=$(find "$LATEST_MINIO_BACKUP" -type f | wc -l | tr -d '[:space:]')
  if [[ "$BACKUP_FILE_COUNT" -gt 0 ]]; then
    record_pass "MinIO backup contains ${BACKUP_FILE_COUNT} files"
  else
    record_fail "MinIO backup file count" "Backup directory is empty"
  fi

  # Verify a sample file is not corrupted (non-zero size)
  SAMPLE_FILE=$(find "$LATEST_MINIO_BACKUP" -type f | head -1)
  if [[ -n "$SAMPLE_FILE" ]]; then
    SAMPLE_SIZE=$(stat -f%z "$SAMPLE_FILE" 2>/dev/null || stat -c%s "$SAMPLE_FILE" 2>/dev/null || echo "0")
    if [[ "$SAMPLE_SIZE" -gt 0 ]]; then
      record_pass "MinIO sample file is valid ($(basename "$SAMPLE_FILE"), ${SAMPLE_SIZE} bytes)"
    else
      record_fail "MinIO sample file integrity" "File is empty: $(basename "$SAMPLE_FILE")"
    fi
  fi
else
  record_fail "MinIO backup directory exists" "No backup found in ${MINIO_BACKUP_DIR}"
fi

# Test 4: MinIO live connectivity and bucket check
if command -v mc >/dev/null 2>&1 && [[ -n "$MINIO_ACCESS_KEY" && -n "$MINIO_SECRET_KEY" ]]; then
  MC_ALIAS="dr-test"
  mc alias set "$MC_ALIAS" "$MINIO_ENDPOINT" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" --api S3v4 >/dev/null 2>&1

  if mc ls "${MC_ALIAS}/${SOURCE_BUCKET}" >/dev/null 2>&1; then
    record_pass "MinIO source bucket '${SOURCE_BUCKET}' is accessible"

    # Count live objects
    LIVE_COUNT=$(mc ls --recursive "${MC_ALIAS}/${SOURCE_BUCKET}" 2>/dev/null | wc -l | tr -d '[:space:]')
    if [[ "$LIVE_COUNT" -gt 0 ]]; then
      record_pass "MinIO source bucket contains ${LIVE_COUNT} objects"

      # Compare with backup count if available
      if [[ -n "$LATEST_MINIO_BACKUP" && "$BACKUP_FILE_COUNT" -gt 0 ]]; then
        if [[ "$BACKUP_FILE_COUNT" -ge "$LIVE_COUNT" ]]; then
          record_pass "MinIO backup count (${BACKUP_FILE_COUNT}) >= live count (${LIVE_COUNT})"
        else
          record_fail "MinIO backup completeness" "Backup has ${BACKUP_FILE_COUNT} files but live has ${LIVE_COUNT} objects"
        fi
      fi
    else
      record_fail "MinIO source bucket data" "Bucket '${SOURCE_BUCKET}' is empty"
    fi
  else
    record_fail "MinIO bucket accessibility" "Cannot access bucket '${SOURCE_BUCKET}'"
  fi

  # Cleanup alias
  mc alias remove "$MC_ALIAS" >/dev/null 2>&1 || true
else
  record_skip "MinIO live connectivity" "mc not found or credentials not set"
fi

# ─────────────────────────────────────────────
# Final Report
# ─────────────────────────────────────────────
log ""
log "============================================"
log "  DR Test Report"
log "============================================"
printf "%b\n" "$REPORT"
log ""
log "  Total: ${TESTS_RUN} | Passed: ${TESTS_PASSED} | Failed: ${TESTS_FAILED}"
log "============================================"

if [[ "$TESTS_FAILED" -gt 0 ]]; then
  log "DR TEST RESULT: FAIL"
  exit 1
else
  log "DR TEST RESULT: PASS"
  exit 0
fi
