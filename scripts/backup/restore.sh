#!/usr/bin/env bash
set -euo pipefail

# Hermes SaaS PostgreSQL restore script.
# Usage: ./restore.sh <backup_file.sql.gz>
# Env: POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB, PGHOST (defaults provided)

if [ $# -lt 1 ]; then
  echo "Usage: $0 <backup_file.sql.gz>"
  echo "Available backups:"
  ls -lh /backup/hermes_*.sql.gz 2>/dev/null || echo "  (none found in /backup/)"
  exit 1
fi

BACKUP_FILE="$1"
DB_USER="${POSTGRES_USER:-hermes}"
DB_NAME="${POSTGRES_DB:-hermes}"
DB_HOST="${PGHOST:-postgres}"

if [ ! -f "${BACKUP_FILE}" ]; then
  echo "ERROR: Backup file not found: ${BACKUP_FILE}"
  exit 1
fi

echo "[$(date)] Restoring ${DB_NAME} from ${BACKUP_FILE}..."
echo "WARNING: This will overwrite the current database. Press Ctrl+C to abort."
echo "Continuing in 5 seconds..."
sleep 5

gunzip -c "${BACKUP_FILE}" | PGPASSWORD="${POSTGRES_PASSWORD}" psql \
  -h "${DB_HOST}" \
  -U "${DB_USER}" \
  -d "${DB_NAME}" \
  --single-transaction \
  -v ON_ERROR_STOP=1

echo "[$(date)] Restore complete."
echo "[$(date)] Running migrations to ensure schema is up to date..."

# If hermes binary is available, run migrations
if command -v hermes &> /dev/null; then
  hermes migrate 2>/dev/null || true
fi

echo "[$(date)] Done."
