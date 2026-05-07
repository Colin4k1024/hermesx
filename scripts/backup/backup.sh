#!/usr/bin/env bash
set -euo pipefail

# Hermes SaaS PostgreSQL backup script.
# Usage: ./backup.sh [output_dir]
# Env: POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB, PGHOST (defaults provided)

BACKUP_DIR="${1:-/backup}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DB_USER="${POSTGRES_USER:-hermes}"
DB_NAME="${POSTGRES_DB:-hermes}"
DB_HOST="${PGHOST:-postgres}"
BACKUP_FILE="${BACKUP_DIR}/hermes_${TIMESTAMP}.sql.gz"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-7}"

mkdir -p "${BACKUP_DIR}"

echo "[$(date)] Starting backup of ${DB_NAME}..."

PGPASSWORD="${POSTGRES_PASSWORD}" pg_dump \
  -h "${DB_HOST}" \
  -U "${DB_USER}" \
  -d "${DB_NAME}" \
  --no-owner \
  --no-privileges \
  --clean \
  --if-exists \
  --format=plain \
  | gzip > "${BACKUP_FILE}"

FILESIZE=$(du -sh "${BACKUP_FILE}" | cut -f1)
echo "[$(date)] Backup complete: ${BACKUP_FILE} (${FILESIZE})"

# Prune old backups
if [ "${RETENTION_DAYS}" -gt 0 ]; then
  PRUNED=$(find "${BACKUP_DIR}" -name "hermes_*.sql.gz" -mtime "+${RETENTION_DAYS}" -delete -print | wc -l)
  if [ "${PRUNED}" -gt 0 ]; then
    echo "[$(date)] Pruned ${PRUNED} backups older than ${RETENTION_DAYS} days"
  fi
fi

echo "[$(date)] Done."
