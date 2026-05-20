# Distributed Scheduler Deployment & Testing Guide

> How to deploy the middleware required by the Cron Scheduler, start the scheduling service, and verify distributed execution correctness.

## Prerequisites

The distributed Cron Scheduler depends on the following middleware:

| Component | Version | Purpose |
|-----------|---------|---------|
| PostgreSQL | 16+ | Stores `cron_jobs`, `cron_job_runs` tables with RLS tenant isolation |
| Redis | 7+ | Distributed lock (multi-pod mutual exclusion) |

## Middleware Deployment

### Docker Compose (Recommended for Development)

Use `docker-compose.saas.yml` to start the complete infrastructure with one command:

```bash
# Create .env file
cat > .env <<'EOF'
POSTGRES_DB=hermesx
POSTGRES_USER=hermesx
POSTGRES_PASSWORD=hermesx
REDIS_URL=redis://redis:6379
MINIO_ACCESS_KEY=hermesx
MINIO_SECRET_KEY=hermesxpass
MINIO_BUCKET=hermes-skills
SAAS_ALLOWED_ORIGINS=*
HERMES_ACP_TOKEN=my-admin-token
HERMES_API_KEY=my-api-key
LLM_API_URL=http://host.docker.internal:11434/v1
LLM_API_KEY=ollama
LLM_MODEL=qwen3:30b
EOF

# Start all services
docker compose -f docker-compose.saas.yml up -d
```

After services start, PostgreSQL and Redis are automatically available for the Scheduler:

```
PostgreSQL  → localhost:5432
Redis       → localhost:6379
SaaS API    → localhost:18080
```

### Middleware Only

To start only the middleware (for local development debugging):

```bash
docker compose -f docker-compose.saas.yml up -d postgres redis
```

Verify middleware readiness:

```bash
# PostgreSQL
docker exec hermesx-pg pg_isready -U hermesx

# Redis
docker exec hermesx-redis redis-cli ping
```

### Kubernetes / Helm

Deploy to a cluster using the Helm Chart:

```bash
cd deploy/helm/hermesx

# Install (automatically creates PG + Redis dependencies)
helm install hermesx . \
  --set env.DATABASE_URL="postgres://hermesx:password@pg-host:5432/hermesx?sslmode=require" \
  --set env.REDIS_URL="redis://redis-host:6379"
```

Scheduler-related defaults in Helm `values.yaml`:

```yaml
env:
  REDIS_URL: "redis://redis:6379"
  # SCHEDULER_POLL_INTERVAL: "30s"   # optional override
  # SCHEDULER_EXEC_TIMEOUT: "5m"     # optional override
  # SCHEDULER_LOCK_TTL: "12m"        # optional override
```

### Database Table Auto-Migration

Tables required by the Scheduler are automatically created when `hermes saas-api` starts (migrations 100-106):

- `cron_jobs` — Cron job definitions
- `cron_job_runs` — Execution records (with idempotent unique constraint)
- RLS read/write policies (SELECT / INSERT / UPDATE / DELETE)
- `scheduler_cleanup_stale_runs()` SECURITY DEFINER function

No manual DDL execution required.

## Startup & Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `REDIS_URL` | Yes (to enable scheduling) | - | Redis connection string |
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `SCHEDULER_POLL_INTERVAL` | No | `30s` | Polling interval for syncing jobs from PG |
| `SCHEDULER_EXEC_TIMEOUT` | No | `5m` | Single task execution timeout |
| `SCHEDULER_LOCK_TTL` | No | `12m` | Redis distributed lock TTL |

**Key constraint**: `SCHEDULER_LOCK_TTL` must be greater than the longest task execution time, otherwise premature lock release may cause duplicate execution.

### Startup Process

```bash
# Method 1: Direct binary startup
export DATABASE_URL="postgres://hermesx:hermesx@localhost:5432/hermesx?sslmode=disable"
export REDIS_URL="redis://localhost:6379"
export HERMES_ACP_TOKEN="my-admin-token"
./hermesx saas-api

# Method 2: Docker Compose
docker compose -f docker-compose.saas.yml up -d
```

You should see in the startup logs:

```
level=INFO msg="scheduler started" poll_interval=30s
```

If Redis is unavailable, a warning is logged but the service still starts normally (Scheduler functionality unavailable):

```
level=WARN msg="scheduler disabled: redis unavailable"
```

### Health Check Confirmation

```bash
# Readiness probe (includes Redis connectivity check)
curl -s http://localhost:18080/health/ready | jq .

# Expected output
{
  "status": "ok",
  "checks": {
    "database": "ok",
    "redis": "ok"
  }
}
```

If `redis` status is `"error"`, the Scheduler failed to start.

### Multi-Replica Deployment

The Scheduler natively supports multi-pod deployment with no additional configuration:

