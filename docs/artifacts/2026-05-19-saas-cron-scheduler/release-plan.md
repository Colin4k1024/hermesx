# Release Plan: SaaS Cron Scheduler v2.4.0

## Release Information

| Field | Value |
|-------|-------|
| Version | v2.4.0 (SaaS Cron Feedback) |
| Date | 2026-05-19 |
| Owner | devops-engineer |
| Scope | Layer 1 (Pull) + Layer 2 (Push) + backlog fixes |
| Branch | main (uncommitted, pending commit) |

## Changes & Risk

| Component | Change | Risk |
|-----------|--------|------|
| `internal/scheduler/` | New package: gocron v2 + Redis lock, executor with delivery | MEDIUM — new runtime component |
| `internal/store/` | CronJobStore.ListRuns, CronJobRun type, source fields | LOW — additive interface extension |
| `internal/tools/cronjob.go` | Dual-path SaaS/CLI, list_runs action | LOW — existing tool, new action |
| `cmd/hermesx/saas.go` | schedulerPlatformDeliverer wiring | LOW — nil-safe, after runner init |
| DB migrations (PG v100-v104, MySQL v17-v20) | Tables + columns + indexes | LOW — all idempotent |

## Execution Steps

### Pre-release

1. [ ] Verify build: `go build ./...` — PASS
2. [ ] Verify vet: `go vet ./...` — PASS
3. [ ] Verify tests: `go test ./internal/scheduler/ -count=1` — 9/9 PASS
4. [ ] Review test-plan.md and launch-acceptance.md — Go
5. [ ] Commit changes to main branch

### Release

6. [ ] Build binary: `go build -o hermesx ./cmd/hermesx`
7. [ ] Deploy to staging with PG + Redis
8. [ ] Verify migrations applied (check `schema_migrations` table)
9. [ ] Create a test cron job via API with source_platform set
10. [ ] Verify `list_runs` returns execution history
11. [ ] Verify push delivery reaches test platform adapter
12. [ ] Deploy to production

### Post-release

13. [ ] Monitor slog for delivery failures (first 24h)
14. [ ] Verify composite index is used: `EXPLAIN ANALYZE` on ListRuns query
15. [ ] Confirm no regression in existing cron job execution

## Go / No-Go

| Criterion | Status |
|-----------|--------|
| Build green | PASS |
| Vet green | PASS |
| Tests green (9/9) | PASS |
| Security review | No blocking issues |
| Tenant isolation verified | PASS |
| Idempotency guard verified | PASS |
| Launch acceptance | Go |
| Rollback path documented | Yes |

## Decision

| Field | Value |
|-------|-------|
| Verdict | **Go** |
| Conditions | None |
| Observation window | 7 days — monitor delivery failure rate |
| Rollback trigger | Delivery failures > 10% or scheduler crashes |

## Rollback Plan

1. Redeploy previous binary (migrations are forward-compatible)
2. If scheduler instability: set `REDIS_URL=""` to disable
3. New DB columns/indexes are harmless to old code (nullable, unused)
4. No data migration rollback required

## Post-release Observation

| Item | Owner | Timeline |
|------|-------|----------|
| Delivery failure rate | devops | 7 days |
| Scheduler memory/goroutine count | devops | 3 days |
| ListRuns query latency | backend | 3 days |
| User feedback on push delivery | product | 14 days |
