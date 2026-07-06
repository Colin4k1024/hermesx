package rediscache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return &Client{rdb: rdb}, mr
}

func TestSessionLockKey_Format(t *testing.T) {
	key := sessionLockKey("tenant-abc", "session-123")
	expected := "hermes:lock:tenant-abc:session-123"
	if key != expected {
		t.Errorf("sessionLockKey = %q, want %q", key, expected)
	}
}

func TestTaskLockKey_Format(t *testing.T) {
	key := taskLockKey("tenant-abc", "session-123", "execute_code")
	expected := "hermes:lock:tenant-abc:session-123:execute_code"
	if key != expected {
		t.Errorf("taskLockKey = %q, want %q", key, expected)
	}
}

func TestAcquireSessionLock_Basic(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	token, acquired, err := client.AcquireSessionLock(ctx, "t1", "s1", 5*time.Second)
	if err != nil {
		t.Fatalf("AcquireSessionLock error: %v", err)
	}
	if !acquired {
		t.Fatal("expected lock to be acquired")
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Second acquire should fail (same session)
	_, acquired2, err := client.AcquireSessionLock(ctx, "t1", "s1", 5*time.Second)
	if err != nil {
		t.Fatalf("second AcquireSessionLock error: %v", err)
	}
	if acquired2 {
		t.Fatal("expected second acquire to fail for same session")
	}
}

func TestAcquireSessionLock_DifferentSessionsSucceed(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	// Two different sessions for the same tenant should both acquire locks
	_, acquired1, err := client.AcquireSessionLock(ctx, "t1", "s1", 5*time.Second)
	if err != nil {
		t.Fatalf("first lock error: %v", err)
	}
	if !acquired1 {
		t.Fatal("expected first lock to be acquired")
	}

	_, acquired2, err := client.AcquireSessionLock(ctx, "t1", "s2", 5*time.Second)
	if err != nil {
		t.Fatalf("second lock error: %v", err)
	}
	if !acquired2 {
		t.Fatal("expected second lock (different session) to be acquired")
	}
}

func TestReleaseSessionLock_OwnerVerified(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	token, _, _ := client.AcquireSessionLock(ctx, "t1", "s1", 5*time.Second)

	// Release with correct token
	err := client.ReleaseSessionLock(ctx, "t1", "s1", token)
	if err != nil {
		t.Fatalf("ReleaseSessionLock error: %v", err)
	}

	// Should be able to re-acquire after release
	_, acquired, _ := client.AcquireSessionLock(ctx, "t1", "s1", 5*time.Second)
	if !acquired {
		t.Fatal("expected lock to be re-acquirable after release")
	}
}

func TestAcquireTaskLock_Basic(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	token, acquired, err := client.AcquireTaskLock(ctx, "t1", "s1", "execute_code", 5*time.Second)
	if err != nil {
		t.Fatalf("AcquireTaskLock error: %v", err)
	}
	if !acquired {
		t.Fatal("expected task lock to be acquired")
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Same tool in same session should fail
	_, acquired2, _ := client.AcquireTaskLock(ctx, "t1", "s1", "execute_code", 5*time.Second)
	if acquired2 {
		t.Fatal("expected duplicate task lock to fail")
	}
}

func TestAcquireTaskLock_DifferentToolsSucceed(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	// Different tools within the same session should both succeed
	_, acquired1, _ := client.AcquireTaskLock(ctx, "t1", "s1", "execute_code", 5*time.Second)
	if !acquired1 {
		t.Fatal("expected execute_code lock to succeed")
	}

	_, acquired2, _ := client.AcquireTaskLock(ctx, "t1", "s1", "read_file", 5*time.Second)
	if !acquired2 {
		t.Fatal("expected read_file lock to succeed (different tool)")
	}
}

// TestConcurrentSessionsAcquireLocks demonstrates that two sessions for the
// same tenant can acquire locks simultaneously without blocking each other.
// This is the key property of the redesigned lock key structure.
func TestConcurrentSessionsAcquireLocks(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	const numSessions = 10
	var wg sync.WaitGroup
	results := make([]bool, numSessions)

	for i := range numSessions {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sessionID := "session-" + time.Now().Format("150405") + "-" + string(rune('A'+idx))
			_, acquired, err := client.AcquireTaskLock(ctx, "tenant-1", sessionID, "execute_code", 5*time.Second)
			if err != nil {
				t.Errorf("session %d lock error: %v", idx, err)
				return
			}
			results[idx] = acquired
		}(i)
	}
	wg.Wait()

	// All sessions should have acquired their locks (different session IDs)
	for i, acquired := range results {
		if !acquired {
			t.Errorf("session %d failed to acquire lock (should not block)", i)
		}
	}
}

func TestReleaseTaskLock_OwnerVerified(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	token, _, _ := client.AcquireTaskLock(ctx, "t1", "s1", "execute_code", 5*time.Second)

	// Release with wrong token should not release
	_ = client.ReleaseTaskLock(ctx, "t1", "s1", "execute_code", "wrong-token")

	// Lock should still be held
	_, acquired, _ := client.AcquireTaskLock(ctx, "t1", "s1", "execute_code", 5*time.Second)
	if acquired {
		t.Fatal("expected lock to still be held after wrong-token release")
	}

	// Release with correct token
	err := client.ReleaseTaskLock(ctx, "t1", "s1", "execute_code", token)
	if err != nil {
		t.Fatalf("ReleaseTaskLock error: %v", err)
	}

	// Should be re-acquirable
	_, reacquired, _ := client.AcquireTaskLock(ctx, "t1", "s1", "execute_code", 5*time.Second)
	if !reacquired {
		t.Fatal("expected lock to be re-acquirable after correct release")
	}
}