```bash
# Verify with multi-replica compose (3 replicas + Nginx LB)
cd deploy
docker compose -f docker-compose.multi-replica.yml up -d
```

Each pod independently runs pollLoop, competing for the Redis lock:
- The pod that acquires the lock executes the task
- Pods that fail to acquire the lock skip (`redislock.WithTries(1)` no retry)
- Idempotent constraint `UNIQUE(cron_job_id, scheduled_at)` provides dedup fallback

## Testing

### Unit Tests

```bash
# Run scheduler package tests
go test ./internal/scheduler/ -v -count=1

# Expected output
=== RUN   TestSchedulerNew
=== RUN   TestSchedulerSync
...
--- PASS: ...
PASS
ok      github.com/hermesx/internal/scheduler   0.XXXs
```

Tests use in-memory mocks (`mockCronJobStore`, `mockAgentRunner`), no external dependencies required.

### Integration Tests

Integration tests require real PostgreSQL and Redis:

```bash
# Method 1: One-command run (start infra → test → cleanup)
make test-integration

# Method 2: Step by step
make test-infra-up                                    # Start PG:5433 + Redis:6380 + MinIO:9002
go test -tags=integration ./tests/integration/...     # Run tests
make test-infra-down                                  # Cleanup
```

`docker-compose.test.yml` uses isolated ports and tmpfs volumes, without affecting the development environment:

| Service | Test Port |
|---------|-----------|
| PostgreSQL | 5433 |
| Redis | 6380 |
| MinIO | 9002 |

### Manual Functional Verification

Create a cron job through the Agent's `cronjob` tool and observe execution:

```bash
# 1. Start services
docker compose -f docker-compose.saas.yml up -d

# 2. Create a per-minute test task via Chat API
curl -X POST http://localhost:18080/v1/chat/completions \
  -H "Authorization: Bearer my-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3:30b",
    "messages": [{"role": "user", "content": "Create a cron job that runs every minute and replies with the current time"}]
  }'

# 3. Wait 1-2 minutes, check execution records
docker exec hermesx-pg psql -U hermesx -c \
  "SELECT id, status, scheduled_at, duration_ms FROM cron_job_runs ORDER BY created_at DESC LIMIT 5;"

# 4. Check Scheduler logs
docker logs hermesx-saas 2>&1 | grep -i "cron\|scheduler" | tail -10
```

### Multi-Pod Mutual Exclusion Verification

Verify that the distributed lock works correctly:

```bash
# 1. Start 3 replicas
cd deploy
docker compose -f docker-compose.multi-replica.yml up -d

# 2. Create a short-interval task (via any replica)
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer test-secret-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3:30b",
    "messages": [{"role": "user", "content": "Create a cron job that reports the current time every minute"}]
  }'

# 3. Observe each pod's logs, confirm only one pod executes
docker logs hermesx-mr-1 2>&1 | grep "cron job executed" | wc -l
docker logs hermesx-mr-2 2>&1 | grep "cron job executed" | wc -l
docker logs hermesx-mr-3 2>&1 | grep "cron job executed" | wc -l

# 4. Confirm no duplicate records in cron_job_runs
docker exec hermesx-mr-pg psql -U hermesx -c \
  "SELECT cron_job_id, scheduled_at, COUNT(*) FROM cron_job_runs GROUP BY 1,2 HAVING COUNT(*) > 1;"
# Expected: 0 rows (no duplicates)

# 5. Cleanup
docker compose -f docker-compose.multi-replica.yml down -v
```

### Fault Recovery Verification

```bash
# Simulate Redis failure
docker stop hermesx-redis

# Observe service logs (Scheduler stops scheduling, but API remains available)
curl -s http://localhost:18080/health/ready | jq .
# redis: "error", but HTTP 200 (service doesn't exit)

# Recover Redis
docker start hermesx-redis

# Scheduler auto-recovers on next poll cycle (≤30s)
docker logs hermesx-saas 2>&1 | grep "scheduler" | tail -5
```

## Operations Notes

| Scenario | Resolution |
|----------|-----------|
| Redis temporarily unavailable | Scheduler pauses, API works normally, auto-recovers when Redis returns |
| Pod abnormal exit | Other pods take over after lock TTL expires; `scheduler_cleanup_stale_runs()` marks timed-out records as failed |
| Task backlog | Increase `SCHEDULER_EXEC_TIMEOUT`, or split into smaller-granularity prompts |
| Lock TTL too short | Increase `SCHEDULER_LOCK_TTL` (must exceed longest task duration) |
| PG and gocron out of sync | Wait for next poll cycle (30s) for automatic alignment |

## Related Documentation

- [Architecture Overview](architecture.en.md) — Scheduler system design
- [Configuration Guide](configuration.en.md) — Complete environment variable reference
- [Database](database.en.md) — cron_jobs / cron_job_runs table schema
- [Deployment Guide](deployment.en.md) — Docker / Helm / Kind deployment
