#!/usr/bin/env bash
# MinIO backup script — mirrors data to secondary bucket or local directory.
# Usage: ./minio-backup.sh
#
# Environment variables:
#   MINIO_ENDPOINT   - MinIO server endpoint (default: http://localhost:9000)
#   MINIO_ACCESS_KEY - MinIO access key (required)
#   MINIO_SECRET_KEY - MinIO secret key (required)
#   SOURCE_BUCKET    - Source bucket name (default: hermes-skills)
#   TARGET           - Target: s3://bucket-name OR local path (default: /backup/minio)
#   MC_ALIAS         - mc alias name (default: hermesx)
#   RETENTION_DAYS   - Days to retain local backups (default: 7, only for local targets)

set -euo pipefail

MINIO_ENDPOINT="${MINIO_ENDPOINT:-http://localhost:9000}"
MINIO_ACCESS_KEY="${MINIO_ACCESS_KEY:-}"
MINIO_SECRET_KEY="${MINIO_SECRET_KEY:-}"
SOURCE_BUCKET="${SOURCE_BUCKET:-hermes-skills}"
TARGET="${TARGET:-/backup/minio}"
MC_ALIAS="${MC_ALIAS:-hermesx}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

error_exit() {
  log "ERROR: $*" >&2
  exit 1
}

# Validate required env vars
[[ -n "$MINIO_ACCESS_KEY" ]] || error_exit "MINIO_ACCESS_KEY is required"
[[ -n "$MINIO_SECRET_KEY" ]] || error_exit "MINIO_SECRET_KEY is required"

# Verify mc is available
command -v mc >/dev/null 2>&1 || error_exit "mc (MinIO Client) not found in PATH"

log "Starting MinIO backup..."
log "  Endpoint: ${MINIO_ENDPOINT}"
log "  Source: ${SOURCE_BUCKET}"
log "  Target: ${TARGET}"

# Configure mc alias
mc alias set "$MC_ALIAS" "$MINIO_ENDPOINT" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" --api S3v4 >/dev/null 2>&1 || \
  error_exit "Failed to configure mc alias"

# Verify source bucket exists
mc ls "${MC_ALIAS}/${SOURCE_BUCKET}" >/dev/null 2>&1 || \
  error_exit "Source bucket '${SOURCE_BUCKET}' not found or not accessible"

# Count source objects for reporting
SOURCE_COUNT=$(mc ls --recursive "${MC_ALIAS}/${SOURCE_BUCKET}" 2>/dev/null | wc -l | tr -d '[:space:]')
log "Source bucket contains ${SOURCE_COUNT} objects"

# Perform mirror based on target type
if [[ "$TARGET" == s3://* ]]; then
  # S3-compatible remote target
  TARGET_BUCKET="${TARGET#s3://}"
  TARGET_ALIAS="hermesx-backup"

  # If TARGET contains endpoint info, parse it; otherwise use same endpoint
  mc alias set "$TARGET_ALIAS" "$MINIO_ENDPOINT" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" --api S3v4 >/dev/null 2>&1

  log "Mirroring to remote bucket: ${TARGET_BUCKET}"
  mc mirror --overwrite --remove \
    "${MC_ALIAS}/${SOURCE_BUCKET}" \
    "${TARGET_ALIAS}/${TARGET_BUCKET}" 2>&1 | while IFS= read -r line; do
      log "  $line"
    done

  if [[ ${PIPESTATUS[0]} -ne 0 ]]; then
    error_exit "Mirror to remote bucket failed"
  fi
else
  # Local directory target
  LOCAL_TARGET="${TARGET}/${SOURCE_BUCKET}-${TIMESTAMP}"
  mkdir -p "$LOCAL_TARGET"

  log "Mirroring to local directory: ${LOCAL_TARGET}"
  mc mirror --overwrite \
    "${MC_ALIAS}/${SOURCE_BUCKET}" \
    "$LOCAL_TARGET" 2>&1 | while IFS= read -r line; do
      log "  $line"
    done

  if [[ ${PIPESTATUS[0]} -ne 0 ]]; then
    error_exit "Mirror to local directory failed"
  fi

  # Report size
  BACKUP_SIZE=$(du -sh "$LOCAL_TARGET" | cut -f1)
  log "Local backup size: ${BACKUP_SIZE}"

  # Prune old local backups
  if [[ "$RETENTION_DAYS" -gt 0 ]]; then
    PRUNED=$(find "${TARGET}" -maxdepth 1 -name "${SOURCE_BUCKET}-*" -type d -mtime "+${RETENTION_DAYS}" -exec rm -rf {} \; -print | wc -l | tr -d '[:space:]')
    if [[ "$PRUNED" -gt 0 ]]; then
      log "Pruned ${PRUNED} backup directories older than ${RETENTION_DAYS} days"
    fi
  fi
fi

# Verify target object count
if [[ "$TARGET" != s3://* ]]; then
  TARGET_COUNT=$(find "${LOCAL_TARGET}" -type f | wc -l | tr -d '[:space:]')
  log "Target contains ${TARGET_COUNT} files (source: ${SOURCE_COUNT} objects)"
fi

log "MinIO backup completed successfully"
exit 0
