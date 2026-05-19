# Deployment Context: SaaS Cron Scheduler Feedback

## Environment

| Environment | Purpose | Target |
|-------------|---------|--------|
| dev | Local development + integration test | `go run cmd/hermesx/main.go saas` |
| staging | Pre-prod validation with real Redis + PG | Docker Compose or K8s staging namespace |
| production | Multi-pod SaaS deployment | K8s production namespace |

## Prerequisites

| Dependency | Required | Notes |
|------------|----------|-------|
| PostgreSQL | Yes | Migrations auto-applied (v100-v104) |
| Redis | Yes | Distributed lock for gocron scheduler |
| Gateway Runner | Optional | Required for Layer 2 push delivery |
| Platform Adapters | Optional | Discord/Telegram/WebUI for push |

## Configuration

| Env Var | Purpose | Default |
|---------|---------|---------|
| `DATABASE_URL` | PG connection string | Required |
| `REDIS_URL` | Redis connection for scheduler lock | Required for cron |
| `HERMES_API_KEY` | API key for gateway runner | Optional (no push if missing) |
| `HERMES_API_PORT` | Gateway adapter port | 8081 |

No new environment variables introduced by this change.

## Schema Migrations

| DB | Version | Description |
|----|---------|-------------|
| PG | v100 | `cron_job_runs` table |
| PG | v101 | `cron_jobs` extensions (last_run_success, last_run_error) |
| PG | v102 | RLS policies for cron_job_runs |
| PG | v103 | `source_platform`, `source_chat_id` columns on cron_jobs |
| PG | v104 | Composite index `idx_cron_job_runs_tenant_job_started` |
| MySQL | v17 | `cron_job_runs` table |
| MySQL | v18 | `cron_jobs` extensions |
| MySQL | v19 | `source_platform`, `source_chat_id` columns |
| MySQL | v20 | Composite index `idx_cron_job_runs_tenant_job_started` |

All migrations are idempotent (IF NOT EXISTS / ON CONFLICT DO NOTHING).

## Deployment Entry

- **Primary**: `go build -o hermesx ./cmd/hermesx && ./hermesx saas`
- **Docker**: Standard Dockerfile multi-stage build
- **Rollback**: Redeploy previous binary; migrations are additive (new columns/indexes only)

## Recovery

| Trigger | Action | Verification |
|---------|--------|--------------|
| Delivery failures > 10% in 5min | Check gateway runner health | `GET /health/ready` |
| Scheduler not firing | Check Redis connectivity | Redis PING + slog scheduler logs |
| Migration failure on startup | Check PG connectivity and permissions | Migration logs at startup |

## Rollback

- **Binary rollback**: Deploy previous version. New columns (`source_platform`, `source_chat_id`) and index are harmless to old code (nullable, unused by old queries).
- **Schema rollback**: Not required for forward-compatible additive migrations. If needed: `DROP INDEX idx_cron_job_runs_tenant_job_started; ALTER TABLE cron_jobs DROP COLUMN source_platform, DROP COLUMN source_chat_id;`
- **Feature disable**: Set `REDIS_URL=""` to disable scheduler entirely (graceful degradation).

## Monitoring

| Signal | Source | Threshold |
|--------|--------|-----------|
| `scheduler: push delivery failed` | slog WARN | > 5/min |
| `scheduler: job execution failed` | slog WARN | Any |
| `scheduler: stale run cleanup` | slog INFO | Startup only |
| Cron job run duration | `cron_job_runs.duration_ms` | p99 > ExecTimeout (5min) |
