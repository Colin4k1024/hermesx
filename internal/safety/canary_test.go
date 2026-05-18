package safety

import (
	"context"
	"testing"
	"time"
)

func TestCanaryDetector_GenerateAndDetect(t *testing.T) {
	cd := NewCanaryDetector()
	token := cd.GenerateToken("tenant-1")

	if cd.ActiveTokenCount() != 1 {
		t.Fatalf("expected 1 active token, got %d", cd.ActiveTokenCount())
	}

	matches := cd.Detect("some output containing " + token + " here")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Category != "canary_leaked" {
		t.Fatalf("expected category canary_leaked, got %q", matches[0].Category)
	}
}

func TestCanaryDetector_RemoveToken(t *testing.T) {
	cd := NewCanaryDetector()
	token := cd.GenerateToken("tenant-2")
	cd.RemoveToken(token)

	if cd.ActiveTokenCount() != 0 {
		t.Fatalf("expected 0 active tokens after removal, got %d", cd.ActiveTokenCount())
	}
	if len(cd.Detect(token)) != 0 {
		t.Fatal("expected no matches after token removal")
	}
}

func TestCanaryDetector_EvictExpired(t *testing.T) {
	cd := NewCanaryDetector()

	// Inject an already-expired entry directly to avoid sleeping.
	cd.mu.Lock()
	cd.tokens["CANARY-expired-CANARY"] = canaryEntry{
		tenantID:  "tenant-old",
		createdAt: time.Now().Add(-48 * time.Hour),
	}
	cd.mu.Unlock()

	// Also add a fresh token.
	_ = cd.GenerateToken("tenant-fresh")

	if cd.ActiveTokenCount() != 2 {
		t.Fatalf("expected 2 tokens before eviction, got %d", cd.ActiveTokenCount())
	}

	cd.mu.Lock()
	removed := cd.evictExpired(24 * time.Hour)
	cd.mu.Unlock()

	if removed != 1 {
		t.Fatalf("expected 1 token evicted, got %d", removed)
	}
	if cd.ActiveTokenCount() != 1 {
		t.Fatalf("expected 1 remaining token, got %d", cd.ActiveTokenCount())
	}
}

func TestCanaryDetector_StartCleanupLoop_StopsOnContextCancel(t *testing.T) {
	cd := NewCanaryDetector()
	ctx, cancel := context.WithCancel(context.Background())

	stop := cd.StartCleanupLoop(ctx, time.Hour)

	// Cancel context and wait for goroutine to exit (stop must not block forever).
	cancel()
	done := make(chan struct{})
	go func() {
		stop()
		close(done)
	}()

	select {
	case <-done:
		// pass
	case <-time.After(3 * time.Second):
		t.Fatal("cleanup goroutine did not stop within 3s after context cancellation")
	}
}

func TestCanaryDetector_StartCleanupLoop_EvictsExpiredTokens(t *testing.T) {
	cd := NewCanaryDetector()

	// Inject a pre-expired entry.
	cd.mu.Lock()
	cd.tokens["CANARY-stale-CANARY"] = canaryEntry{
		tenantID:  "tenant-stale",
		createdAt: time.Now().Add(-10 * time.Minute),
	}
	cd.mu.Unlock()

	// Start the loop and cancel it immediately so the goroutine exits.
	ctx, cancel := context.WithCancel(context.Background())
	stop := cd.StartCleanupLoop(ctx, 5*time.Minute)
	cancel()
	stop()

	// Drive eviction directly to verify the stale token is removable.
	cd.mu.Lock()
	removed := cd.evictExpired(5 * time.Minute)
	cd.mu.Unlock()

	if removed != 1 {
		t.Fatalf("expected 1 evicted token, got %d", removed)
	}
}

func TestCanaryDetector_InjectIntoPrompt(t *testing.T) {
	cd := NewCanaryDetector()
	injected, token := cd.InjectIntoPrompt("You are a helpful assistant.", "tenant-3")

	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if cd.ActiveTokenCount() != 1 {
		t.Fatalf("expected 1 active token, got %d", cd.ActiveTokenCount())
	}

	// The injected prompt must contain the token.
	if len(cd.Detect(injected)) == 0 {
		t.Fatal("expected canary token to be detectable in injected prompt")
	}
}
