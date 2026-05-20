package pg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgCronJobStore struct {
	pool *pgxpool.Pool
}

func (s *pgCronJobStore) Create(ctx context.Context, job *store.CronJob) error {
	return withTenantTx(ctx, s.pool, job.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO cron_jobs (id, tenant_id, name, prompt, schedule, deliver, enabled, model, next_run_at, metadata, source_platform, source_chat_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, COALESCE($10::jsonb, '{}'), $11, $12)`,
			job.ID, job.TenantID, job.Name, job.Prompt, job.Schedule,
			job.Deliver, job.Enabled, job.Model, job.NextRunAt, job.Metadata,
			job.SourcePlatform, job.SourceChatID)
		if err != nil {
			return fmt.Errorf("pg create cron job: %w", err)
		}
		return nil
	})
}

func (s *pgCronJobStore) Get(ctx context.Context, tenantID, jobID string) (*store.CronJob, error) {
	var j store.CronJob
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, prompt, schedule, COALESCE(deliver,''), enabled,
		        COALESCE(model,''), next_run_at, last_run_at, run_count, created_at, COALESCE(metadata::text,'{}'),
		        COALESCE(source_platform,''), COALESCE(source_chat_id,'')
		 FROM cron_jobs WHERE tenant_id = $1 AND id = $2`,
		tenantID, jobID).Scan(
		&j.ID, &j.TenantID, &j.Name, &j.Prompt, &j.Schedule, &j.Deliver, &j.Enabled,
		&j.Model, &j.NextRunAt, &j.LastRunAt, &j.RunCount, &j.CreatedAt, &j.Metadata,
		&j.SourcePlatform, &j.SourceChatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("pg get cron job: %w", err)
	}
	return &j, nil
}

func (s *pgCronJobStore) Update(ctx context.Context, job *store.CronJob) error {
	return withTenantTx(ctx, s.pool, job.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE cron_jobs SET name=$3, prompt=$4, schedule=$5, deliver=$6, enabled=$7, model=$8, next_run_at=$9
			 WHERE tenant_id = $1 AND id = $2`,
			job.TenantID, job.ID, job.Name, job.Prompt, job.Schedule,
			job.Deliver, job.Enabled, job.Model, job.NextRunAt)
		if err != nil {
			return fmt.Errorf("pg update cron job: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return store.ErrNotFound
		}
		return nil
	})
}

func (s *pgCronJobStore) Delete(ctx context.Context, tenantID, jobID string) error {
	return withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`DELETE FROM cron_jobs WHERE tenant_id = $1 AND id = $2`,
			tenantID, jobID)
		if err != nil {
			return fmt.Errorf("pg delete cron job: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return store.ErrNotFound
		}
		return nil
	})
}

func (s *pgCronJobStore) List(ctx context.Context, tenantID string) ([]*store.CronJob, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, name, prompt, schedule, COALESCE(deliver,''), enabled,
		        COALESCE(model,''), next_run_at, last_run_at, run_count, created_at, COALESCE(metadata::text,'{}'),
		        COALESCE(source_platform,''), COALESCE(source_chat_id,'')
		 FROM cron_jobs WHERE tenant_id = $1 ORDER BY created_at DESC`,
		tenantID)
	if err != nil {
		return nil, fmt.Errorf("pg list cron jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*store.CronJob
	for rows.Next() {
		var j store.CronJob
		if err := rows.Scan(&j.ID, &j.TenantID, &j.Name, &j.Prompt, &j.Schedule, &j.Deliver, &j.Enabled,
			&j.Model, &j.NextRunAt, &j.LastRunAt, &j.RunCount, &j.CreatedAt, &j.Metadata,
			&j.SourcePlatform, &j.SourceChatID); err != nil {
			return nil, fmt.Errorf("pg scan cron job: %w", err)
		}
		jobs = append(jobs, &j)
	}
	return jobs, rows.Err()
}

// ListAllEnabled returns all enabled jobs across tenants for scheduler sync.
// tenant_sql_check:skip — intentional cross-tenant query.
func (s *pgCronJobStore) ListAllEnabled(ctx context.Context) ([]*store.CronJob, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, name, prompt, schedule, COALESCE(deliver,''), enabled,
		        COALESCE(model,''), next_run_at, last_run_at, run_count, created_at, COALESCE(metadata::text,'{}'),
		        COALESCE(source_platform,''), COALESCE(source_chat_id,'')
		 FROM cron_jobs WHERE enabled = true ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("pg list all enabled cron jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*store.CronJob
	for rows.Next() {
		var j store.CronJob
		if err := rows.Scan(&j.ID, &j.TenantID, &j.Name, &j.Prompt, &j.Schedule, &j.Deliver, &j.Enabled,
			&j.Model, &j.NextRunAt, &j.LastRunAt, &j.RunCount, &j.CreatedAt, &j.Metadata,
			&j.SourcePlatform, &j.SourceChatID); err != nil {
			return nil, fmt.Errorf("pg scan cron job: %w", err)
		}
		jobs = append(jobs, &j)
	}
	return jobs, rows.Err()
}

// ListDue returns all due cron jobs across tenants (system-level scheduler query).
// tenant_sql_check:skip — intentional cross-tenant query for scheduler dispatch.
func (s *pgCronJobStore) ListDue(ctx context.Context, now time.Time) ([]*store.CronJob, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, name, prompt, schedule, COALESCE(deliver,''), enabled,
		        COALESCE(model,''), next_run_at, last_run_at, run_count, created_at, COALESCE(metadata::text,'{}'),
		        COALESCE(source_platform,''), COALESCE(source_chat_id,'')
		 FROM cron_jobs WHERE enabled = true AND next_run_at <= $1
		 ORDER BY next_run_at ASC`, now)
	if err != nil {
		return nil, fmt.Errorf("pg list due cron jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*store.CronJob
	for rows.Next() {
		var j store.CronJob
		if err := rows.Scan(&j.ID, &j.TenantID, &j.Name, &j.Prompt, &j.Schedule, &j.Deliver, &j.Enabled,
			&j.Model, &j.NextRunAt, &j.LastRunAt, &j.RunCount, &j.CreatedAt, &j.Metadata,
			&j.SourcePlatform, &j.SourceChatID); err != nil {
			return nil, fmt.Errorf("pg scan due cron job: %w", err)
		}
		jobs = append(jobs, &j)
	}
	return jobs, rows.Err()
}

func (s *pgCronJobStore) ListRuns(ctx context.Context, tenantID, jobID string, limit int) ([]*store.CronJobRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, cron_job_id, tenant_id, status, scheduled_at, started_at,
		        finished_at, duration_ms, COALESCE(result,''), COALESCE(error,''), COALESCE(pod_id,''), created_at
		 FROM cron_job_runs WHERE tenant_id = $1 AND cron_job_id = $2
		 ORDER BY started_at DESC LIMIT $3`, tenantID, jobID, limit)
	if err != nil {
		return nil, fmt.Errorf("pg list cron runs: %w", err)
	}
	defer rows.Close()

	var runs []*store.CronJobRun
	for rows.Next() {
		var r store.CronJobRun
		if err := rows.Scan(&r.ID, &r.CronJobID, &r.TenantID, &r.Status, &r.ScheduledAt, &r.StartedAt,
			&r.FinishedAt, &r.DurationMs, &r.Result, &r.Error, &r.PodID, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("pg scan cron run: %w", err)
		}
		runs = append(runs, &r)
	}
	return runs, rows.Err()
}

var _ store.CronJobStore = (*pgCronJobStore)(nil)
