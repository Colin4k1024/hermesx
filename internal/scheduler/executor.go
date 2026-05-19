package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Colin4k1024/hermesx/internal/store"
)

// execute is the gocron task function. Called by gocron after winning the distributed lock.
func (s *SaasScheduler) execute(ctx context.Context, job *store.CronJob) {
	runID := uuid.New().String()
	scheduledAt := time.Now().Truncate(time.Minute) // coarse trigger time for idempotency key

	// Idempotent INSERT — zero rows means another pod already started this run.
	tag, err := s.pool.Exec(ctx, insertRunSQL,
		runID, job.ID, job.TenantID, scheduledAt, s.cfg.PodID)
	if err != nil {
		slog.Error("scheduler: insert run record failed", "job_id", job.ID, "error", err)
		return
	}
	if tag.RowsAffected() == 0 {
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

	// Update run record.
	if _, err := s.pool.Exec(ctx, updateRunSQL, runID, status, durationMs, result, errMsg); err != nil {
		slog.Error("scheduler: update run record failed", "run_id", runID, "error", err)
	}

	// Update job stats and compute next_run_at.
	nextRun := computeNextRun(job.Schedule, time.Now())
	var lastErrPtr *string
	if errMsg != "" {
		lastErrPtr = &errMsg
	}
	if _, err := s.pool.Exec(ctx, updateJobStatsSQL,
		job.ID, success, lastErrPtr, nextRun); err != nil {
		slog.Error("scheduler: update job stats failed", "job_id", job.ID, "error", err)
	}

	// Push result to user's source platform if configured.
	if s.deliverer != nil && job.SourcePlatform != "" && job.SourceChatID != "" && job.Deliver != "local" {
		deliverCtx, deliverCancel := context.WithTimeout(context.Background(), 30*time.Second)
		if dErr := s.deliverer.Deliver(deliverCtx, job.SourcePlatform, job.SourceChatID, job.Name, result, errMsg); dErr != nil {
			slog.Warn("scheduler: push delivery failed", "job_id", job.ID, "platform", job.SourcePlatform, "error", dErr)
		}
		deliverCancel()
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
