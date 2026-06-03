# Redis & MinIO/RustFS Backup Runbook

## Overview

This runbook covers backup and restore procedures for the two stateful services
beyond PostgreSQL: Redis (session locks, rate-limit state, caches) and
MinIO/RustFS (tenant skill packages, GDPR exports, object storage).

---

## Redis Backup

### What is stored in Redis

| Key pattern | Purpose | Durability requirement |
|-------------|---------|----------------------|
| `lock:session:*` | Distributed session locks | Ephemeral (TTL-based) |
| `rl:*` / `ratelimit:*` | Rate limit sliding windows | Ephemeral (rebuilds automatically) |
| `agent:cache:*` | Context cache | Ephemeral |
| `pairing:*` | Channel pairing approvals | Ephemeral (TTL-based) |
| `status:gateway:*` | Instance health status | Ephemeral |

**Assessment:** Redis data in HermesX is entirely ephemeral/cacheable. Loss of Redis
causes brief rate-limit inaccuracy and session lock re-acquisition, but no data loss.
Backup is optional but useful for debugging and capacity planning.

### RDB Snapshots (recommended for debugging)

```bash
# Trigger manual snapshot
redis-cli -u $REDIS_URL BGSAVE

# Check last save time
redis-cli -u $REDIS_URL LASTSAVE

# Copy dump file (typical location)
cp /var/lib/redis/dump.rdb /backup/redis/dump-$(date +%Y%m%d-%H%M%S).rdb
```

### AOF Persistence (optional, for warm restart)

```ini
# redis.conf additions for warm restart (not required for correctness)
appendonly yes
appendfsync everysec
auto-aof-rewrite-percentage 100
auto-aof-rewrite-min-size 64mb
```

### Kubernetes CronJob (if Redis runs in K8s)

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: redis-rdb-backup
  namespace: hermesx
spec:
  schedule: "0 2 * * *"  # daily at 02:00
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: redis:7-alpine
            command:
            - sh
            - -c
            - |
              redis-cli -u $REDIS_URL BGSAVE
              sleep 5
              redis-cli -u $REDIS_URL --rdb /backup/dump.rdb
              gzip /backup/dump.rdb
              mv /backup/dump.rdb.gz /backup/redis-$(date +%Y%m%d).rdb.gz
            env:
            - name: REDIS_URL
              valueFrom:
                secretKeyRef:
                  name: hermesx-secrets
                  key: redis-url
            volumeMounts:
            - name: backup-vol
              mountPath: /backup
          volumes:
          - name: backup-vol
            persistentVolumeClaim:
              claimName: hermesx-backup-pvc
          restartPolicy: OnFailure
```

### Redis Restore

```bash
# Stop Redis, replace dump, restart
systemctl stop redis
cp /backup/redis/dump-YYYYMMDD-HHMMSS.rdb /var/lib/redis/dump.rdb
chown redis:redis /var/lib/redis/dump.rdb
systemctl start redis

# Verify
redis-cli -u $REDIS_URL DBSIZE
redis-cli -u $REDIS_URL INFO keyspace
```

### Redis High Availability

For production multi-pod deployments:
- Use Redis Sentinel (minimum 3 nodes) or Redis Cluster for automatic failover
- Configure `REDIS_URL` with sentinel protocol: `redis+sentinel://sentinel1:26379,sentinel2:26379/mymaster`
- Rate limiter and session locks automatically reconnect on failover

---

## MinIO / RustFS Backup

### What is stored in object storage

| Bucket/Prefix | Purpose | Durability requirement |
|---------------|---------|----------------------|
| `skills/` | Per-tenant skill packages | **Critical** — user-uploaded content |
| `gdpr-exports/` | GDPR data export archives | **High** — regulatory compliance |
| `evolution/` | Gene sharing artifacts | Medium |

### mc (MinIO Client) Setup

```bash
# Configure mc alias
mc alias set hermesx $MINIO_ENDPOINT $MINIO_ACCESS_KEY $MINIO_SECRET_KEY

# Verify connectivity
mc admin info hermesx
```

### Daily Mirror Backup

```bash
# Mirror entire bucket to backup location (incremental)
mc mirror hermesx/$MINIO_BUCKET /backup/minio/$(date +%Y%m%d)/ \
  --overwrite \
  --remove=false

# Or to a remote S3-compatible backup target
mc mirror hermesx/$MINIO_BUCKET backup-target/hermesx-backup/$(date +%Y%m%d)/
```

### Kubernetes CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: minio-backup
  namespace: hermesx
