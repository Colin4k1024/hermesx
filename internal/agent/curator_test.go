package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hermes-agent/hermes-agent-go/internal/llm"
	"github.com/hermes-agent/hermes-agent-go/internal/store"
)

type mockMemoryStore struct {
	entries map[string][]store.MemoryEntry
	deleted []string
}

func newMockMemoryStore() *mockMemoryStore {
	return &mockMemoryStore{
		entries: make(map[string][]store.MemoryEntry),
	}
}

func (m *mockMemoryStore) key(tenantID, userID string) string {
	return tenantID + ":" + userID
}

func (m *mockMemoryStore) Get(ctx context.Context, tenantID, userID, key string) (string, error) {
	for _, e := range m.entries[m.key(tenantID, userID)] {
		if e.Key == key {
			return e.Content, nil
		}
	}
	return "", fmt.Errorf("not found")
}

func (m *mockMemoryStore) List(ctx context.Context, tenantID, userID string) ([]store.MemoryEntry, error) {
	return m.entries[m.key(tenantID, userID)], nil
}

func (m *mockMemoryStore) Upsert(ctx context.Context, tenantID, userID, key, content string) error {
	k := m.key(tenantID, userID)
	for i, e := range m.entries[k] {
		if e.Key == key {
			m.entries[k][i].Content = content
			m.entries[k][i].UpdatedAt = time.Now()
			return nil
		}
	}
	m.entries[k] = append(m.entries[k], store.MemoryEntry{
		TenantID:  tenantID,
		UserID:    userID,
		Key:       key,
		Content:   content,
		UpdatedAt: time.Now(),
	})
	return nil
}

func (m *mockMemoryStore) Delete(ctx context.Context, tenantID, userID, key string) error {
	k := m.key(tenantID, userID)
	m.deleted = append(m.deleted, key)
	var remaining []store.MemoryEntry
	for _, e := range m.entries[k] {
		if e.Key != key {
			remaining = append(remaining, e)
		}
	}
	m.entries[k] = remaining
	return nil
}

func (m *mockMemoryStore) DeleteAllByUser(ctx context.Context, tenantID, userID string) (int64, error) {
	k := m.key(tenantID, userID)
	n := int64(len(m.entries[k]))
	delete(m.entries, k)
	return n, nil
}

func (m *mockMemoryStore) DeleteAllByTenant(ctx context.Context, tenantID string) (int64, error) {
	var n int64
	for k := range m.entries {
		if strings.HasPrefix(k, tenantID+":") {
			n += int64(len(m.entries[k]))
			delete(m.entries, k)
		}
	}
	return n, nil
}

func (m *mockMemoryStore) addEntry(tenantID, userID, key, content string, updatedAt time.Time) {
	k := m.key(tenantID, userID)
	m.entries[k] = append(m.entries[k], store.MemoryEntry{
		TenantID:  tenantID,
		UserID:    userID,
		Key:       key,
		Content:   content,
		UpdatedAt: updatedAt,
	})
}

func TestCurate_Disabled(t *testing.T) {
	ms := newMockMemoryStore()
	cfg := DefaultCuratorConfig()
	cfg.Enabled = false
	c := NewMemoryCurator(ms, nil, cfg)

	result, err := c.Curate(context.Background(), "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 0 {
		t.Errorf("expected 0 scanned when disabled, got %d", result.Scanned)
	}
}

func TestCurate_EmptyStore(t *testing.T) {
	ms := newMockMemoryStore()
	c := NewMemoryCurator(ms, nil, DefaultCuratorConfig())

	result, err := c.Curate(context.Background(), "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 0 || result.Pruned != 0 {
		t.Errorf("expected 0 scanned and pruned for empty store, got scanned=%d pruned=%d", result.Scanned, result.Pruned)
	}
}

func TestCurate_NoDuplicates(t *testing.T) {
	ms := newMockMemoryStore()
	now := time.Now()
	ms.addEntry("t1", "u1", "key1", "content one", now)
	ms.addEntry("t1", "u1", "key2", "completely different content", now)

	c := NewMemoryCurator(ms, nil, DefaultCuratorConfig())
	result, err := c.Curate(context.Background(), "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Duplicates != 0 {
		t.Errorf("expected 0 duplicates, got %d", result.Duplicates)
	}
	if result.Kept != 2 {
		t.Errorf("expected 2 kept, got %d", result.Kept)
	}
}

func TestCurate_ExactKeyDuplicates(t *testing.T) {
	ms := newMockMemoryStore()
	now := time.Now()
	k := ms.key("t1", "u1")
	ms.entries[k] = []store.MemoryEntry{
		{TenantID: "t1", UserID: "u1", Key: "key1", Content: "first", UpdatedAt: now},
		{TenantID: "t1", UserID: "u1", Key: "key1", Content: "second", UpdatedAt: now},
	}

	c := NewMemoryCurator(ms, nil, DefaultCuratorConfig())
	result, err := c.Curate(context.Background(), "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Duplicates < 1 {
		t.Errorf("expected at least 1 duplicate for same key, got %d", result.Duplicates)
	}
}

