package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hermes-agent/hermes-agent-go/internal/llm"
	"github.com/hermes-agent/hermes-agent-go/internal/store"
)

// CuratorConfig controls autonomous memory curation behavior.
type CuratorConfig struct {
	// Enabled activates the curator.
	Enabled bool

	// MaxMemories is the ceiling. When exceeded, lowest-scored entries are pruned.
	MaxMemories int

	// StaleAfter marks memories older than this as candidates for pruning.
	StaleAfter time.Duration

	// DedupeThreshold is the minimum similarity ratio (0.0-1.0) to consider
	// two memory entries as duplicates. Uses normalized key+content comparison.
	DedupeThreshold float64
}

// DefaultCuratorConfig returns sensible defaults.
func DefaultCuratorConfig() CuratorConfig {
	return CuratorConfig{
		Enabled:         true,
		MaxMemories:     100,
		StaleAfter:      30 * 24 * time.Hour, // 30 days
		DedupeThreshold: 0.85,
	}
}

// CuratorResult holds the outcome of a curation run.
type CuratorResult struct {
	Scanned    int
	Duplicates int
	Pruned     int
	Kept       int
}

// MemoryCurator autonomously reviews and maintains memory quality.
type MemoryCurator struct {
	store     store.MemoryStore
	completer chatCompleter
	config    CuratorConfig
}

// NewMemoryCurator creates a curator with the given store and optional LLM
// completer for semantic deduplication. If completer is nil, only heuristic
// deduplication is performed.
func NewMemoryCurator(ms store.MemoryStore, completer chatCompleter, cfg CuratorConfig) *MemoryCurator {
	return &MemoryCurator{
		store:     ms,
		completer: completer,
		config:    cfg,
	}
}

// Curate runs a full curation pass for the given tenant and user.
func (c *MemoryCurator) Curate(ctx context.Context, tenantID, userID string) (*CuratorResult, error) {
	if !c.config.Enabled {
		return &CuratorResult{}, nil
	}

	entries, err := c.store.List(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("curator list memories: %w", err)
	}

	result := &CuratorResult{Scanned: len(entries)}

	// Phase 1: Detect and remove duplicates.
	deduped, dupeCount := c.deduplicateEntries(entries)
	result.Duplicates = dupeCount

	// Delete duplicates from store.
	for _, key := range c.findDuplicateKeys(entries, deduped) {
		if err := c.store.Delete(ctx, tenantID, userID, key); err != nil {
			slog.Warn("Curator failed to delete duplicate", "key", key, "error", err)
		} else {
			result.Pruned++
		}
	}

	// Phase 2: Prune stale entries if over limit.
	if len(deduped) > c.config.MaxMemories {
		stale := c.findStaleEntries(deduped)
		pruneCount := len(deduped) - c.config.MaxMemories
		if pruneCount > len(stale) {
			pruneCount = len(stale)
		}
		for i := 0; i < pruneCount; i++ {
			if err := c.store.Delete(ctx, tenantID, userID, stale[i].Key); err != nil {
				slog.Warn("Curator failed to prune stale entry", "key", stale[i].Key, "error", err)
			} else {
				result.Pruned++
			}
		}
	}

	result.Kept = result.Scanned - result.Pruned
	slog.Info("Memory curation complete",
		"tenant", tenantID,
		"user", userID,
		"scanned", result.Scanned,
		"duplicates", result.Duplicates,
		"pruned", result.Pruned,
		"kept", result.Kept,
	)
	return result, nil
}

// CurateWithLLM performs an LLM-assisted curation that can merge semantically
// similar memories and generate improved consolidated entries.
func (c *MemoryCurator) CurateWithLLM(ctx context.Context, tenantID, userID string) (*CuratorResult, error) {
	if c.completer == nil {
		return c.Curate(ctx, tenantID, userID)
	}

	entries, err := c.store.List(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("curator list memories: %w", err)
	}

	result := &CuratorResult{Scanned: len(entries)}

	if len(entries) < 2 {
		result.Kept = len(entries)
		return result, nil
	}

	// Ask LLM to identify consolidation opportunities.
	mergeGroups, err := c.identifyMergeGroups(ctx, entries)
	if err != nil {
		slog.Warn("LLM-assisted curation failed, falling back to heuristic", "error", err)
		return c.Curate(ctx, tenantID, userID)
	}

	// Process merge groups: consolidate into single entries.
	for _, group := range mergeGroups {
		if len(group) < 2 {
			continue
		}
		// Keep first entry, merge content, delete the rest.
		merged := c.mergeContents(group)
		if err := c.store.Upsert(ctx, tenantID, userID, group[0].Key, merged); err != nil {
			slog.Warn("Curator merge failed", "key", group[0].Key, "error", err)
			continue
		}
		for _, entry := range group[1:] {
			if err := c.store.Delete(ctx, tenantID, userID, entry.Key); err != nil {
				slog.Warn("Curator delete merged entry failed", "key", entry.Key, "error", err)
			} else {
				result.Pruned++
				result.Duplicates++
			}
		}
	}

	result.Kept = result.Scanned - result.Pruned
	return result, nil
}

