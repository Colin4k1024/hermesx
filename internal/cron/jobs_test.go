package cron

import (
	"os"
	"path/filepath"
	"testing"
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
