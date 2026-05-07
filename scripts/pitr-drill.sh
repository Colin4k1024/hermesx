#!/usr/bin/env bash
# pitr-drill.sh — Automated PITR recovery drill
#
# This script validates the backup/restore pipeline by:
#   1. Inserting test data with a known timestamp
#   2. Taking an incremental backup
#   3. Simulating data loss (DROP TABLE)
#   4. Restoring to the pre-loss timestamp
#   5. Verifying data integrity
#
# Usage:
#   ./scripts/pitr-drill.sh
#
# Prerequisites:
#   - deploy/pitr/docker-compose.pitr.yml stack running
#   - pgBackRest stanza created (run init profile first)

set -euo pipefail

# --- Configuration ---
COMPOSE_FILE="deploy/pitr/docker-compose.pitr.yml"
PG_CONTAINER="hermesx-pg-pitr"
PG_USER="hermesx"
PG_DB="hermesx"
DRILL_TABLE="pitr_drill_test"
DRILL_ROWS=100

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_step()  { echo -e "\n${GREEN}=== STEP: $* ===${NC}"; }

psql_exec() {
  docker exec "${PG_CONTAINER}" psql -U "${PG_USER}" -d "${PG_DB}" -tAc "$1"
}

pgbackrest_exec() {
  docker exec "${PG_CONTAINER}" pgbackrest "$@"
}

# --- Pre-flight ---
log_step "Pre-flight checks"

if ! docker inspect "${PG_CONTAINER}" &>/dev/null; then
  log_error "Container ${PG_CONTAINER} not running. Start the stack first:"
  log_error "  docker compose -f ${COMPOSE_FILE} up -d"
  exit 1
fi

# Ensure pgbackrest is installed in the container
if ! docker exec "${PG_CONTAINER}" which pgbackrest &>/dev/null; then
  log_info "Installing pgbackrest in container..."
  docker exec "${PG_CONTAINER}" bash -c "apt-get update -qq && apt-get install -y -qq pgbackrest > /dev/null 2>&1"
fi

log_info "Container and tools ready."

# --- Step 1: Insert test data ---
log_step "Insert test data"

psql_exec "DROP TABLE IF EXISTS ${DRILL_TABLE};"
psql_exec "CREATE TABLE ${DRILL_TABLE} (id serial PRIMARY KEY, payload text, created_at timestamptz DEFAULT now());"
psql_exec "INSERT INTO ${DRILL_TABLE} (payload) SELECT md5(random()::text) FROM generate_series(1, ${DRILL_ROWS});"

ROW_COUNT=$(psql_exec "SELECT count(*) FROM ${DRILL_TABLE};")
log_info "Inserted ${ROW_COUNT} rows into ${DRILL_TABLE}."

# Record the timestamp AFTER insert (this is our recovery target)
sleep 2  # Ensure WAL is written
RECOVERY_TARGET=$(psql_exec "SELECT now();")
log_info "Recovery target timestamp: ${RECOVERY_TARGET}"

# Force a WAL switch to ensure data is archived
psql_exec "SELECT pg_switch_wal();" > /dev/null
sleep 3  # Wait for archive

# --- Step 2: Take incremental backup ---
log_step "Take incremental backup"

pgbackrest_exec --stanza=hermesx backup --type=diff 2>&1 | tail -5
log_info "Differential backup complete."

# --- Step 3: Simulate data loss ---
log_step "Simulate data loss"

psql_exec "DROP TABLE ${DRILL_TABLE};"
VERIFY_DROP=$(psql_exec "SELECT count(*) FROM information_schema.tables WHERE table_name='${DRILL_TABLE}';")

if [ "${VERIFY_DROP}" -eq 0 ]; then
  log_info "Table ${DRILL_TABLE} dropped. Data loss simulated."
else
  log_error "DROP TABLE failed. Aborting."
  exit 1
fi

# --- Step 4: Restore to pre-loss timestamp ---
log_step "Restore to pre-loss timestamp"

log_info "Stopping PostgreSQL..."
docker stop "${PG_CONTAINER}" > /dev/null

log_info "Clearing data directory..."
docker run --rm \
  -v "$(docker volume inspect pitr_pg_data --format '{{.Mountpoint}}' 2>/dev/null || echo 'pitr_pg_data'):/data" \
  alpine sh -c "rm -rf /data/*" 2>/dev/null || true

# Use a temporary container for restore
log_info "Restoring from backup to target: ${RECOVERY_TARGET}"
docker run --rm \
  --volumes-from "${PG_CONTAINER}" \
  -v "$(pwd)/deploy/pitr/pgbackrest.conf:/etc/pgbackrest/pgbackrest.conf:ro" \
  postgres:16-bookworm bash -c "
    apt-get update -qq && apt-get install -y -qq pgbackrest > /dev/null 2>&1
    pgbackrest --stanza=hermesx restore \
      --type=time \
      --target=\"${RECOVERY_TARGET}\" \
      --target-action=promote
  " 2>&1 | tail -10

log_info "Restore complete. Starting PostgreSQL..."
docker start "${PG_CONTAINER}" > /dev/null
sleep 5  # Wait for recovery replay

# Wait for PG to become ready
for i in $(seq 1 30); do
  if docker exec "${PG_CONTAINER}" pg_isready -U "${PG_USER}" &>/dev/null; then
    break
  fi
  sleep 1
done

# --- Step 5: Verify data integrity ---
log_step "Verify data integrity"

RECOVERED_COUNT=$(psql_exec "SELECT count(*) FROM ${DRILL_TABLE};" 2>/dev/null || echo "0")

if [ "${RECOVERED_COUNT}" -eq "${DRILL_ROWS}" ]; then
  log_info "PASS: Recovered ${RECOVERED_COUNT}/${DRILL_ROWS} rows."
  RESULT="PASS"
else
  log_error "FAIL: Expected ${DRILL_ROWS} rows, got ${RECOVERED_COUNT}."
  RESULT="FAIL"
fi

# --- Step 6: Cleanup and report ---
log_step "Drill Report"

echo ""
echo "============================================"
echo "  PITR RECOVERY DRILL RESULT: ${RESULT}"
echo "============================================"
echo "  Test rows inserted:   ${DRILL_ROWS}"
echo "  Rows recovered:       ${RECOVERED_COUNT}"
echo "  Recovery target:      ${RECOVERY_TARGET}"
echo "  Container:            ${PG_CONTAINER}"
echo "============================================"
echo ""

# Cleanup test table
psql_exec "DROP TABLE IF EXISTS ${DRILL_TABLE};" > /dev/null 2>&1 || true

if [ "${RESULT}" = "PASS" ]; then
  log_info "Drill completed successfully. PITR pipeline is operational."
  exit 0
else
  log_error "Drill FAILED. Investigate backup/restore pipeline."
  exit 1
fi
