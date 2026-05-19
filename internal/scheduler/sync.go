package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	gocron "github.com/go-co-op/gocron/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Colin4k1024/hermesx/internal/store"
)

// syncOnce fetches all enabled jobs from PG and reconciles the gocron scheduler.
// PG is the source of truth: new jobs are registered, changed jobs are re-registered,
// deleted/disabled jobs are removed.
func (s *SaasScheduler) syncOnce(ctx context.Context) error {
	jobs, err := s.store.ListAllEnabled(ctx)
	if err != nil {
		return fmt.Errorf("sync: list all enabled: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	pgIDs := make(map[string]struct{}, len(jobs))
	for _, j := range jobs {
		pgIDs[j.ID] = struct{}{}
	}

	// Remove jobs no longer enabled or deleted in PG.
	for id, reg := range s.jobs {
		if _, ok := pgIDs[id]; !ok {
			if err := s.sched.RemoveJob(reg.handle.ID()); err != nil {
				slog.Warn("scheduler: remove job failed", "job_id", id, "error", err)
			}
			delete(s.jobs, id)
		}
	}

	// Add or re-register jobs.
	for _, j := range jobs {
		if err := s.upsertJob(ctx, j); err != nil {
			slog.Warn("scheduler: upsert job failed", "job_id", j.ID, "error", err)
		}
	}

	return nil
}

// upsertJob registers a new gocron job or replaces an existing one.
// Must be called with s.mu held.
func (s *SaasScheduler) upsertJob(ctx context.Context, j *store.CronJob) error {
	existing, exists := s.jobs[j.ID]
	if exists {
		if existing.schedule == j.Schedule {
			return nil // unchanged, skip re-registration
		}
		if err := s.sched.RemoveJob(existing.handle.ID()); err != nil {
			slog.Warn("scheduler: remove stale job", "job_id", j.ID, "error", err)
		}
		delete(s.jobs, j.ID)
	}

	jobCopy := *j
	handle, err := s.sched.NewJob(
		gocron.CronJob(j.Schedule, false), // withSeconds=false
		gocron.NewTask(s.execute, ctx, &jobCopy),
		gocron.WithName(j.ID),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return fmt.Errorf("new gocron job %s: %w", j.ID, err)
	}

	s.jobs[j.ID] = registeredJob{handle: handle, schedule: j.Schedule}
	return nil
}

// cleanupStaleRuns marks running records older than lockTTL as failed.
// Called once at scheduler startup.
func cleanupStaleRuns(ctx context.Context, pool *pgxpool.Pool, lockTTL time.Duration) error {
	interval := fmt.Sprintf("%d seconds", int(lockTTL.Seconds()))
	tag, err := pool.Exec(ctx, cleanupStaleRunsSQL, interval)
	if err != nil {
		return fmt.Errorf("cleanup stale runs: %w", err)
	}
	if tag.RowsAffected() > 0 {
		slog.Info("scheduler: cleaned up stale runs", "count", tag.RowsAffected())
	}
	return nil
}
