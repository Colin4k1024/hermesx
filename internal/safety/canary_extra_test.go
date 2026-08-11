package safety

import (
	"testing"
)

func TestCanaryDetector_ListTokens(t *testing.T) {
	cd := NewCanaryDetector()

	// Empty initially.
	if tokens := cd.ListTokens(); len(tokens) != 0 {
		t.Fatalf("expected 0 tokens, got %d", len(tokens))
	}

	// Generate two tokens and verify ListTokens returns them.
	_ = cd.GenerateToken("tenant-a")
	_ = cd.GenerateToken("tenant-b")

	tokens := cd.ListTokens()
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}

	// Verify token info is populated.
	for _, info := range tokens {
		if info.ID == "" {
			t.Error("TokenInfo.ID should not be empty")
		}
		if info.TenantID == "" {
			t.Error("TokenInfo.TenantID should not be empty")
		}
		if info.CreatedAt.IsZero() {
			t.Error("TokenInfo.CreatedAt should not be zero")
		}
	}
}

func TestCanaryDetector_RemoveTokenByID(t *testing.T) {
	cd := NewCanaryDetector()

	_ = cd.GenerateToken("tenant-x")
	tokens := cd.ListTokens()
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token before removal, got %d", len(tokens))
	}

	id := tokens[0].ID
	cd.RemoveTokenByID(id)

	if cd.ActiveTokenCount() != 0 {
		t.Fatalf("expected 0 tokens after RemoveTokenByID, got %d", cd.ActiveTokenCount())
	}

	// Removing a non-existent ID should not panic.
	cd.RemoveTokenByID("non-existent-id")
}

func TestCanaryDetector_EvictOldest_DirectCall(t *testing.T) {
	cd := NewCanaryDetector()

	// Generate tokens to have something to evict.
	_ = cd.GenerateToken("t1")
	_ = cd.GenerateToken("t2")
	_ = cd.GenerateToken("t3")

	if cd.ActiveTokenCount() != 3 {
		t.Fatalf("expected 3 tokens before eviction, got %d", cd.ActiveTokenCount())
	}

	// Directly call evictOldest to remove 2.
	cd.mu.Lock()
	cd.evictOldest(2)
	cd.mu.Unlock()

	if cd.ActiveTokenCount() != 1 {
		t.Fatalf("expected 1 token after evicting 2, got %d", cd.ActiveTokenCount())
	}
}

func TestCanaryDetector_EvictOldest_ZeroAndNegative(t *testing.T) {
	cd := NewCanaryDetector()
	_ = cd.GenerateToken("t1")

	// evictOldest with n<=0 should be no-op.
	cd.mu.Lock()
	cd.evictOldest(0)
	cd.evictOldest(-1)
	cd.mu.Unlock()

	if cd.ActiveTokenCount() != 1 {
		t.Fatalf("expected token count unchanged, got %d", cd.ActiveTokenCount())
	}
}