// deduplicateEntries returns the de-duplicated set and count of duplicates found.
func (c *MemoryCurator) deduplicateEntries(entries []store.MemoryEntry) ([]store.MemoryEntry, int) {
	if len(entries) < 2 {
		return entries, 0
	}

	seen := make(map[string]bool)
	var unique []store.MemoryEntry
	dupes := 0

	for i := range entries {
		isDupe := false
		for j := range unique {
			if c.isSimilar(entries[i], unique[j]) {
				isDupe = true
				dupes++
				break
			}
		}
		if !isDupe {
			key := entries[i].Key
			if !seen[key] {
				seen[key] = true
				unique = append(unique, entries[i])
			} else {
				dupes++
			}
		}
	}
	return unique, dupes
}

// findDuplicateKeys returns the keys that are in entries but not in deduped.
func (c *MemoryCurator) findDuplicateKeys(entries, deduped []store.MemoryEntry) []string {
	dedupedKeys := make(map[string]bool, len(deduped))
	for _, e := range deduped {
		dedupedKeys[e.Key] = true
	}
	var dupeKeys []string
	seen := make(map[string]bool)
	for _, e := range entries {
		if !dedupedKeys[e.Key] && !seen[e.Key] {
			dupeKeys = append(dupeKeys, e.Key)
			seen[e.Key] = true
		}
	}
	return dupeKeys
}

// findStaleEntries returns entries older than StaleAfter, sorted oldest first.
func (c *MemoryCurator) findStaleEntries(entries []store.MemoryEntry) []store.MemoryEntry {
	cutoff := time.Now().Add(-c.config.StaleAfter)
	var stale []store.MemoryEntry
	for _, e := range entries {
		if e.UpdatedAt.Before(cutoff) {
			stale = append(stale, e)
		}
	}
	return stale
}

// isSimilar returns true if two entries are sufficiently similar to be
// considered duplicates.
func (c *MemoryCurator) isSimilar(a, b store.MemoryEntry) bool {
	// Exact key match is always a duplicate.
	if strings.EqualFold(a.Key, b.Key) {
		return true
	}
	// Content similarity using normalized comparison.
	ratio := normalizedSimilarity(a.Content, b.Content)
	return ratio >= c.config.DedupeThreshold
}

// normalizedSimilarity computes a simple word-overlap similarity ratio.
func normalizedSimilarity(a, b string) float64 {
	wordsA := strings.Fields(strings.ToLower(a))
	wordsB := strings.Fields(strings.ToLower(b))

	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0
	}

	setB := make(map[string]bool, len(wordsB))
	for _, w := range wordsB {
		setB[w] = true
	}

	overlap := 0
	for _, w := range wordsA {
		if setB[w] {
			overlap++
		}
	}

	maxLen := len(wordsA)
	if len(wordsB) > maxLen {
		maxLen = len(wordsB)
	}
	return float64(overlap) / float64(maxLen)
}

// identifyMergeGroups uses LLM to find semantically similar memory groups.
func (c *MemoryCurator) identifyMergeGroups(ctx context.Context, entries []store.MemoryEntry) ([][]store.MemoryEntry, error) {
	var sb strings.Builder
	sb.WriteString("Below is a list of memory entries. Identify groups of entries that are semantically duplicates or should be merged. ")
	sb.WriteString("Return ONLY the group numbers (1-indexed) separated by semicolons, with entries in each group separated by commas. ")
	sb.WriteString("Example: '1,3;2,5' means entries 1&3 form one group and 2&5 form another. ")
	sb.WriteString("If no merges needed, return 'NONE'.\n\n")

	for i, e := range entries {
		content := e.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		fmt.Fprintf(&sb, "%d. [%s] %s\n", i+1, e.Key, content)
	}

	resp, err := c.completer.CreateChatCompletion(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: sb.String()},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("identify merge groups: %w", err)
	}

	return parseMergeGroups(resp.Content, entries), nil
}

// parseMergeGroups parses the LLM response into entry groups.
func parseMergeGroups(response string, entries []store.MemoryEntry) [][]store.MemoryEntry {
	response = strings.TrimSpace(response)
	if strings.EqualFold(response, "NONE") || response == "" {
		return nil
	}

	var groups [][]store.MemoryEntry
	parts := strings.Split(response, ";")
	for _, part := range parts {
		indices := strings.Split(strings.TrimSpace(part), ",")
		var group []store.MemoryEntry
		for _, idx := range indices {
			idx = strings.TrimSpace(idx)
			var n int
			if _, err := fmt.Sscanf(idx, "%d", &n); err == nil && n >= 1 && n <= len(entries) {
				group = append(group, entries[n-1])
			}
		}
		if len(group) >= 2 {
			groups = append(groups, group)
		}
	}
	return groups
}

// mergeContents combines the content of multiple entries.
func (c *MemoryCurator) mergeContents(entries []store.MemoryEntry) string {
	var parts []string
	for _, e := range entries {
		parts = append(parts, e.Content)
	}
	return strings.Join(parts, "\n\n---\n\n")
}
