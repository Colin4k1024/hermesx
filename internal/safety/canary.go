package safety

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
)

type CanaryDetector struct {
	mu     sync.RWMutex
	tokens map[string]string // token -> tenant_id
}

func NewCanaryDetector() *CanaryDetector {
	return &CanaryDetector{
		tokens: make(map[string]string),
	}
}

func (cd *CanaryDetector) GenerateToken(tenantID string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	token := "CANARY-" + hex.EncodeToString(b) + "-CANARY"

	cd.mu.Lock()
	cd.tokens[token] = tenantID
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
	for token, tenantID := range cd.tokens {
		if strings.Contains(output, token) {
			matches = append(matches, PatternMatch{
				Category: "canary_leaked",
				Pattern:  "canary_token_" + tenantID,
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
