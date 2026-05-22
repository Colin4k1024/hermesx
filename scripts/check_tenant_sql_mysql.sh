#!/bin/bash
set -euo pipefail

# MySQL Tenant SQL Isolation Checker
# Scans internal/store/mysql/*.go for SQL queries on tenant-scoped tables
# and verifies they include a tenant_id predicate or an explicit skip marker.

STORE_DIR="internal/store/mysql"
TENANT_TABLES="sessions|messages|users|audit_logs|api_keys|memories|user_profiles|cron_jobs|cron_job_runs|roles|role_permissions|execution_receipts|workflow_definitions|workflow_versions|workflow_runs|workflow_step_runs|agent_checkpoints|usage_records"
SKIP_FILES="migrate.go|mysql.go"

ERRORS=0

for file in "$STORE_DIR"/*.go; do
  basename=$(basename "$file")

  if echo "$basename" | grep -qE "^($SKIP_FILES)$"; then
    continue
  fi

  while IFS= read -r line_num; do
    line=$(sed -n "${line_num}p" "$file")

    if echo "$line" | grep -qiE "FROM\s+($TENANT_TABLES)|INTO\s+($TENANT_TABLES)|UPDATE\s+($TENANT_TABLES)|DELETE\s+FROM\s+($TENANT_TABLES)|JOIN\s+($TENANT_TABLES)"; then
      if echo "$line" | grep -qiE "role_permissions" && echo "$basename" | grep -q "roles"; then
        continue
      fi

      skip_context=$(sed -n "$((line_num > 10 ? line_num - 10 : 1)),${line_num}p" "$file")
      if echo "$skip_context" | grep -q "tenant_sql_check:skip"; then
        continue
      fi

      context=$(sed -n "${line_num},$((line_num + 15))p" "$file")
      if ! echo "$context" | grep -q "tenant_id"; then
        echo "WARNING: $file:$line_num — SQL on tenant-scoped table may lack tenant_id filter:"
        echo "  $line"
        ERRORS=$((ERRORS + 1))
      fi
    fi
  done < <(grep -nE "FROM\s|INTO\s|UPDATE\s|DELETE\s|JOIN\s" "$file" | cut -d: -f1)
done

if [ "$ERRORS" -gt 0 ]; then
  echo ""
  echo "FAIL: Found $ERRORS potential MySQL tenant isolation gaps."
  exit 1
else
  echo "OK: All MySQL SQL queries on tenant-scoped tables include tenant_id filter."
  exit 0
fi
