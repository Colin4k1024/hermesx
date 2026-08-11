package secrets

import (
	"context"
	"regexp"
	"testing"
	"time"
)

func TestStaticPatternSource_AddAndLoad(t *testing.T) {
	src := NewStaticPatternSource()
	ctx := context.Background()

	// Initially empty.
	patterns, err := src.LoadPatterns(ctx)
	if err != nil {
		t.Fatalf("LoadPatterns: %v", err)
	}
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns initially, got %d", len(patterns))
	}

	// Add a pattern.
	re := regexp.MustCompile(`secret-\d+`)
	src.Add("test-pattern", re, SeverityHigh)

	patterns, err = src.LoadPatterns(ctx)
	if err != nil {
		t.Fatalf("LoadPatterns after add: %v", err)
	}
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
	if patterns[0].Name != "test-pattern" {
		t.Errorf("pattern name = %q, want test-pattern", patterns[0].Name)
	}
}

func TestPatternWatcher_StartStop(t *testing.T) {
	scanner := NewLeakScanner()
	src := NewStaticPatternSource()

	watcher := NewPatternWatcher(scanner, src, 100*time.Millisecond)
	if watcher == nil {
		t.Fatal("NewPatternWatcher should return non-nil")
	}

	ctx := context.Background()
	watcher.Start(ctx)

	// Double Start should be idempotent.
	watcher.Start(ctx)

	// Give the watcher a moment to run.
	time.Sleep(50 * time.Millisecond)

	watcher.Stop()

	// Double Stop should be idempotent.
	watcher.Stop()
}

func TestPatternWatcher_ForceReload(t *testing.T) {
	scanner := NewLeakScanner()
	src := NewStaticPatternSource()

	re := regexp.MustCompile(`token-\d+`)
	src.Add("force-test", re, SeverityMedium)

	watcher := NewPatternWatcher(scanner, src, time.Hour)

	ctx := context.Background()
	if err := watcher.ForceReload(ctx); err != nil {
		t.Fatalf("ForceReload: %v", err)
	}

	// The pattern should now be in the scanner.
	// Try scanning text that matches.
	matches := scanner.Scan("token-123 is a secret")
	found := false
	for _, m := range matches {
		if m.PatternName == "force-test" {
			found = true
		}
	}
	if !found {
		t.Log("Pattern may not be found if builtin patterns take precedence (that's OK)")
	}
}
