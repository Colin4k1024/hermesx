package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

type myCronJobStore struct{ db *sql.DB }

func (s *myCronJobStore) Create(ctx context.Context, job *store.CronJob) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cron_jobs (id, tenant_id, name, prompt, schedule, deliver, enabled, model, next_run_at, metadata, source_platform, source_chat_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.TenantID, job.Name, job.Prompt, job.Schedule,
		nullStr(job.Deliver), job.Enabled, nullStr(job.Model), job.NextRunAt, nullStr(job.Metadata),
		nullStr(job.SourcePlatform), nullStr(job.SourceChatID))
	return err
}

func (s *myCronJobStore) Get(ctx context.Context, tenantID, jobID string) (*store.CronJob, error) {
	var j store.CronJob
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, prompt, schedule, COALESCE(deliver,''), enabled,
		        COALESCE(model,''), next_run_at, last_run_at, run_count, created_at, COALESCE(metadata,'{}'),
		        COALESCE(source_platform,''), COALESCE(source_chat_id,'')
		 FROM cron_jobs WHERE tenant_id = ? AND id = ?`,
		tenantID, jobID).Scan(
		&j.ID, &j.TenantID, &j.Name, &j.Prompt, &j.Schedule, &j.Deliver, &j.Enabled,
		&j.Model, &j.NextRunAt, &j.LastRunAt, &j.RunCount, &j.CreatedAt, &j.Metadata,
		&j.SourcePlatform, &j.SourceChatID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("cron job not found")
	}
	return &j, err
}

func (s *myCronJobStore) Update(ctx context.Context, job *store.CronJob) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE cron_jobs SET name=?, prompt=?, schedule=?, deliver=?, enabled=?, model=?, next_run_at=?
		 WHERE tenant_id = ? AND id = ?`,
		job.Name, job.Prompt, job.Schedule, nullStr(job.Deliver), job.Enabled, nullStr(job.Model),
		job.NextRunAt, job.TenantID, job.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("cron job not found")
	}
	return nil
}

func (s *myCronJobStore) Delete(ctx context.Context, tenantID, jobID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM cron_jobs WHERE tenant_id = ? AND id = ?`, tenantID, jobID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("cron job not found")
	}
	return nil
}

func (s *myCronJobStore) List(ctx context.Context, tenantID string) ([]*store.CronJob, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, prompt, schedule, COALESCE(deliver,''), enabled,
		        COALESCE(model,''), next_run_at, last_run_at, run_count, created_at, COALESCE(metadata,'{}'),
		        COALESCE(source_platform,''), COALESCE(source_chat_id,'')
		 FROM cron_jobs WHERE tenant_id = ? ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*store.CronJob
	for rows.Next() {
		var j store.CronJob
		if err := rows.Scan(&j.ID, &j.TenantID, &j.Name, &j.Prompt, &j.Schedule, &j.Deliver, &j.Enabled,
			&j.Model, &j.NextRunAt, &j.LastRunAt, &j.RunCount, &j.CreatedAt, &j.Metadata,
			&j.SourcePlatform, &j.SourceChatID); err != nil {
			return nil, err
		}
		jobs = append(jobs, &j)
	}
	return jobs, rows.Err()
}

// ListAllEnabled returns all enabled jobs across tenants for scheduler sync.
// tenant_sql_check:skip — intentional cross-tenant query.
func (s *myCronJobStore) ListAllEnabled(ctx context.Context) ([]*store.CronJob, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, prompt, schedule, COALESCE(deliver,''), enabled,
		        COALESCE(model,''), next_run_at, last_run_at, run_count, created_at, COALESCE(metadata,'{}'),
		        COALESCE(source_platform,''), COALESCE(source_chat_id,'')
		 FROM cron_jobs WHERE enabled = 1 ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*store.CronJob
	for rows.Next() {
		var j store.CronJob
		if err := rows.Scan(&j.ID, &j.TenantID, &j.Name, &j.Prompt, &j.Schedule, &j.Deliver, &j.Enabled,
			&j.Model, &j.NextRunAt, &j.LastRunAt, &j.RunCount, &j.CreatedAt, &j.Metadata,
			&j.SourcePlatform, &j.SourceChatID); err != nil {
			return nil, err
		}
		jobs = append(jobs, &j)
	}
	return jobs, rows.Err()
}

// ListDue is a cross-tenant scheduler query (intentional, no tenant filter).
// tenant_sql_check:skip — intentional cross-tenant query for scheduler dispatch.
func (s *myCronJobStore) ListDue(ctx context.Context, now time.Time) ([]*store.CronJob, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, prompt, schedule, COALESCE(deliver,''), enabled,
		        COALESCE(model,''), next_run_at, last_run_at, run_count, created_at, COALESCE(metadata,'{}'),
		        COALESCE(source_platform,''), COALESCE(source_chat_id,'')
		 FROM cron_jobs WHERE enabled = 1 AND next_run_at <= ?
		 ORDER BY next_run_at ASC LIMIT 100`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*store.CronJob
	for rows.Next() {
		var j store.CronJob
		if err := rows.Scan(&j.ID, &j.TenantID, &j.Name, &j.Prompt, &j.Schedule, &j.Deliver, &j.Enabled,
			&j.Model, &j.NextRunAt, &j.LastRunAt, &j.RunCount, &j.CreatedAt, &j.Metadata,
			&j.SourcePlatform, &j.SourceChatID); err != nil {
			return nil, err
		}
		jobs = append(jobs, &j)
	}
	return jobs, rows.Err()
}

func (s *myCronJobStore) ListRuns(ctx context.Context, tenantID, jobID string, limit int) ([]*store.CronJobRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, cron_job_id, tenant_id, status, scheduled_at, started_at,
		        finished_at, duration_ms, COALESCE(result,''), COALESCE(error,''), COALESCE(pod_id,''), created_at
		 FROM cron_job_runs WHERE tenant_id = ? AND cron_job_id = ?
		 ORDER BY started_at DESC LIMIT ?`, tenantID, jobID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*store.CronJobRun
	for rows.Next() {
		var r store.CronJobRun
		if err := rows.Scan(&r.ID, &r.CronJobID, &r.TenantID, &r.Status, &r.ScheduledAt, &r.StartedAt,
			&r.FinishedAt, &r.DurationMs, &r.Result, &r.Error, &r.PodID, &r.CreatedAt); err != nil {
			return nil, err
		}
		runs = append(runs, &r)
	}
	return runs, rows.Err()
}

var _ store.CronJobStore = (*myCronJobStore)(nil)
