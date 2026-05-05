# PostgreSQL PITR Recovery Runbook

## Overview

| Parameter | Value |
|-----------|-------|
| RPO | < 5 minutes |
| RTO | < 1 hour |
| Backup Tool | pgBackRest |
| Stanza | hermes |
| Retention | 2 full + 7 differential |

## Prerequisites

- pgBackRest installed and configured (`/etc/pgbackrest/pgbackrest.conf`)
- Access to the backup repository (`/var/lib/pgbackrest`)
- PostgreSQL 16 binaries available
- Sufficient disk space for restored data (at least 2x current DB size)
- Target recovery timestamp identified (UTC)

## Step-by-Step Recovery

### 1. Stop PostgreSQL

```bash
systemctl stop postgresql
# Or in Docker:
docker compose -f deploy/pitr/docker-compose.pitr.yml stop pg
```

### 2. Verify Available Backups

```bash
pgbackrest --stanza=hermes info
```

Note the backup timestamps and types (full/diff/incr). Confirm the target recovery time falls within the available WAL range.

### 3. Clear Existing Data Directory

```bash
# CAUTION: This removes the current (corrupted/unwanted) data
rm -rf /var/lib/postgresql/data/*
```

### 4. Restore to Specific Timestamp

```bash
pgbackrest --stanza=hermes restore \
  --type=time \
  --target="2024-01-15 14:30:00+00" \
  --target-action=promote \
  --set=latest
```

Replace the `--target` value with your desired recovery point (UTC).

#### Alternative: Restore to Latest Available

```bash
pgbackrest --stanza=hermes restore \
  --type=default \
  --set=latest
```

### 5. Start PostgreSQL

```bash
systemctl start postgresql
# Or in Docker:
docker compose -f deploy/pitr/docker-compose.pitr.yml start pg
```

PostgreSQL will replay WAL files up to the target timestamp, then promote to primary.

### 6. Verify Recovery

```bash
# Check PG is running and accepting connections
psql -U hermes -d hermes -c "SELECT pg_is_in_recovery();"
# Expected: f (false = promoted to primary)

# Check timeline advanced
psql -U hermes -d hermes -c "SELECT pg_current_wal_lsn(), pg_postmaster_start_time();"

# Validate application data integrity
psql -U hermes -d hermes -c "SELECT count(*) FROM conversations;"
psql -U hermes -d hermes -c "SELECT max(updated_at) FROM conversations;"
```

### 7. Re-enable Archiving

After recovery, verify archiving is active:

```bash
pgbackrest --stanza=hermes check
```

Take a new full backup to establish a fresh baseline:

```bash
pgbackrest --stanza=hermes backup --type=full
```

## Troubleshooting

### "WAL file not found" during recovery

- Verify the backup repo is intact: `pgbackrest --stanza=hermes info`
- Check archive completeness: `pgbackrest --stanza=hermes check`
- If WAL gap exists, recovery is limited to the last complete WAL segment before the gap

### Recovery hangs or takes too long

- Check WAL replay progress: `SELECT pg_last_xact_replay_timestamp() FROM pg_stat_replication;`
- Monitor disk I/O; recovery is I/O bound
- Consider increasing `maintenance_work_mem` and `max_parallel_workers` in `recovery.conf`

### "could not connect to server" after restore

- Check `pg_hba.conf` was restored correctly
- Verify socket path and port match expectations
- Check PostgreSQL logs: `tail -f /var/lib/postgresql/data/log/postgresql-*.log`

### Timeline mismatch after recovery

- This is normal. A new timeline is created after PITR
- Update any streaming replicas to follow the new timeline
- Take a fresh full backup on the new timeline

## Estimated Recovery Time

| DB Size | Estimated RTO |
|---------|--------------|
| < 10 GB | 5-15 min |
| 10-50 GB | 15-30 min |
| 50-100 GB | 30-45 min |
| > 100 GB | 45-60 min |

Factors: disk I/O speed, WAL volume to replay, network (if remote repo).

## Contacts

| Role | Responsibility |
|------|---------------|
| DBA on-call | Execute recovery |
| Tech Lead | Approve recovery target time |
| App team | Validate data post-recovery |
