package cron

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJobStore(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	store := NewJobStore()

	// Add job
	job := &Job{
		Schedule: "*/5 * * * *",
		Prompt:   "Check server status",
		Enabled:  true,
	}
	err := store.Add(job)
	if err != nil {
		t.Fatalf("Add job failed: %v", err)
	}
	if job.ID == "" {
		t.Error("Expected non-empty job ID")
	}

	// List jobs
	jobs := store.List()
	if len(jobs) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs))
	}

	// Get job
	got := store.Get(job.ID)
	if got == nil {
		t.Fatal("Expected to find job by ID")
	}
	if got.Prompt != "Check server status" {
		t.Errorf("Expected prompt match, got '%s'", got.Prompt)
	}

	// Pause
	err = store.Pause(job.ID)
	if err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	got = store.Get(job.ID)
	if got.Enabled {
		t.Error("Expected job to be disabled after pause")
	}

	// Resume
	err = store.Resume(job.ID)
	if err != nil {
		t.Fatalf("Resume failed: %v", err)
	}
	got = store.Get(job.ID)
	if !got.Enabled {
		t.Error("Expected job to be enabled after resume")
	}

	// Remove
	err = store.Remove(job.ID)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	jobs = store.List()
	if len(jobs) != 0 {
		t.Errorf("Expected 0 jobs after remove, got %d", len(jobs))
	}
}

func TestJobStoreSaveAndGetLatestOutput(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	store := NewJobStore()

	// Save two outputs for same job.
	_, err := store.SaveJobOutput("job1", "first output")
	if err != nil {
		t.Fatalf("SaveJobOutput first: %v", err)
	}
	_, err = store.SaveJobOutput("job1", "second output")
	if err != nil {
		t.Fatalf("SaveJobOutput second: %v", err)
	}

	// GetLatestOutput should return the most recent (lexicographically last).
	content, err := store.GetLatestOutput("job1")
	if err != nil {
		t.Fatalf("GetLatestOutput: %v", err)
	}
	if content != "second output" {
		t.Errorf("expected 'second output', got %q", content)
	}
}

func TestJobStoreGetLatestOutputNoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	store := NewJobStore()

	_, err := store.GetLatestOutput("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent job output")
	}
}

func TestJobWorkdirAndContextFromFields(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	store := NewJobStore()

	job := &Job{
		Name:        "test-chain",
		Schedule:    "*/5 * * * *",
		Prompt:      "check status",
		Workdir:     "/tmp/myproject",
		ContextFrom: "abc123",
	}
	if err := store.Add(job); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Reload from disk.
	store2 := &JobStore{
		jobs:    make(map[string]*Job),
		jobsDir: filepath.Join(tmpDir, "cron"),
	}
	if err := store2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := store2.Get(job.ID)
	if got == nil {
		t.Fatal("expected job after reload")
	}
	if got.Workdir != "/tmp/myproject" {
		t.Errorf("Workdir=%q, want /tmp/myproject", got.Workdir)
	}
	if got.ContextFrom != "abc123" {
		t.Errorf("ContextFrom=%q, want abc123", got.ContextFrom)
	}
}

func TestJobStoreNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	store := NewJobStore()

	got := store.Get("nonexistent-id")
	if got != nil {
		t.Error("Expected nil for nonexistent job")
	}

	err := store.Pause("nonexistent-id")
	if err == nil {
		t.Error("Expected error for pausing nonexistent job")
	}

	err = store.Remove("nonexistent-id")
	if err == nil {
		t.Error("Expected error for removing nonexistent job")
	}
}

func TestJobStore_Save(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	store := NewJobStore()
	if err := store.Add(&Job{Schedule: "0 9 * * *", Prompt: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func TestJobStore_Update(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	store := NewJobStore()
	job := &Job{ID: "upd-1", Schedule: "0 9 * * *", Prompt: "before"}
	store.jobs["upd-1"] = job

	job.Prompt = "after"
	if err := store.Update(job); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := store.Get("upd-1")
	if got == nil || got.Prompt != "after" {
		t.Errorf("Update: prompt not persisted, got %v", got)
	}

	// Update non-existent job should error.
	if err := store.Update(&Job{ID: "nonexistent"}); err == nil {
		t.Error("Update nonexistent should return error")
	}
}

func TestJobStore_GetDueJobs(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	store := NewJobStore()
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	store.jobs["due"] = &Job{ID: "due", Enabled: true, NextRunAt: &past}
	store.jobs["not-due"] = &Job{ID: "not-due", Enabled: true, NextRunAt: &future}
	store.jobs["disabled"] = &Job{ID: "disabled", Enabled: false, NextRunAt: &past}

	due := store.GetDueJobs()
	if len(due) != 1 || due[0].ID != "due" {
		t.Errorf("GetDueJobs: got %d jobs, want 1 with ID 'due'", len(due))
	}
}

func TestJobStore_MarkRun(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	store := NewJobStore()
	store.jobs["run-1"] = &Job{ID: "run-1", Enabled: true}

	store.MarkRun("run-1", true, "")
	job := store.Get("run-1")
	if job.RunCount != 1 {
		t.Errorf("RunCount = %d, want 1", job.RunCount)
	}
	if job.LastRunSuccess == nil || !*job.LastRunSuccess {
		t.Error("LastRunSuccess should be true")
	}

	// MarkRun on non-existent job should be a no-op.
	store.MarkRun("nonexistent", false, "error")
}
