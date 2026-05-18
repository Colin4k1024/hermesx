package safety

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// canaryEntry holds a canary token alongside the tenant it belongs to, the
// wall-clock time it was issued, and an opaque handle (first 8 hex bytes of
// SHA-256). The handle is safe to expose via the admin API without leaking the
// raw token value — the token itself stays server-side only.
type canaryEntry struct {
	id        string // opaque handle: hex(sha256(token)[:4])
	tenantID  string
	createdAt time.Time
}

type CanaryDetector struct {
	mu     sync.RWMutex
	tokens map[string]canaryEntry // token -> entry
}

func NewCanaryDetector() *CanaryDetector {
	return &CanaryDetector{
		tokens: make(map[string]canaryEntry),
	}
}

// tokenHandle derives the opaque 8-hex-char handle for a raw token.
func tokenHandle(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:4])
}

func (cd *CanaryDetector) GenerateToken(tenantID string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	token := "CANARY-" + hex.EncodeToString(b) + "-CANARY"

	cd.mu.Lock()
	cd.tokens[token] = canaryEntry{
		id:        tokenHandle(token),
		tenantID:  tenantID,
		createdAt: time.Now(),
	}
	cd.mu.Unlock()

	return token
}

func (cd *CanaryDetector) InjectIntoPrompt(systemPrompt, tenantID string) (string, string) {
	token := cd.GenerateToken(tenantID)
	injected := systemPrompt + "\n\n<!-- " + token + " -->"
	return injected, token
}

func (cd *CanaryDetector) Detect(output string) []PatternMatch {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	var matches []PatternMatch
	for token, entry := range cd.tokens {
		if strings.Contains(output, token) {
			matches = append(matches, PatternMatch{
				Category: "canary_leaked",
				Pattern:  "canary_token_" + entry.tenantID,
				Match:    truncateMatch(token, 60),
				Severity: 10,
			})
		}
	}
	return matches
}

func (cd *CanaryDetector) RemoveToken(token string) {
	cd.mu.Lock()
	delete(cd.tokens, token)
	cd.mu.Unlock()
}

func (cd *CanaryDetector) ActiveTokenCount() int {
	cd.mu.RLock()
	defer cd.mu.RUnlock()
	return len(cd.tokens)
}

// TokenInfo holds a summary of a single active canary token for admin inspection.
// ID is an opaque handle (hex(sha256(token)[:4])); the raw token is never exposed.
type TokenInfo struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ListTokens returns a snapshot of all currently active canary tokens using
// opaque handles — raw token values are never included in the response (B-2).
func (cd *CanaryDetector) ListTokens() []TokenInfo {
	cd.mu.RLock()
	defer cd.mu.RUnlock()
	out := make([]TokenInfo, 0, len(cd.tokens))
	for _, entry := range cd.tokens {
		out = append(out, TokenInfo{
			ID:        entry.id,
			TenantID:  entry.tenantID,
			CreatedAt: entry.createdAt,
		})
	}
	return out
}

// RemoveTokenByID revokes the token whose opaque handle matches id.
func (cd *CanaryDetector) RemoveTokenByID(id string) {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	for token, entry := range cd.tokens {
		if entry.id == id {
			delete(cd.tokens, token)
			return
		}
	}
}

// evictExpired removes all tokens whose age exceeds ttl. Called under write lock.
func (cd *CanaryDetector) evictExpired(ttl time.Duration) int {
	deadline := time.Now().Add(-ttl)
	removed := 0
	for token, entry := range cd.tokens {
		if entry.createdAt.Before(deadline) {
			delete(cd.tokens, token)
			removed++
		}
	}
	return removed
}

// StartCleanupLoop launches a background goroutine that evicts tokens older
// than ttl. The sweep interval is ttl/2, floored at 1 minute.
//
// The returned stop function blocks until the goroutine exits; callers may
// also cancel the supplied context to achieve the same effect.
//
// Typical usage:
//
//	stop := detector.StartCleanupLoop(ctx, 24*time.Hour)
//	defer stop()
func (cd *CanaryDetector) StartCleanupLoop(ctx context.Context, ttl time.Duration) func() {
	interval := max(ttl/2, time.Minute)

	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cd.mu.Lock()
				cd.evictExpired(ttl)
				cd.mu.Unlock()
			}
		}
	}()

	return func() { <-done }
}
