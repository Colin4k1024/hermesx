package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

// chatCompleter is the subset of llm.Client used by context compression.
type chatCompleter interface {
	CreateChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

// CompressionStrategy defines which compression approach to use.
type CompressionStrategy string

const (
	// StrategySlidingWindow keeps the most recent N messages, drops the oldest.
	StrategySlidingWindow CompressionStrategy = "sliding_window"

	// StrategySummarize uses LLM to summarize older messages.
	StrategySummarize CompressionStrategy = "summarize"

	// StrategyHybrid keeps key messages + summarizes the rest (default).
	StrategyHybrid CompressionStrategy = "hybrid"
)

const (
	// toolResultPruneThreshold is the character count above which tool results
	// are replaced with a short placeholder before LLM summarisation.
	toolResultPruneThreshold = 500

	// toolResultPreviewLen is how many leading characters of a pruned tool
	// result to keep in the placeholder.
	toolResultPreviewLen = 100

	// tailBudgetFraction is the fraction of the context window reserved for
	// recent (tail) messages that are never compressed.
	tailBudgetFraction = 0.25

	// compressionFailureCooldown is the minimum duration between compression
	// attempts after a failure.
	compressionFailureCooldown = 10 * time.Minute

	// summaryHeader is the markdown header that marks a previous conversation
	// summary inside the message stream.
	summaryHeader = "## Conversation Summary"
)

// CompressionConfig controls context compression behavior.
type CompressionConfig struct {
	// Threshold is the fraction of context window that triggers compression (0.0-1.0).
	Threshold float64

	// Strategy selects which compression approach to use.
	Strategy CompressionStrategy

	// KeepCount is the minimum number of recent messages to preserve.
	// Deprecated: token-budget tail protection is used when ContextWindow > 0.
	KeepCount int

	// SummaryMaxWords is the target length for LLM summaries.
	SummaryMaxWords int

	// ContextWindow overrides the model's context length for tail-budget
	// calculation.  When zero the model metadata is used.
	ContextWindow int

	// RetryWithMain controls whether compression retries with the main model
	// when the summary model fails. Default: true.
	RetryWithMain *bool
}

// DefaultCompressionConfig returns sensible defaults.
func DefaultCompressionConfig() CompressionConfig {
	retryEnabled := true
	return CompressionConfig{
		Threshold:       0.75,
		Strategy:        StrategyHybrid,
		KeepCount:       6,
		SummaryMaxWords: 500,
		RetryWithMain:   &retryEnabled,
	}
}

// retryWithMainEnabled returns true if the config allows retry with main model.
func (c CompressionConfig) retryWithMainEnabled() bool {
	return c.RetryWithMain == nil || *c.RetryWithMain
}

// ShouldCompress returns true if the conversation should be compressed.
// Delegates to ContextManager.
func (a *AIAgent) ShouldCompress(messages []llm.Message) bool {
	return a.contextManager.ShouldCompress(messages)
}

// CompressContext applies the configured compression strategy to free context space.
// Delegates to ContextManager.
func (a *AIAgent) CompressContext(ctx context.Context, messages []llm.Message) ([]llm.Message, error) {
	return a.contextManager.CompressContext(ctx, messages)
}

// --- Sliding window strategy (standalone) ---

func compressSlidingWindow(messages []llm.Message, keepCount int) []llm.Message {
	if keepCount > len(messages) {
		keepCount = len(messages)
	}

	kept := messages[len(messages)-keepCount:]
	droppedCount := len(messages) - keepCount

	result := make([]llm.Message, 0, keepCount+1)
	result = append(result, llm.Message{
		Role:    "system",
		Content: fmt.Sprintf("[Context Note: %d earlier messages were dropped to fit context window]", droppedCount),
	})
	result = append(result, kept...)

	slog.Info("Sliding window compression",
		"dropped", droppedCount,
		"kept", keepCount,
	)
	return result
}

// isKeyMessage returns true if a message should be preserved during compression.
// Key messages include: system prompts, user corrections/decisions, assistant
// corrections, and short messages that are cheap in tokens.
func isKeyMessage(m llm.Message) bool {
	if m.Role == "system" {
		return true
	}

	lower := strings.ToLower(m.Content)

	// User corrections / decisions
	if m.Role == "user" {
		correctionMarkers := []string{
			"no, ", "wrong", "don't ", "do not ", "stop ", "instead ",
			"actually ", "correction:", "important:", "remember:",
			"decision:", "note:", "always ", "never ",
		}
		for _, marker := range correctionMarkers {
			if strings.Contains(lower, marker) {
				return true
			}
		}
	}

	// Assistant corrections / feedback
	if m.Role == "assistant" {
		correctionMarkers := []string{
			"correction:", "actually", "i was wrong", "let me fix",
			"apologize", "mistake",
		}
		for _, marker := range correctionMarkers {
			if strings.Contains(lower, marker) {
				return true
			}
		}
	}

	// Short messages are often corrections or confirmations — keep them
	// (they're cheap in tokens anyway)
	if len(m.Content) < 100 {
		return true
	}

	return false
}

// extractKeyMessages returns a set of indices for messages that should be preserved.
func extractKeyMessages(messages []llm.Message, keepCount int) map[int]bool {
	result := make(map[int]bool)
	for i, m := range messages {
		if i >= len(messages)-keepCount {
			break
		}
		if isKeyMessage(m) {
			result[i] = true
		}
	}
	return result
}

// pruneToolResults replaces oversized tool results with placeholders.
func pruneToolResults(messages []llm.Message) []llm.Message {
	result := make([]llm.Message, len(messages))
	copy(result, messages)
	for i, m := range result {
		if m.Role == "tool" && len(m.Content) > toolResultPruneThreshold {
			preview := m.Content
			if len(preview) > toolResultPreviewLen {
				preview = preview[:toolResultPreviewLen]
			}
			result[i].Content = fmt.Sprintf("[Tool result truncated: %s... (%d chars)]", preview, len(m.Content))
		}
	}
	return result
}

// buildSummaryPrompt creates a prompt for the LLM to summarize messages.
func buildSummaryPrompt(messages []llm.Message, maxWords int) string {
	var sb strings.Builder
	sb.WriteString("Please summarize the following conversation in under ")
	sb.WriteString(fmt.Sprintf("%d words. ", maxWords))
	sb.WriteString("Focus on key decisions, action items, and important context.\n\n")

	for _, m := range messages {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n\n", m.Role, m.Content))
	}

	return sb.String()
}

// estimateTokens gives a rough token count for a string.
func estimateTokens(text string) int {
	return len(text) / 4
}

// estimateConversationTokens estimates total tokens for a conversation.
func estimateConversationTokens(messages []llm.Message, systemPrompt string) int {
	total := estimateTokens(systemPrompt)
	for _, m := range messages {
		total += estimateTokens(m.Content)
	}
	return total
}

// ToolSpineEntry represents a tool call with its result status.
type ToolSpineEntry struct {
	ToolName  string
	Success   bool
	KeyResult string
}

// ExtractToolSpine extracts tool call/result pairs for structural preservation.
func ExtractToolSpine(messages []llm.Message) []ToolSpineEntry {
	var spine []ToolSpineEntry
	for i, m := range messages {
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			// Find matching tool results for each call.
			for _, tc := range m.ToolCalls {
				entry := ToolSpineEntry{ToolName: tc.Function.Name}
				for j := i + 1; j < len(messages); j++ {
					if messages[j].Role == "tool" && messages[j].ToolCallID == tc.ID {
						entry.Success = !strings.Contains(messages[j].Content, "error")
						entry.KeyResult = truncate(messages[j].Content, 200)
						break
					}
				}
				spine = append(spine, entry)
			}
		}
	}
	return spine
}

// FormatToolSpine converts a tool spine into a human-readable string
// for embedding in the context summary.
func FormatToolSpine(spine []ToolSpineEntry) string {
	if len(spine) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("### Tool Call History\n")
	for _, entry := range spine {
		status := "ok"
		if !entry.Success {
			status = "FAIL"
		}
		sb.WriteString(fmt.Sprintf("- %s [%s]", entry.ToolName, status))
		if entry.KeyResult != "" {
			sb.WriteString(fmt.Sprintf(": %s", truncate(entry.KeyResult, 100)))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
