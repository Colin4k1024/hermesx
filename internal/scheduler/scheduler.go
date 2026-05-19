package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	redislock "github.com/go-co-op/gocron-redis-lock/v2"
	gocron "github.com/go-co-op/gocron/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/Colin4k1024/hermesx/internal/store"
)

// AgentRunner abstracts RunConversation for testability.
type AgentRunner interface {
	Run(ctx context.Context, tenantID, sessionID, prompt string) (string, error)
}

// ResultDeliverer pushes execution results back to the user's source platform.
type ResultDeliverer interface {
	Deliver(ctx context.Context, platform, chatID, jobName, result, errMsg string) error
}

// Config holds all SaasScheduler tunables.
type Config struct {
	PollInterval time.Duration // default 30s
	ExecTimeout  time.Duration // default 5min
	LockTTL      time.Duration // default 12min
	PodID        string        // defaults to hostname
}

func (c *Config) setDefaults() {
	if c.PollInterval <= 0 {
		c.PollInterval = 30 * time.Second
	}
	if c.ExecTimeout <= 0 {
		c.ExecTimeout = 5 * time.Minute
	}
	if c.LockTTL <= 0 {
		c.LockTTL = 12 * time.Minute
	}
	if c.PodID == "" {
		if h, err := os.Hostname(); err == nil {
			c.PodID = h
		} else {
			c.PodID = fmt.Sprintf("pod-%d", os.Getpid())
		}
	}
}

// registeredJob holds a gocron handle alongside the schedule expression for change detection.
type registeredJob struct {
	handle   gocron.Job
	schedule string
}

// SaasScheduler orchestrates distributed cron job execution for SaaS mode.
type SaasScheduler struct {
	cfg       Config
	sched     gocron.Scheduler
	store     store.CronJobStore
	pool      *pgxpool.Pool // direct pool for cron_job_runs writes (bypasses RLS)
	agent     AgentRunner
	deliverer ResultDeliverer // optional: pushes results to source platform
	mu        sync.Mutex
	jobs      map[string]registeredJob // jobID → handle + schedule
	started   bool
}

// New creates a SaasScheduler. Call Start to begin execution.
// deliverer is optional — pass nil to disable push delivery.
func New(cfg Config, jobStore store.CronJobStore, pool *pgxpool.Pool, rc redis.UniversalClient, agent AgentRunner, deliverer ResultDeliverer) (*SaasScheduler, error) {
	cfg.setDefaults()

	locker, err := redislock.NewRedisLocker(rc,
		redislock.WithTries(1), // no retry — competing pods skip
		redislock.WithExpiry(cfg.LockTTL),
	)
	if err != nil {
		return nil, fmt.Errorf("scheduler: create redis locker: %w", err)
	}

	sched, err := gocron.NewScheduler(
		gocron.WithDistributedLocker(locker),
	)
	if err != nil {
		return nil, fmt.Errorf("scheduler: create gocron scheduler: %w", err)
	}

	return &SaasScheduler{
		cfg:       cfg,
		sched:     sched,
		store:     jobStore,
		pool:      pool,
		agent:     agent,
		deliverer: deliverer,
		jobs:      make(map[string]registeredJob),
	}, nil
}

// Start boots the scheduler: cleans stale runs, syncs jobs, then polls on cfg.PollInterval.
func (s *SaasScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.mu.Unlock()

	if err := cleanupStaleRuns(ctx, s.pool, s.cfg.LockTTL); err != nil {
		slog.Warn("scheduler: stale run cleanup failed (non-fatal)", "error", err)
	}

	if err := s.syncOnce(ctx); err != nil {
		slog.Warn("scheduler: initial sync failed (non-fatal)", "error", err)
	}

	s.sched.Start()

	go s.pollLoop(ctx)
	slog.Info("scheduler: started", "pod", s.cfg.PodID, "poll_interval", s.cfg.PollInterval)
	return nil
}

// Stop gracefully shuts down the scheduler.
func (s *SaasScheduler) Stop() error {
	return s.sched.Shutdown()
}

func (s *SaasScheduler) pollLoop(ctx context.Context) {
	t := time.NewTicker(s.cfg.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.syncOnce(ctx); err != nil {
				slog.Warn("scheduler: sync error", "error", err)
			}
		}
	}
}
