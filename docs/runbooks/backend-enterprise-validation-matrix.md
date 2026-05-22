# Backend Enterprise Validation Matrix

> Scope: PostgreSQL and MySQL SaaS production support.

## Baseline

HermesX supports both `DATABASE_DRIVER=postgres` and `DATABASE_DRIVER=mysql` for SaaS production. PostgreSQL may use RLS/FORCE RLS as an additional database-side guard, but MySQL support is proven through application/store tenant scoping, static SQL guards, regression tests, audit records, and restore drills.

## Required Gates

| Capability | PostgreSQL Gate | MySQL Gate |
|------------|-----------------|------------|
| Tenant isolation | `scripts/check_tenant_sql.sh`; RLS policy review | `scripts/check_tenant_sql_mysql.sh`; explicit cross-tenant skip markers |
| Usage metering | `PGUsageStore` aggregate tests | `MySQLUsageStore` aggregate tests |
| Admin usage aggregation | `TenantUsageAggregator` only; no handler-level RLS disable | `TenantUsageAggregator` only |
| Audit logs | Append/list/delete-by-tenant tests | Append/list/delete-by-tenant tests |
| Execution receipts | Idempotency and tenant query tests | Idempotency and tenant query tests |
| GDPR delete | Export/delete smoke plus object cleanup | Export/delete smoke plus object cleanup |
| Workflows | Definition/version/run/task lifecycle | Definition/version/run/task lifecycle |
| Backup restore | `docs/runbooks/pg-pitr-recovery.md` | `docs/runbooks/mysql-backup-restore.md` |

## Command Set

```bash
scripts/check_tenant_sql.sh
scripts/check_tenant_sql_mysql.sh
/usr/local/go/bin/go test ./internal/store/pg ./internal/store/mysql ./internal/metering ./internal/api -count=1
```

For live database rehearsals, run the same API and store suites twice with backend-specific environment:

```bash
DATABASE_DRIVER=postgres DATABASE_URL="$POSTGRES_DATABASE_URL" /usr/local/go/bin/go test ./internal/store/pg ./internal/api ./internal/metering -count=1
DATABASE_DRIVER=mysql DATABASE_URL="$MYSQL_DATABASE_URL" /usr/local/go/bin/go test ./internal/store/mysql ./internal/api ./internal/metering -count=1
```

## Acceptance Rule

A release may claim enterprise production support for a backend only if its release plan links to:

- Static guard output.
- Test output.
- Backup/restore rehearsal.
- GDPR deletion rehearsal.
- Known deviations and compensating controls.
