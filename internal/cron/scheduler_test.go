package cron

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchedulerStartStop(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	s := NewScheduler()
	err := s.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop()
}

func TestSchedulerAddRemoveJob(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	s := NewScheduler()
	s.Start()
	defer s.Stop()

	job := &Job{
		Schedule: "*/5 * * * *",
		Prompt:   "Test job",
		Enabled:  true,
	}

	err := s.AddJob(job)
	if err != nil {
		t.Fatalf("AddJob failed: %v", err)
	}

	jobs := s.ListJobs()
	if len(jobs) != 1 {
		t.Errorf("Expected 1 job, got %d", len(jobs))
	}

	err = s.RemoveJob(job.ID)
	if err != nil {
		t.Fatalf("RemoveJob failed: %v", err)
	}

	jobs = s.ListJobs()
	if len(jobs) != 0 {
		t.Errorf("Expected 0 jobs after remove, got %d", len(jobs))
	}
}

func TestBuildJobPromptContextFrom(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	s := NewScheduler()

	// Save output for a "source" job.
	s.store.SaveJobOutput("source1", "previous report content")

	job := &Job{
		ID:          "consumer1",
		Prompt:      "Summarize the previous report",
		ContextFrom: "source1",
	}

	prompt := s.buildJobPrompt(job)

	if !strings.Contains(prompt, "previous report content") {
		t.Error("expected prompt to contain injected context from source job")
	}
	if !strings.Contains(prompt, "Prior Context (from job source1)") {
		t.Error("expected prompt to contain context header")
	}
	if !strings.Contains(prompt, "Summarize the previous report") {
		t.Error("expected prompt to contain original job prompt")
	}
}

func TestBuildJobPromptNoContextFrom(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	s := NewScheduler()

	job := &Job{
		ID:     "solo1",
		Prompt: "Just run standalone",
	}

	prompt := s.buildJobPrompt(job)

	if strings.Contains(prompt, "Prior Context") {
		t.Error("expected no context block for job without ContextFrom")
	}
	if !strings.Contains(prompt, "Just run standalone") {
		t.Error("expected prompt to contain original job prompt")
	}
}

func TestRunJobWorkdirChange(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	origDir, _ := os.Getwd()

	s := NewScheduler()

	// Test with invalid workdir — should fail gracefully.
	job := &Job{
		ID:      "wdtest",
		Name:    "workdir-test",
		Prompt:  "test",
		Workdir: "/nonexistent_path_that_does_not_exist_xyz",
	}

	success, output, _, errMsg := s.runJob(job)
	if success {
		t.Error("expected failure for invalid workdir")
	}
	if errMsg == "" {
		t.Error("expected error message for invalid workdir")
	}
	if !strings.Contains(output, "FAILED") {
		t.Error("expected FAILED in output")
	}

	// Verify we're still in the original directory.
	currentDir, _ := os.Getwd()
	if currentDir != origDir {
		t.Errorf("working directory changed: was %s, now %s", origDir, currentDir)
	}
}

func TestSchedulerPauseResume(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")
	os.MkdirAll(filepath.Join(tmpDir, "cron"), 0755)

	s := NewScheduler()
	s.Start()
	defer s.Stop()

	job := &Job{
		Schedule: "0 9 * * *",
		Prompt:   "Morning check",
		Enabled:  true,
	}
	s.AddJob(job)

	err := s.PauseJob(job.ID)
	if err != nil {
		t.Fatalf("PauseJob failed: %v", err)
	}

	j := s.store.Get(job.ID)
	if j.Enabled {
		t.Error("Expected job to be disabled after pause")
	}

	err = s.ResumeJob(job.ID)
	if err != nil {
		t.Fatalf("ResumeJob failed: %v", err)
	}

	j = s.store.Get(job.ID)
	if !j.Enabled {
		t.Error("Expected job to be enabled after resume")
	}
}
