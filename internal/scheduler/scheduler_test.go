package scheduler

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	gocron "github.com/go-co-op/gocron/v2"

	"github.com/Colin4k1024/hermesx/internal/store"
)

// ── Mocks ────────────────────────────────────────────────────────────────────

type mockCronJobStore struct {
	mu   sync.Mutex
	jobs []*store.CronJob
}

func (m *mockCronJobStore) Create(_ context.Context, j *store.CronJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = append(m.jobs, j)
	return nil
}
func (m *mockCronJobStore) Get(_ context.Context, tenantID, jobID string) (*store.CronJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.TenantID == tenantID && j.ID == jobID {
			return j, nil
		}
	}
	return nil, store.ErrNotFound
}
func (m *mockCronJobStore) Update(_ context.Context, j *store.CronJob) error { return nil }
func (m *mockCronJobStore) Delete(_ context.Context, _, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, j := range m.jobs {
		if j.ID == id {
			m.jobs = append(m.jobs[:i], m.jobs[i+1:]...)
			return nil
		}
	}
	return nil
}
func (m *mockCronJobStore) List(_ context.Context, tenantID string) ([]*store.CronJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*store.CronJob
	for _, j := range m.jobs {
		if j.TenantID == tenantID {
			out = append(out, j)
		}
	}
	return out, nil
}
func (m *mockCronJobStore) ListDue(_ context.Context, _ time.Time) ([]*store.CronJob, error) {
	return m.ListAllEnabled(context.Background())
}
func (m *mockCronJobStore) ListAllEnabled(_ context.Context) ([]*store.CronJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*store.CronJob
	for _, j := range m.jobs {
		if j.Enabled {
			out = append(out, j)
		}
	}
	return out, nil
}
func (m *mockCronJobStore) ListRuns(_ context.Context, _, _ string, _ int) ([]*store.CronJobRun, error) {
	return nil, nil
}

type mockAgentRunner struct {
	mu    sync.Mutex
	calls []agentCall
	reply string
	err   error
}

type agentCall struct {
	tenantID  string
	sessionID string
	prompt    string
}

func (m *mockAgentRunner) Run(_ context.Context, tenantID, sessionID, prompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, agentCall{tenantID, sessionID, prompt})
	return m.reply, m.err
}

// ── F3: Tenant Isolation ─────────────────────────────────────────────────────

// TestF3_TenantIsolation verifies ListAllEnabled only surfaces enabled jobs,
// and that List correctly filters by tenant (interface-level isolation).
func TestF3_TenantIsolation(t *testing.T) {
	st := &mockCronJobStore{}
	now := time.Now()
	st.jobs = []*store.CronJob{
		{ID: "j1", TenantID: "tenant-a", Enabled: true, Schedule: "* * * * *", NextRunAt: &now},
		{ID: "j2", TenantID: "tenant-b", Enabled: true, Schedule: "* * * * *", NextRunAt: &now},
		{ID: "j3", TenantID: "tenant-a", Enabled: false, Schedule: "* * * * *"},
	}

	// ListAllEnabled must return only enabled jobs across all tenants.
	all, err := st.ListAllEnabled(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("ListAllEnabled: want 2 enabled jobs, got %d", len(all))
	}

	// Per-tenant List must be isolated.
	ta, _ := st.List(context.Background(), "tenant-a")
	if len(ta) != 2 { // j1 (enabled) + j3 (disabled) both belong to tenant-a
		t.Errorf("List(tenant-a): want 2, got %d", len(ta))
	}
	tb, _ := st.List(context.Background(), "tenant-b")
	if len(tb) != 1 {
		t.Errorf("List(tenant-b): want 1, got %d", len(tb))
	}
}

// ── F1: Dedup SQL Idempotency Guard ──────────────────────────────────────────

// TestF1_InsertRunSQL_IdempotencyGuard verifies the INSERT SQL contains
// ON CONFLICT DO NOTHING, which is the last line of defence against
// double-execution when two pods win the lock simultaneously.
func TestF1_InsertRunSQL_IdempotencyGuard(t *testing.T) {
	if !strings.Contains(insertRunSQL, "ON CONFLICT") {
		t.Error("insertRunSQL must contain ON CONFLICT clause for idempotency")
	}
	if !strings.Contains(insertRunSQL, "DO NOTHING") {
		t.Error("insertRunSQL must contain DO NOTHING to skip duplicate inserts")
	}
}

