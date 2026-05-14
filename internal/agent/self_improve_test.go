package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/store"
)

func TestRecordTurn_Disabled(t *testing.T) {
	cfg := DefaultSelfImproveConfig()
	cfg.Enabled = false
	s := NewSelfImprover(&stubChatClient{response: "NONE"}, nil, cfg)

	for i := 0; i < 20; i++ {
		if s.RecordTurn() {
			t.Fatal("disabled self-improver should never trigger review")
		}
	}
}

func TestRecordTurn_NilCompleter(t *testing.T) {
	s := NewSelfImprover(nil, nil, DefaultSelfImproveConfig())

	for i := 0; i < 20; i++ {
		if s.RecordTurn() {
			t.Fatal("nil completer should never trigger review")
		}
	}
}

func TestRecordTurn_TriggersAtCorrectInterval(t *testing.T) {
	cfg := SelfImproveConfig{
		Enabled:              true,
		MinTurnsBeforeReview: 3,
		ReviewInterval:       2,
		MaxInsights:          10,
	}
	s := NewSelfImprover(&stubChatClient{response: "NONE"}, nil, cfg)

	var triggers []int
	for i := 1; i <= 15; i++ {
		if s.RecordTurn() {
			triggers = append(triggers, i)
		}
	}

	// MinTurns=3, Interval=2: triggers at turn 3, 5, 7, 9, 11, 13, 15
	expected := []int{3, 5, 7, 9, 11, 13, 15}
	if len(triggers) != len(expected) {
		t.Fatalf("expected triggers at %v, got %v", expected, triggers)
	}
	for i, v := range expected {
		if triggers[i] != v {
			t.Errorf("trigger %d: expected turn %d, got %d", i, v, triggers[i])
		}
	}
}

func TestRecordTurn_DefaultConfig(t *testing.T) {
	cfg := DefaultSelfImproveConfig()
	s := NewSelfImprover(&stubChatClient{response: "NONE"}, nil, cfg)

	var triggers []int
	for i := 1; i <= 20; i++ {
		if s.RecordTurn() {
			triggers = append(triggers, i)
		}
	}
	// MinTurns=5, Interval=5: triggers at turn 5, 10, 15, 20
	expected := []int{5, 10, 15, 20}
	if len(triggers) != len(expected) {
		t.Fatalf("expected triggers at %v, got %v", expected, triggers)
	}
}

func TestTurnCount(t *testing.T) {
	s := NewSelfImprover(nil, nil, DefaultSelfImproveConfig())
	if s.TurnCount() != 0 {
		t.Error("initial turn count should be 0")
	}
	s.RecordTurn()
	s.RecordTurn()
	if s.TurnCount() != 2 {
		t.Errorf("expected turn count 2, got %d", s.TurnCount())
	}
}

func TestReview_Disabled(t *testing.T) {
	cfg := DefaultSelfImproveConfig()
	cfg.Enabled = false
	s := NewSelfImprover(&stubChatClient{response: "- be clearer"}, nil, cfg)

	msgs := []llm.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	result, err := s.Review(context.Background(), msgs, "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Insights) != 0 {
		t.Error("disabled review should produce no insights")
	}
}

func TestReview_TooFewMessages(t *testing.T) {
	s := NewSelfImprover(&stubChatClient{response: "- insight"}, nil, DefaultSelfImproveConfig())

	msgs := []llm.Message{{Role: "user", Content: "hello"}}
	result, err := s.Review(context.Background(), msgs, "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Insights) != 0 {
		t.Error("should not review with fewer than 2 messages")
	}
}

func TestReview_ExtractsInsights(t *testing.T) {
	stub := &stubChatClient{response: "- Be more concise\n- Add examples\n- Use simpler language"}
	s := NewSelfImprover(stub, nil, DefaultSelfImproveConfig())

	msgs := []llm.Message{
		{Role: "user", Content: "explain quantum computing"},
		{Role: "assistant", Content: "quantum computing uses qubits..."},
	}
	result, err := s.Review(context.Background(), msgs, "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Insights) != 3 {
		t.Fatalf("expected 3 insights, got %d", len(result.Insights))
	}
	if result.Insights[0] != "Be more concise" {
		t.Errorf("unexpected first insight: %s", result.Insights[0])
	}
}

func TestReview_NoneResponse(t *testing.T) {
	stub := &stubChatClient{response: "NONE"}
	s := NewSelfImprover(stub, nil, DefaultSelfImproveConfig())

	msgs := []llm.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}
	result, err := s.Review(context.Background(), msgs, "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Insights) != 0 {
		t.Errorf("expected 0 insights for NONE, got %d", len(result.Insights))
	}
}

func TestReview_LLMError(t *testing.T) {
	stub := &stubChatClient{err: fmt.Errorf("model overloaded")}
	s := NewSelfImprover(stub, nil, DefaultSelfImproveConfig())

	msgs := []llm.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	_, err := s.Review(context.Background(), msgs, "t1", "u1")
	if err == nil {
		t.Error("expected error when LLM fails")
	}
}