spec:
  schedule: "30 2 * * *"  # daily at 02:30 (after Redis backup)
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: minio/mc:latest
            command:
            - sh
            - -c
            - |
              mc alias set src $MINIO_ENDPOINT $MINIO_ACCESS_KEY $MINIO_SECRET_KEY
              mc alias set dst $BACKUP_ENDPOINT $BACKUP_ACCESS_KEY $BACKUP_SECRET_KEY
              mc mirror src/$MINIO_BUCKET dst/hermesx-backup/$(date +%Y%m%d)/ \
                --overwrite --remove=false
              echo "Backup completed: $(mc ls dst/hermesx-backup/$(date +%Y%m%d)/ | wc -l) objects"
            envFrom:
            - secretRef:
                name: hermesx-minio-secrets
            - secretRef:
                name: hermesx-backup-target
          restartPolicy: OnFailure
  successfulJobsHistoryLimit: 7
  failedJobsHistoryLimit: 3
```

### MinIO Restore

```bash
# Full bucket restore
mc mirror /backup/minio/YYYYMMDD/ hermesx/$MINIO_BUCKET/ --overwrite

# Single tenant restore
mc cp --recursive /backup/minio/YYYYMMDD/skills/tenant-id/ hermesx/$MINIO_BUCKET/skills/tenant-id/

# Verify object count
mc ls hermesx/$MINIO_BUCKET/skills/ --recursive | wc -l
```

### MinIO Versioning (recommended)

Enable bucket versioning for point-in-time recovery without external backups:

```bash
mc version enable hermesx/$MINIO_BUCKET

# List versions of a specific object
mc ls hermesx/$MINIO_BUCKET/skills/tenant-123/main.yaml --versions

# Restore specific version
mc cp hermesx/$MINIO_BUCKET/skills/tenant-123/main.yaml?versionId=<id> \
  hermesx/$MINIO_BUCKET/skills/tenant-123/main.yaml
```

### MinIO Erasure Coding

For self-hosted MinIO with durability requirements:

```bash
# Minimum 4 drives for erasure coding (tolerates 2 drive failures)
minio server /data{1...4}

# Verify healing status
mc admin heal hermesx --recursive
```

---

## Backup Verification

### Weekly Verification Script

```bash
#!/bin/bash
set -euo pipefail

echo "=== Redis Backup Verification ==="
LATEST_RDB=$(ls -t /backup/redis/*.rdb.gz 2>/dev/null | head -1)
if [ -z "$LATEST_RDB" ]; then
  echo "FAIL: No Redis backup found"
  exit 1
fi
AGE_HOURS=$(( ($(date +%s) - $(stat -f %m "$LATEST_RDB")) / 3600 ))
if [ "$AGE_HOURS" -gt 48 ]; then
  echo "WARN: Latest Redis backup is ${AGE_HOURS}h old"
fi
echo "OK: $LATEST_RDB (${AGE_HOURS}h old)"

echo ""
echo "=== MinIO Backup Verification ==="
LATEST_MINIO=$(ls -td /backup/minio/2* 2>/dev/null | head -1)
if [ -z "$LATEST_MINIO" ]; then
  echo "FAIL: No MinIO backup found"
  exit 1
fi
OBJ_COUNT=$(find "$LATEST_MINIO" -type f | wc -l)
echo "OK: $LATEST_MINIO ($OBJ_COUNT objects)"

# Compare object count with live
LIVE_COUNT=$(mc ls hermesx/$MINIO_BUCKET/ --recursive | wc -l)
DRIFT=$(( LIVE_COUNT - OBJ_COUNT ))
if [ "$DRIFT" -gt 100 ]; then
  echo "WARN: Backup has $DRIFT fewer objects than live"
fi

echo ""
echo "=== Backup Verification Complete ==="
```

---

## Retention Policy

| Data | Retention | Reason |
|------|-----------|--------|
| Redis RDB snapshots | 7 days | Debugging only |
| MinIO daily mirrors | 30 days | GDPR compliance window |
| MinIO GDPR exports | 90 days | Regulatory requirement |
| MinIO versioned objects | 60 days | Self-service rollback |

---

## Alerting

Configure alerts for backup failures:

```yaml
# Prometheus alerting rules
groups:
- name: backup
  rules:
  - alert: RedisBackupStale
    expr: time() - hermesx_backup_redis_last_success_timestamp > 172800
    labels:
      severity: warning
    annotations:
      summary: "Redis backup is older than 48h"

  - alert: MinIOBackupStale
    expr: time() - hermesx_backup_minio_last_success_timestamp > 172800
    labels:
      severity: warning
    annotations:
      summary: "MinIO backup is older than 48h"

  - alert: MinIOBackupDrift
    expr: hermesx_backup_minio_object_drift > 100
    labels:
      severity: warning
    annotations:
      summary: "MinIO backup has >100 fewer objects than live"
```
