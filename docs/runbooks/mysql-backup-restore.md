# MySQL Backup And Restore Runbook

> Scope: HermesX SaaS production deployments using `DATABASE_DRIVER=mysql`.

## Objective

Provide a MySQL recovery proof that is independent from PostgreSQL RLS/PITR assumptions. A MySQL deployment is enterprise-ready only when backup, restore, tenant isolation checks, and audit continuity are rehearsed together.

## Supported Backup Modes

| Mode | Tool | Use Case | RPO | Notes |
|------|------|----------|-----|-------|
| Logical backup | `mysqldump --single-transaction` | Small and medium deployments, portable restore | Last scheduled dump | Requires InnoDB and consistent snapshot. |
| Physical backup | Percona XtraBackup or managed cloud snapshot | Large production datasets | Snapshot interval + binlog | Preferred for high write volume. |
| Point-in-time recovery | Full backup + binary logs | Enterprise recovery drill | Binlog retention window | Requires `log_bin` and retained binlogs. |

## Minimum Production Settings

```ini
binlog_format=ROW
log_bin=mysql-bin
expire_logs_days=7
transaction_isolation=READ-COMMITTED
character_set_server=utf8mb4
collation_server=utf8mb4_0900_ai_ci
```

## Logical Backup

```bash
mysqldump \
  --single-transaction \
  --routines \
  --triggers \
  --events \
  --set-gtid-purged=OFF \
  "$HERMESX_MYSQL_DATABASE" \
  | gzip > "hermesx-mysql-$(date -u +%Y%m%dT%H%M%SZ).sql.gz"
```

## Restore Drill

1. Restore into an isolated database, never directly over production.
2. Run schema migration idempotency check with the same HermesX binary.
3. Run tenant SQL static guard:

```bash
scripts/check_tenant_sql_mysql.sh
```

4. Run backend smoke tests against the restored database:

```bash
DATABASE_DRIVER=mysql DATABASE_URL="$RESTORE_DATABASE_URL" /usr/local/go/bin/go test ./internal/store/mysql ./internal/api ./internal/metering -count=1
```

5. Verify audit continuity:

```sql
SELECT tenant_id, COUNT(*) FROM audit_logs GROUP BY tenant_id;
SELECT tenant_id, COUNT(*) FROM execution_receipts GROUP BY tenant_id;
SELECT tenant_id, COUNT(*) FROM usage_records GROUP BY tenant_id;
```

6. Verify deletion compensation paths on a non-production tenant:
   - GDPR export returns expected tenant-scoped data.
   - GDPR delete removes sessions, messages, memories, profiles, API keys, cron jobs, workflows, audit logs, and tenant objects.
   - Object storage cleanup is run when `MINIO_*` is configured.

## Release Gate

MySQL production support is blocked unless the latest release plan records:

- Backup artifact timestamp and retention location.
- Restore target and checksum or row-count comparison.
- Static SQL guard result.
- Store/API/metering test result.
- GDPR deletion rehearsal result.
- Operator, approver, and rollback decision.