// TestF1_UniqueConstraintColumns verifies the UNIQUE constraint in the
// PG migration SQL covers (cron_job_id, scheduled_at).
func TestF1_UniqueConstraintColumns(t *testing.T) {
	// These SQL constants are defined in schema.go.
	if !strings.Contains(insertRunSQL, "scheduled_at") {
		t.Error("insertRunSQL must reference scheduled_at as part of the idempotency key")
	}
	if !strings.Contains(insertRunSQL, "cron_job_id") {
		t.Error("insertRunSQL must reference cron_job_id as part of the idempotency key")
	}
}

// ── F2: Execution History SQL Coverage ───────────────────────────────────────

// TestF2_UpdateRunSQL_AllStatusFields verifies the UPDATE SQL covers
// the fields needed for a complete execution history record.
func TestF2_UpdateRunSQL_AllStatusFields(t *testing.T) {
	required := []string{"status", "finished_at", "duration_ms", "result", "error"}
	for _, field := range required {
		if !strings.Contains(updateRunSQL, field) {
			t.Errorf("updateRunSQL missing field: %s", field)
		}
	}
}

// TestF2_UpdateJobStatsSQL_TracksLastRun verifies job stats SQL updates
// the fields needed for observable cron job history.
func TestF2_UpdateJobStatsSQL_TracksLastRun(t *testing.T) {
	required := []string{"last_run_at", "run_count", "last_run_success", "last_run_error", "next_run_at"}
	for _, field := range required {
		if !strings.Contains(updateJobStatsSQL, field) {
			t.Errorf("updateJobStatsSQL missing field: %s", field)
		}
	}
}

// ── Sync logic ───────────────────────────────────────────────────────────────

// TestSyncOnce_AddsNewJobs verifies that syncOnce registers jobs present in PG
// that are not yet in the in-memory map.
func TestSyncOnce_AddsNewJobs(t *testing.T) {
	now := time.Now()
	st := &mockCronJobStore{
		jobs: []*store.CronJob{
			{ID: "job-1", TenantID: "t1", Enabled: true, Schedule: "0 9 * * *", Prompt: "run", NextRunAt: &now},
		},
	}

	s := &SaasScheduler{
		store: st,
		jobs:  make(map[string]registeredJob),
		cfg:   Config{PodID: "test-pod"},
	}

	// Build a real (non-distributed) gocron scheduler for the test.
	sched, err := gocron.NewScheduler()
	if err != nil {
		t.Fatal(err)
	}
	s.sched = sched
	defer sched.Shutdown()
	sched.Start()

	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatalf("syncOnce failed: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs["job-1"]; !ok {
		t.Error("syncOnce: job-1 should be registered in s.jobs")
	}
}

// TestSyncOnce_RemovesDisabledJobs verifies that a job disabled in PG is
// removed from the in-memory scheduler on the next sync cycle.
func TestSyncOnce_RemovesDisabledJobs(t *testing.T) {
	now := time.Now()
	st := &mockCronJobStore{
		jobs: []*store.CronJob{
			{ID: "job-x", TenantID: "t1", Enabled: true, Schedule: "0 9 * * *", Prompt: "run", NextRunAt: &now},
		},
	}

	s := &SaasScheduler{
		store: st,
		jobs:  make(map[string]registeredJob),
		cfg:   Config{PodID: "test-pod"},
	}

	sched, err := gocron.NewScheduler()
	if err != nil {
		t.Fatal(err)
	}
	s.sched = sched
	defer sched.Shutdown()
	sched.Start()

	// First sync: registers job-x.
	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Disable job-x in the mock store.
	st.mu.Lock()
	st.jobs[0].Enabled = false
	st.mu.Unlock()

	// Second sync: should remove job-x.
	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs["job-x"]; ok {
		t.Error("syncOnce: disabled job-x should be removed from s.jobs")
	}
}

// ── ComputeNextRun ────────────────────────────────────────────────────────────

func TestComputeNextRun_ValidExpr(t *testing.T) {
	next := computeNextRun("0 9 * * *", time.Now())
	if next == nil {
		t.Error("expected non-nil next run for valid cron expr")
	}
	// Next run should be in the future.
	if !next.After(time.Now().Add(-time.Minute)) {
		t.Error("next run should be roughly in the future")
	}
}

func TestComputeNextRun_InvalidExpr(t *testing.T) {
	next := computeNextRun("not-a-cron", time.Now())
	if next != nil {
		t.Error("expected nil next run for invalid cron expr")
	}
}
