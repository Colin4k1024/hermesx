package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Colin4k1024/hermesx/internal/store"
)

// execWithTenant runs fn inside a transaction with RLS tenant context set.
func (s *SaasScheduler) execWithTenant(ctx context.Context, tenantID string, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", tenantID); err != nil {
		tx.Rollback(ctx) //nolint:errcheck
		return fmt.Errorf("set tenant context: %w", err)
	}
	if err := fn(tx); err != nil {
		tx.Rollback(ctx) //nolint:errcheck
		return err
	}
	return tx.Commit(ctx)
}

// execute is the gocron task function. Called by gocron after winning the distributed lock.
func (s *SaasScheduler) execute(ctx context.Context, job *store.CronJob) {
	runID := uuid.New().String()
	scheduledAt := time.Now().Truncate(time.Minute)

	// Idempotent INSERT with tenant context for RLS compliance.
	var inserted bool
	err := s.execWithTenant(ctx, job.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, insertRunSQL,
			runID, job.ID, job.TenantID, scheduledAt, s.cfg.PodID)
		if err != nil {
			return err
		}
		inserted = tag.RowsAffected() > 0
		return nil
	})
	if err != nil {
		slog.Error("scheduler: insert run record failed", "job_id", job.ID, "error", err)
		return
	}
	if !inserted {
		slog.Debug("scheduler: duplicate execution detected, skipping", "job_id", job.ID)
		return
	}

	slog.Info("scheduler: executing job", "job_id", job.ID, "tenant_id", job.TenantID, "run_id", runID)
	start := time.Now()

	execCtx, cancel := context.WithTimeout(ctx, s.cfg.ExecTimeout)
	defer cancel()

	sessionID := fmt.Sprintf("cron-%s-%d", job.ID[:8], start.Unix())
	result, agentErr := s.agent.Run(execCtx, job.TenantID, sessionID, job.Prompt)

	elapsed := time.Since(start)
	durationMs := elapsed.Milliseconds()

	status := "success"
	errMsg := ""
	success := true
	if agentErr != nil {
		status = "failed"
		errMsg = agentErr.Error()
		success = false
		slog.Warn("scheduler: job execution failed", "job_id", job.ID, "run_id", runID, "error", agentErr)
	}

	// Update run record with tenant context.
	if err := s.execWithTenant(ctx, job.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, updateRunSQL, runID, status, durationMs, result, errMsg)
		return err
	}); err != nil {
		slog.Error("scheduler: update run record failed", "run_id", runID, "error", err)
	}

	// Update job stats with tenant context.
	nextRun := computeNextRun(job.Schedule, time.Now())
	var lastErrPtr *string
	if errMsg != "" {
		lastErrPtr = &errMsg
	}
	if err := s.execWithTenant(ctx, job.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, updateJobStatsSQL, job.ID, success, lastErrPtr, nextRun)
		return err
	}); err != nil {
		slog.Error("scheduler: update job stats failed", "job_id", job.ID, "error", err)
	}

	// Push result to user's source platform if configured.
	if s.deliverer != nil && job.SourcePlatform != "" && job.SourceChatID != "" && job.Deliver != "local" {
		deliverCtx, deliverCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer deliverCancel()
		if dErr := s.deliverer.Deliver(deliverCtx, job.SourcePlatform, job.SourceChatID, job.Name, result, errMsg); dErr != nil {
			slog.Warn("scheduler: push delivery failed", "job_id", job.ID, "platform", job.SourcePlatform, "error", dErr)
		}
	}

	slog.Info("scheduler: job completed", "job_id", job.ID, "run_id", runID,
		"status", status, "duration_ms", durationMs)
}

// computeNextRun uses the robfig/cron parser to calculate the next fire time.
func computeNextRun(cronExpr string, from time.Time) *time.Time {
	parser := newCronParser()
	sched, err := parser.Parse(cronExpr)
	if err != nil {
		slog.Warn("scheduler: cannot parse cron expr", "expr", cronExpr, "error", err)
		return nil
	}
	next := sched.Next(from)
	return &next
}
