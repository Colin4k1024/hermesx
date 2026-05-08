package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/store"
)

// SelfImproveConfig controls the self-improvement loop behavior.
type SelfImproveConfig struct {
	Enabled bool

	// MinTurnsBeforeReview is how many turns must pass before triggering a review.
	MinTurnsBeforeReview int

	// ReviewInterval controls how often reviews happen (every N turns after min).
	ReviewInterval int

	// MaxInsights caps the number of stored improvement insights.
	MaxInsights int
}

// DefaultSelfImproveConfig returns sensible defaults.
func DefaultSelfImproveConfig() SelfImproveConfig {
	return SelfImproveConfig{
		Enabled:              true,
		MinTurnsBeforeReview: 5,
		ReviewInterval:       5,
		MaxInsights:          20,
	}
}

// SelfImproveResult holds the outcome of a self-review.
type SelfImproveResult struct {
	TurnCount     int
	ReviewedTurns int
	Insights      []string
	Timestamp     time.Time
}

// SelfImprover performs periodic self-reflection on conversation quality.
type SelfImprover struct {
	mu        sync.Mutex
	completer chatCompleter
	store     store.MemoryStore
	config    SelfImproveConfig
	turnCount int
}

// NewSelfImprover creates a self-improvement loop controller.
func NewSelfImprover(completer chatCompleter, ms store.MemoryStore, cfg SelfImproveConfig) *SelfImprover {
	return &SelfImprover{
		completer: completer,
		store:     ms,
		config:    cfg,
	}
}

// RecordTurn increments the turn counter and returns true if a review should
// be triggered.
func (s *SelfImprover) RecordTurn() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.turnCount++
	if !s.config.Enabled || s.completer == nil {
		return false
	}
	if s.config.ReviewInterval <= 0 {
		return false
	}
	if s.turnCount < s.config.MinTurnsBeforeReview {
		return false
	}
	return (s.turnCount-s.config.MinTurnsBeforeReview)%s.config.ReviewInterval == 0
}

// TurnCount returns the current turn count.
func (s *SelfImprover) TurnCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.turnCount
}

// Review performs LLM-assisted self-reflection on recent conversation turns.
// It returns actionable insights that can be stored as behavioral memories.
func (s *SelfImprover) Review(ctx context.Context, recentMessages []llm.Message, tenantID, userID string) (*SelfImproveResult, error) {
	s.mu.Lock()
	turnCount := s.turnCount
	s.mu.Unlock()

	if !s.config.Enabled || s.completer == nil {
		return &SelfImproveResult{TurnCount: turnCount}, nil
	}

	if len(recentMessages) < 2 {
		return &SelfImproveResult{TurnCount: turnCount}, nil
	}

	prompt := buildReviewPrompt(recentMessages)
	resp, err := s.completer.CreateChatCompletion(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("self-improvement review: %w", err)
	}

	insights := parseInsights(resp.Content)
	result := &SelfImproveResult{
		TurnCount:     turnCount,
		ReviewedTurns: len(recentMessages),
		Insights:      insights,
		Timestamp:     time.Now(),
	}

	if s.store != nil && len(insights) > 0 {
		if err := s.persistInsights(ctx, tenantID, userID, insights); err != nil {
			slog.Warn("Failed to persist self-improvement insights", "error", err)
		}
	}

	slog.Info("Self-improvement review complete",
		"turn_count", turnCount,
		"reviewed_turns", len(recentMessages),
		"insights_generated", len(insights),
	)
	return result, nil
}

// buildReviewPrompt creates the self-reflection prompt from recent messages.
func buildReviewPrompt(messages []llm.Message) string {
	var sb strings.Builder
	sb.WriteString("Review the following conversation excerpt and identify 1-3 specific, actionable improvements for the assistant's responses. ")
	sb.WriteString("Focus on: clarity, helpfulness, accuracy, and tone appropriateness. ")
	sb.WriteString("Return each insight on its own line prefixed with '- '. ")
	sb.WriteString("If the responses are already optimal, return 'NONE'.\n\n")

	for _, m := range messages {
		content := sanitizeForPrompt(m.Content, 300)
		role := sanitizeForPrompt(m.Role, 20)
		fmt.Fprintf(&sb, "[%s]: %s\n", role, content)
	}
	return sb.String()
}

// parseInsights extracts bullet-point insights from the LLM response.
func parseInsights(response string) []string {
	response = strings.TrimSpace(response)
	if strings.EqualFold(response, "NONE") || response == "" {
		return nil
	}

	var insights []string
	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			insight := strings.TrimPrefix(line, "- ")
			if insight != "" {
				insights = append(insights, insight)
			}
		}
	}
	return insights
}

// persistInsights stores insights as behavioral memory entries, enforcing MaxInsights cap.
func (s *SelfImprover) persistInsights(ctx context.Context, tenantID, userID string, insights []string) error {
	if s.config.MaxInsights > 0 && len(insights) > s.config.MaxInsights {
		insights = insights[:s.config.MaxInsights]
	}

	// Evict oldest insight entries if over cap.
	if s.config.MaxInsights > 0 {
		entries, err := s.store.List(ctx, tenantID, userID)
		if err == nil {
			var insightKeys []string
			for _, e := range entries {
				if strings.HasPrefix(e.Key, "_self_improvement_") {
					insightKeys = append(insightKeys, e.Key)
				}
			}
			// Sort ascending so oldest (smallest timestamp) comes first.
			sort.Strings(insightKeys)
			overflow := len(insightKeys) + 1 - s.config.MaxInsights
			for i := 0; i < overflow && i < len(insightKeys); i++ {
				_ = s.store.Delete(ctx, tenantID, userID, insightKeys[i])
			}
		}
	}

	key := fmt.Sprintf("_self_improvement_%d", time.Now().UnixNano())
	content := strings.Join(insights, "\n")
	return s.store.Upsert(ctx, tenantID, userID, key, content)
}
