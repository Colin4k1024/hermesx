#!/usr/bin/env bash
# Redis backup script — triggers BGSAVE and copies RDB to backup location.
# Usage: ./redis-backup.sh
#
# Environment variables:
#   REDIS_HOST       - Redis hostname (default: localhost)
#   REDIS_PORT       - Redis port (default: 6379)
#   REDIS_PASSWORD   - Redis auth password (default: empty)
#   BACKUP_DIR       - Local backup directory (default: /backup/redis)
#   S3_BUCKET        - Optional S3 bucket for offsite backup (e.g., s3://my-bucket/redis-backups)
#   REDIS_DATA_DIR   - Redis data directory where dump.rdb lives (default: /data)
#   RETENTION_DAYS   - Days to retain local backups (default: 7)

set -euo pipefail

REDIS_HOST="${REDIS_HOST:-localhost}"
REDIS_PORT="${REDIS_PORT:-6379}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"
BACKUP_DIR="${BACKUP_DIR:-/backup/redis}"
REDIS_DATA_DIR="${REDIS_DATA_DIR:-/data}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/redis-${TIMESTAMP}.rdb"

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

error_exit() {
  log "ERROR: $*" >&2
  exit 1
}

# Build redis-cli command with optional auth
redis_cmd() {
  local cmd=(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT")
  if [[ -n "$REDIS_PASSWORD" ]]; then
    cmd+=(-a "$REDIS_PASSWORD" --no-auth-warning)
  fi
  "${cmd[@]}" "$@"
}

# Verify redis-cli is available
command -v redis-cli >/dev/null 2>&1 || error_exit "redis-cli not found in PATH"

# Verify Redis is reachable
redis_cmd PING >/dev/null 2>&1 || error_exit "Cannot connect to Redis at ${REDIS_HOST}:${REDIS_PORT}"

# Create backup directory
mkdir -p "${BACKUP_DIR}"

log "Starting Redis backup..."
log "  Host: ${REDIS_HOST}:${REDIS_PORT}"
log "  Backup dir: ${BACKUP_DIR}"

# Get last save timestamp before BGSAVE
LAST_SAVE_BEFORE=$(redis_cmd LASTSAVE | tr -d '[:space:]')

# Trigger background save
redis_cmd BGSAVE >/dev/null 2>&1 || error_exit "BGSAVE command failed"
log "BGSAVE initiated, waiting for completion..."

# Wait for BGSAVE to complete (poll LASTSAVE)
MAX_WAIT=300  # 5 minutes max
WAITED=0
while true; do
  LAST_SAVE_AFTER=$(redis_cmd LASTSAVE | tr -d '[:space:]')
  if [[ "$LAST_SAVE_AFTER" != "$LAST_SAVE_BEFORE" ]]; then
    break
  fi
  if [[ $WAITED -ge $MAX_WAIT ]]; then
    error_exit "BGSAVE did not complete within ${MAX_WAIT}s"
  fi
  sleep 2
  WAITED=$((WAITED + 2))
done

log "BGSAVE completed in ~${WAITED}s"

# Copy RDB file to backup location
RDB_PATH="${REDIS_DATA_DIR}/dump.rdb"
if [[ ! -f "$RDB_PATH" ]]; then
  error_exit "RDB file not found at ${RDB_PATH}"
fi

cp "$RDB_PATH" "$BACKUP_FILE" || error_exit "Failed to copy RDB to ${BACKUP_FILE}"

FILESIZE=$(du -sh "$BACKUP_FILE" | cut -f1)
log "Backup saved: ${BACKUP_FILE} (${FILESIZE})"

# Optional: upload to S3
if [[ -n "${S3_BUCKET:-}" ]]; then
  if command -v aws >/dev/null 2>&1; then
    S3_PATH="${S3_BUCKET}/redis-${TIMESTAMP}.rdb"
    log "Uploading to ${S3_PATH}..."
    aws s3 cp "$BACKUP_FILE" "$S3_PATH" --quiet || {
      log "WARNING: S3 upload failed, local backup is still available"
    }
    log "S3 upload complete"
  else
    log "WARNING: aws CLI not found, skipping S3 upload"
  fi
fi

# Prune old local backups
if [[ "$RETENTION_DAYS" -gt 0 ]]; then
  PRUNED=$(find "${BACKUP_DIR}" -name "redis-*.rdb" -mtime "+${RETENTION_DAYS}" -delete -print | wc -l | tr -d '[:space:]')
  if [[ "$PRUNED" -gt 0 ]]; then
    log "Pruned ${PRUNED} backups older than ${RETENTION_DAYS} days"
  fi
fi

log "Redis backup completed successfully"
exit 0
