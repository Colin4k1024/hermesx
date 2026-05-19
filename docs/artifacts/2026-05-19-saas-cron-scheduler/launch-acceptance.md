# Launch Acceptance: SaaS Cron Scheduler Feedback Mechanism

## Overview

| Field | Value |
|-------|-------|
| Object | SaaS Cron Job Feedback — Layer 1 (Pull) + Layer 2 (Push) |
| Date | 2026-05-19 |
| Reviewer | QA (inline) |
| Status | **Go** |

## Acceptance Scope

### Business

- Users can query execution history of their cron jobs via `list_runs` tool action
- Cron job execution results are automatically pushed back to the user's source platform (Discord, Telegram, WebUI, etc.)
- Users who created jobs from "local" mode are not pushed — they use pull only
- Tenant isolation maintained: users only see their own job runs

### Technical

- `CronJobStore.ListRuns()` implemented across all backends (PG, MySQL, SQLite/noop, traced)
- `ResultDeliverer` interface abstracts platform-specific push logic
- `schedulerPlatformDeliverer` bridges scheduler → gateway adapter → platform
- Schema migrations add `source_platform`, `source_chat_id` columns (PG v103, MySQL v19)
- `cron_job_runs` table tracks full execution history with idempotency guard
- Build / vet / test all green (9/9 scheduler tests pass)

### Non-functional

- Delivery timeout: 30s hard cap per push
- Result payload: capped at 2000 chars for push, 4096 chars in DB
- ListRuns limit: capped at 100, default 20
- Error logged at WARN; delivery failure does not block job completion

## Evidence

| Criterion | Result |
|-----------|--------|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| `go test ./internal/scheduler/` | 9/9 PASS |
| SQL idempotency guard present | Verified (ON CONFLICT DO NOTHING) |
| Tenant isolation in ListRuns | Verified (WHERE tenant_id = $1 AND cron_job_id = $2) |
| Nil-safe deliverer handling | Verified (nil runner → debug log, return nil) |
| Source tracking on job creation | Verified (saasCreateCronJob captures platform + chat_id) |
| Delivery condition gating | Verified (requires non-nil deliverer + source_platform + source_chat_id + deliver != "local") |
| Migration idempotent (IF NOT EXISTS) | Verified (PG DO $$ block, MySQL ADD COLUMN IF NOT EXISTS) |

## Risk Judgment

### Satisfied

- [x] Tenant isolation
- [x] Idempotent execution (dedup by cron_job_id + scheduled_at)
- [x] Bounded payloads (DB + push)
- [x] Graceful degradation (delivery failure → WARN log, does not affect job status)
- [x] Proper shutdown ordering (scheduler stop before pool close)
- [x] No secrets exposure in logs or responses

### Accepted Risk

- UTF-8 unsafe byte truncation in push message (cosmetic, non-blocking)
- Sync re-registers all jobs every cycle (gocron v2 limitation, documented)
- No composite index on `cron_job_runs(tenant_id, cron_job_id, started_at)` — acceptable at current scale

### Blocking

None.

## Conclusion

| Decision | Approved |
|----------|----------|
| Go / No-Go | **Go** |
| Conditions | None |
| Observation | Monitor delivery failure rate in WARN logs during first week |

### Confirmation

- Reviewer: QA (inline review)
- Date: 2026-05-19
- Verdict: Release approved. All quality gates pass, no blocking issues.
