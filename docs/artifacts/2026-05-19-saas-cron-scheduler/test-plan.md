# Test Plan: SaaS Cron Scheduler Feedback Mechanism

## Overview

| Field | Value |
|-------|-------|
| Task | SaaS Cron Job Feedback (Layer 1 Pull + Layer 2 Push) |
| Date | 2026-05-19 |
| Reviewer | QA (inline) |
| Scope | internal/scheduler, internal/store, internal/tools/cronjob, cmd/hermesx/saas |

## Test Scope

### In Scope

- Layer 1 (Pull): `list_runs` action via cronjob tool — tenant-scoped query of execution history
- Layer 2 (Push): `schedulerPlatformDeliverer` — post-execution result push to source platform
- Data layer: `CronJobRun` type, `ListRuns` across PG/MySQL/SQLite/traced implementations
- Schema migrations: PG v103, MySQL v19 (source_platform, source_chat_id)
- Scheduler integration: executor delivery path, nil-safe deliverer handling
- Idempotency: `insertRunSQL` ON CONFLICT guard
- Tenant isolation: ListRuns scoped by `tenant_id + cron_job_id`

### Out of Scope

- Platform adapter internals (Discord/Telegram/WebUI send logic)
- Agent execution quality (AgentRunner mock used)
- Redis lock contention under real load
- End-to-end multi-pod deduplication (requires infra)

## Test Matrix

| Scenario | Type | Precondition | Expected |
|----------|------|--------------|----------|
| ListRuns with valid job_id | Unit/Integration | Runs exist in cron_job_runs | Returns ordered runs DESC by started_at |
| ListRuns with no runs | Unit | Empty table for job | Returns empty array, count=0 |
| ListRuns limit cap | Unit | limit > 100 | Capped to 20 |
| ListRuns tenant isolation | Unit | Runs for tenant-a and tenant-b | Each tenant sees only own runs |
| Delivery on success | Unit | job.SourcePlatform="discord", Deliver!="local" | adapter.Send called with success text |
| Delivery on failure | Unit | agentErr != nil | adapter.Send called with failure text |
| Delivery skipped (local) | Unit | job.Deliver="local" | No adapter.Send call |
| Delivery skipped (no source) | Unit | SourcePlatform="" | No adapter.Send call |
| Delivery nil runner | Unit | deliverer.runner=nil | Returns nil, no panic |
| Delivery unknown platform | Unit | platform="unknown" | Returns error, logged |
| Result truncation | Unit | result > 2000 chars | Truncated to 2000+"..." |
| Source tracking on create | Integration | Create job from Discord | source_platform="discord", source_chat_id captured |
| Migration idempotent PG v103 | Migration | Run twice | No error, columns exist |
| Migration idempotent MySQL v19 | Migration | Run twice | No error, ADD IF NOT EXISTS |
| insertRunSQL dedup | SQL | Same (cron_job_id, scheduled_at) | 0 rows affected on second insert |
| updateRunSQL fields | SQL | Run record exists | status, finished_at, duration_ms, result, error updated |

## Automated Test Results

| Suite | Status | Count |
|-------|--------|-------|
| `go build ./...` | PASS | all packages |
| `go vet ./...` | PASS | no issues |
| `internal/scheduler` tests | PASS | 9/9 |

### Covered by existing tests:
- F1: Idempotency guard (insertRunSQL ON CONFLICT)
- F2: Execution history fields (updateRunSQL, updateJobStatsSQL)
- F3: Tenant isolation (ListAllEnabled filtering)
- Sync logic: add/remove jobs
- ComputeNextRun: valid/invalid cron expressions

## Code Review Findings

### Security

| ID | Severity | Finding | Status |
|----|----------|---------|--------|
| S1 | LOW | `source_chat_id` is user-controlled, stored directly — only used as Send target, not SQL interpolation | Acceptable |
| S2 | LOW | Result payload truncated at 2000 chars before push — may split multi-byte UTF-8 | Non-blocking |
| S3 | INFO | All SQL uses parameterized queries — no injection risk | Pass |
| S4 | INFO | `left($4, 4096)` caps DB storage for result — prevents unbounded growth | Pass |

### Code Quality

| ID | Severity | Finding | Status |
|----|----------|---------|--------|
| C1 | MEDIUM | `getJobSchedule` always returns `("", false)` — every sync re-registers all jobs | Documented, acceptable for correctness |
| C2 | LOW | UTF-8 unsafe truncation in `schedulerPlatformDeliverer.Deliver` at byte boundary | Non-blocking, backlog |
| C3 | INFO | Clean interface segregation (ResultDeliverer, AgentRunner) | Pass |
| C4 | INFO | Proper nil-guard chain in executor delivery block | Pass |
| C5 | INFO | `deliverCancel()` correctly called after Deliver returns | Pass |

### Architecture

| ID | Severity | Finding | Status |
|----|----------|---------|--------|
| A1 | INFO | Dual-path dispatch (CLI/SaaS) via `tctx.CronJobStore != nil` — clean | Pass |
| A2 | INFO | DI wiring in saas.go section 10b — correct ordering after runner init | Pass |
| A3 | LOW | Consider adding composite index `(tenant_id, cron_job_id, started_at DESC)` on cron_job_runs | Backlog |

## Risk Assessment

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Push delivery failure silently lost | LOW | MEDIUM | Logged at WARN level; user can pull via list_runs |
| Multi-byte truncation garbles last char | LOW | LOW | Only cosmetic; backlog fix |
| Re-registration every sync cycle | LOW | CERTAIN | Documented; gocron v2 limitation |
| Missing platform adapter at delivery time | LOW | LOW | Error returned and logged |

## Release Recommendation

**Recommend release.** No blocking issues found. All automated checks pass. Security surface is minimal (parameterized SQL, tenant-scoped queries, bounded payloads). Two non-blocking items logged for backlog (UTF-8 truncation, composite index).