func TestCurate_ContentSimilarityDuplicates(t *testing.T) {
	ms := newMockMemoryStore()
	now := time.Now()
	ms.addEntry("t1", "u1", "pref-a", "the user prefers dark mode for all interfaces", now)
	ms.addEntry("t1", "u1", "pref-b", "the user prefers dark mode for all interfaces and apps", now)

	cfg := DefaultCuratorConfig()
	cfg.DedupeThreshold = 0.80
	c := NewMemoryCurator(ms, nil, cfg)

	result, err := c.Curate(context.Background(), "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Duplicates < 1 {
		t.Errorf("expected at least 1 duplicate for similar content, got %d", result.Duplicates)
	}
}

func TestCurate_StaleEntryPruning(t *testing.T) {
	ms := newMockMemoryStore()
	now := time.Now()
	staleTime := now.Add(-60 * 24 * time.Hour) // 60 days ago

	cfg := DefaultCuratorConfig()
	cfg.MaxMemories = 2
	cfg.StaleAfter = 30 * 24 * time.Hour

	ms.addEntry("t1", "u1", "fresh1", "recent content", now)
	ms.addEntry("t1", "u1", "fresh2", "also recent", now)
	ms.addEntry("t1", "u1", "stale1", "old content one", staleTime)
	ms.addEntry("t1", "u1", "stale2", "old content two", staleTime)

	c := NewMemoryCurator(ms, nil, cfg)
	result, err := c.Curate(context.Background(), "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Pruned < 1 {
		t.Errorf("expected at least 1 pruned stale entry, got %d", result.Pruned)
	}
}

func TestCurate_DoesNotPruneWhenUnderLimit(t *testing.T) {
	ms := newMockMemoryStore()
	staleTime := time.Now().Add(-60 * 24 * time.Hour)

	cfg := DefaultCuratorConfig()
	cfg.MaxMemories = 100

	ms.addEntry("t1", "u1", "stale1", "old but under limit", staleTime)
	ms.addEntry("t1", "u1", "stale2", "also old but under limit", staleTime)

	c := NewMemoryCurator(ms, nil, cfg)
	result, err := c.Curate(context.Background(), "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Pruned != 0 {
		t.Errorf("should not prune when under MaxMemories limit, got pruned=%d", result.Pruned)
	}
}

func TestNormalizedSimilarity_Identical(t *testing.T) {
	ratio := normalizedSimilarity("hello world foo bar", "hello world foo bar")
	if ratio != 1.0 {
		t.Errorf("expected 1.0 for identical strings, got %f", ratio)
	}
}

func TestNormalizedSimilarity_NoOverlap(t *testing.T) {
	ratio := normalizedSimilarity("alpha beta gamma", "delta epsilon zeta")
	if ratio != 0.0 {
		t.Errorf("expected 0.0 for no overlap, got %f", ratio)
	}
}

func TestNormalizedSimilarity_PartialOverlap(t *testing.T) {
	ratio := normalizedSimilarity("the cat sat on the mat", "the dog sat on the rug")
	if ratio < 0.4 || ratio > 0.8 {
		t.Errorf("expected partial overlap ratio in 0.4-0.8, got %f", ratio)
	}
}

func TestNormalizedSimilarity_EmptyStrings(t *testing.T) {
	if normalizedSimilarity("", "hello") != 0 {
		t.Error("expected 0 when first is empty")
	}
	if normalizedSimilarity("hello", "") != 0 {
		t.Error("expected 0 when second is empty")
	}
}

func TestParseMergeGroups_None(t *testing.T) {
	entries := []store.MemoryEntry{{Key: "a"}, {Key: "b"}}
	groups := parseMergeGroups("NONE", entries)
	if groups != nil {
		t.Error("expected nil for NONE response")
	}
}

func TestParseMergeGroups_Empty(t *testing.T) {
	entries := []store.MemoryEntry{{Key: "a"}, {Key: "b"}}
	groups := parseMergeGroups("", entries)
	if groups != nil {
		t.Error("expected nil for empty response")
	}
}

func TestParseMergeGroups_SingleGroup(t *testing.T) {
	entries := []store.MemoryEntry{{Key: "a"}, {Key: "b"}, {Key: "c"}}
	groups := parseMergeGroups("1,3", entries)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0]) != 2 {
		t.Fatalf("expected 2 entries in group, got %d", len(groups[0]))
	}
	if groups[0][0].Key != "a" || groups[0][1].Key != "c" {
		t.Error("wrong entries in group")
	}
}

func TestParseMergeGroups_MultipleGroups(t *testing.T) {
	entries := []store.MemoryEntry{{Key: "a"}, {Key: "b"}, {Key: "c"}, {Key: "d"}, {Key: "e"}}
	groups := parseMergeGroups("1,3;2,5", entries)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
}

func TestParseMergeGroups_InvalidIndices(t *testing.T) {
	entries := []store.MemoryEntry{{Key: "a"}, {Key: "b"}}
	groups := parseMergeGroups("1,99", entries) // 99 is out of bounds
	if len(groups) != 0 {
		t.Errorf("expected no valid groups for out-of-range index, got %d", len(groups))
	}
}