func TestReview_PersistsInsights(t *testing.T) {
	stub := &stubChatClient{response: "- Remember to ask clarifying questions"}
	ms := newMockMemoryStore()
	s := NewSelfImprover(stub, ms, DefaultSelfImproveConfig())

	msgs := []llm.Message{
		{Role: "user", Content: "do the thing"},
		{Role: "assistant", Content: "done"},
	}
	_, err := s.Review(context.Background(), msgs, "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}

	entries, _ := ms.List(context.Background(), "t1", "u1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 persisted insight entry, got %d", len(entries))
	}
	if entries[0].Content != "Remember to ask clarifying questions" {
		t.Errorf("unexpected persisted content: %s", entries[0].Content)
	}
}

func TestParseInsights_MixedContent(t *testing.T) {
	input := "Here are suggestions:\n- First insight\nSome text\n- Second insight\n\n- Third insight"
	insights := parseInsights(input)
	if len(insights) != 3 {
		t.Fatalf("expected 3 insights, got %d: %v", len(insights), insights)
	}
}

func TestParseInsights_EmptyBullets(t *testing.T) {
	input := "- \n- actual insight\n- "
	insights := parseInsights(input)
	if len(insights) != 1 {
		t.Fatalf("expected 1 insight (skip empty bullets), got %d", len(insights))
	}
}

func TestBuildReviewPrompt_TruncatesLongMessages(t *testing.T) {
	longContent := ""
	for i := 0; i < 100; i++ {
		longContent += "word "
	}
	msgs := []llm.Message{
		{Role: "user", Content: longContent},
		{Role: "assistant", Content: "short reply"},
	}
	prompt := buildReviewPrompt(msgs)
	if len(prompt) > 2000 {
		t.Errorf("prompt should be reasonably sized, got %d chars", len(prompt))
	}
	if !contains(prompt, "...") {
		t.Error("expected truncation marker in prompt")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestRecordTurn_ReviewIntervalZero(t *testing.T) {
	cfg := SelfImproveConfig{
		Enabled:              true,
		MinTurnsBeforeReview: 1,
		ReviewInterval:       0,
		MaxInsights:          5,
	}
	s := NewSelfImprover(&stubChatClient{response: "NONE"}, nil, cfg)
	for i := 0; i < 20; i++ {
		if s.RecordTurn() {
			t.Fatal("ReviewInterval=0 should never trigger review")
		}
	}
}

func TestPersistInsights_MaxInsightsEnforced(t *testing.T) {
	ms := &mockSelfImproveStore{memories: make(map[string]string)}
	cfg := SelfImproveConfig{
		Enabled:              true,
		MinTurnsBeforeReview: 5,
		ReviewInterval:       5,
		MaxInsights:          3,
	}
	s := NewSelfImprover(&stubChatClient{response: "- a\n- b\n- c"}, ms, cfg)

	msgs := []llm.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	// Simulate multiple reviews to accumulate insights.
	for i := 0; i < 5; i++ {
		s.Review(context.Background(), msgs, "t1", "u1")
	}

	// Count insight keys - should not exceed MaxInsights.
	count := 0
	for k := range ms.memories {
		if len(k) > 18 && k[:18] == "_self_improvement_" {
			count++
		}
	}
	if count > cfg.MaxInsights {
		t.Errorf("expected at most %d insight entries, got %d", cfg.MaxInsights, count)
	}
}

func TestSanitizeForPrompt(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		expect string
	}{
		{"normal text", 50, "normal text"},
		{"has\x00control\x01chars", 50, "hascontrolchars"},
		{"newlines\nare\tok", 50, "newlines\nare\tok"},
		{"long string here", 4, "long..."},
		{"", 10, ""},
		{"hello 世界", 7, "hello 世..."},
		// bidi override characters must be stripped
		{"prefix‮malicious", 50, "prefixmalicious"},
		{"a‎b‏c‫d", 50, "abcd"},
		{"⁦⁩inject", 50, "inject"},
	}
	for _, tc := range tests {
		got := sanitizeForPrompt(tc.input, tc.maxLen)
		if got != tc.expect {
			t.Errorf("sanitizeForPrompt(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.expect)
		}
	}
}

type mockSelfImproveStore struct {
	memories map[string]string
}

func (m *mockSelfImproveStore) Get(_ context.Context, _, _, key string) (string, error) {
	return m.memories[key], nil
}
func (m *mockSelfImproveStore) List(_ context.Context, _, _ string) ([]store.MemoryEntry, error) {
	var entries []store.MemoryEntry
	for k, v := range m.memories {
		entries = append(entries, store.MemoryEntry{Key: k, Content: v})
	}
	return entries, nil
}
func (m *mockSelfImproveStore) Upsert(_ context.Context, _, _, key, content string) error {
	m.memories[key] = content
	return nil
}
func (m *mockSelfImproveStore) Delete(_ context.Context, _, _, key string) error {
	delete(m.memories, key)
	return nil
}
func (m *mockSelfImproveStore) DeleteAllByUser(_ context.Context, _, _ string) (int64, error) {
	return 0, nil
}
func (m *mockSelfImproveStore) DeleteAllByTenant(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
