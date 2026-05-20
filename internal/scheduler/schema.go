package scheduler

import (
	"time"

	"github.com/robfig/cron/v3"
)

// newCronParser returns a standard 5-field cron parser (no seconds).
func newCronParser() cron.Parser {
	return cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
}

// CronJobRun is the in-memory model for a single job execution record.
// The actual persistence target is the cron_job_runs table written directly via pgxpool.
type CronJobRun struct {
	ID          string
	CronJobID   string
	TenantID    string
	Status      string // pending | running | success | failed
	ScheduledAt time.Time
	StartedAt   time.Time
	FinishedAt  *time.Time
	DurationMs  *int64
	Result      string
	Error       string
	PodID       string
}

// cleanupStaleRunsSQL calls the SECURITY DEFINER function for cross-tenant stale run cleanup.
const cleanupStaleRunsSQL = `SELECT scheduler_cleanup_stale_runs($1)`

// insertRunSQL inserts a new run record with idempotency protection.
// ON CONFLICT DO NOTHING means a 0-rows result signals duplicate execution.
const insertRunSQL = `
INSERT INTO cron_job_runs
       (id, cron_job_id, tenant_id, status, scheduled_at, started_at, pod_id)
VALUES ($1, $2,          $3,        'running', $4,          now(),      $5)
ON CONFLICT (cron_job_id, scheduled_at) DO NOTHING
`

const updateRunSQL = `
UPDATE cron_job_runs
SET    status      = $2,
       finished_at = now(),
       duration_ms = $3,
       result      = left($4, 4096),
       error       = left($5, 1024)
WHERE  id = $1
`

const updateJobStatsSQL = `
UPDATE cron_jobs
SET    last_run_at      = now(),
       run_count        = run_count + 1,
       last_run_success = $2,
       last_run_error   = $3,
       next_run_at      = $4
WHERE  id = $1
`