func TestParseMergeGroups_SingleEntryGroup(t *testing.T) {
	entries := []store.MemoryEntry{{Key: "a"}, {Key: "b"}, {Key: "c"}}
	groups := parseMergeGroups("1", entries) // single entry, not a valid group
	if len(groups) != 0 {
		t.Errorf("expected no groups for single-entry group, got %d", len(groups))
	}
}

func TestCurateWithLLM_NilCompleter(t *testing.T) {
	ms := newMockMemoryStore()
	now := time.Now()
	ms.addEntry("t1", "u1", "k1", "content one", now)
	ms.addEntry("t1", "u1", "k2", "content two", now)

	c := NewMemoryCurator(ms, nil, DefaultCuratorConfig())
	result, err := c.CurateWithLLM(context.Background(), "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 2 {
		t.Errorf("expected 2 scanned, got %d", result.Scanned)
	}
}

func TestCurateWithLLM_SingleEntry(t *testing.T) {
	ms := newMockMemoryStore()
	ms.addEntry("t1", "u1", "k1", "content one", time.Now())

	stub := &stubChatClient{response: "NONE"}
	c := NewMemoryCurator(ms, stub, DefaultCuratorConfig())
	result, err := c.CurateWithLLM(context.Background(), "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Kept != 1 {
		t.Errorf("expected 1 kept for single entry, got %d", result.Kept)
	}
}

func TestCurateWithLLM_MergesEntries(t *testing.T) {
	ms := newMockMemoryStore()
	now := time.Now()
	ms.addEntry("t1", "u1", "k1", "user likes dark mode", now)
	ms.addEntry("t1", "u1", "k2", "user prefers dark themes", now)
	ms.addEntry("t1", "u1", "k3", "unrelated memory", now)

	stub := &stubChatClient{response: "1,2"}
	c := NewMemoryCurator(ms, stub, DefaultCuratorConfig())
	result, err := c.CurateWithLLM(context.Background(), "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Pruned != 1 {
		t.Errorf("expected 1 pruned from merge, got %d", result.Pruned)
	}
	if result.Kept != 2 {
		t.Errorf("expected 2 kept after merge, got %d", result.Kept)
	}
}

func TestCurateWithLLM_FallsBackOnError(t *testing.T) {
	ms := newMockMemoryStore()
	now := time.Now()
	ms.addEntry("t1", "u1", "k1", "content one", now)
	ms.addEntry("t1", "u1", "k2", "content two", now)

	stub := &stubChatClient{err: fmt.Errorf("LLM unavailable")}
	c := NewMemoryCurator(ms, stub, DefaultCuratorConfig())
	result, err := c.CurateWithLLM(context.Background(), "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 2 {
		t.Errorf("expected fallback to heuristic with 2 scanned, got %d", result.Scanned)
	}
}

func TestMergeContents(t *testing.T) {
	c := &MemoryCurator{}
	entries := []store.MemoryEntry{
		{Content: "first"},
		{Content: "second"},
		{Content: "third"},
	}
	merged := c.mergeContents(entries)
	if !strings.Contains(merged, "first") || !strings.Contains(merged, "second") || !strings.Contains(merged, "third") {
		t.Error("merged content should contain all parts")
	}
	if strings.Count(merged, "---") != 2 {
		t.Errorf("expected 2 separators in merged content, got %d", strings.Count(merged, "---"))
	}
}

func TestIsSimilar_ExactKeyMatch(t *testing.T) {
	c := &MemoryCurator{config: DefaultCuratorConfig()}
	a := store.MemoryEntry{Key: "preferences", Content: "abc"}
	b := store.MemoryEntry{Key: "PREFERENCES", Content: "xyz"}
	if !c.isSimilar(a, b) {
		t.Error("expected case-insensitive key match to be similar")
	}
}

func TestIsSimilar_DifferentKeysLowSimilarity(t *testing.T) {
	c := &MemoryCurator{config: DefaultCuratorConfig()}
	a := store.MemoryEntry{Key: "k1", Content: "alpha beta gamma"}
	b := store.MemoryEntry{Key: "k2", Content: "delta epsilon zeta"}
	if c.isSimilar(a, b) {
		t.Error("expected completely different content to not be similar")
	}
}

func TestFindStaleEntries(t *testing.T) {
	c := &MemoryCurator{config: DefaultCuratorConfig()}
	now := time.Now()
	entries := []store.MemoryEntry{
		{Key: "fresh", UpdatedAt: now},
		{Key: "stale", UpdatedAt: now.Add(-60 * 24 * time.Hour)},
	}
	stale := c.findStaleEntries(entries)
	if len(stale) != 1 || stale[0].Key != "stale" {
		t.Errorf("expected 1 stale entry, got %d", len(stale))
	}
}

// stubChatClient is a simple mock for chatCompleter.
type stubChatClient struct {
	response string
	err      error
}

func (s *stubChatClient) CreateChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &llm.ChatResponse{Content: s.response}, nil
}
