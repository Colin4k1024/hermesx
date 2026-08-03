package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

// ContextManager handles conversation context compression and token management.
// Extracted from AIAgent to reduce god-object complexity and enable
// independent testing of compression logic.
type ContextManager struct {
	cfg                   CompressionConfig
	model                 string
	systemPrompt          string
	completer             chatCompleter
	summaryCompleter      chatCompleter // overrides completer for summaries; nil = use completer
	stream                *StreamHandler
	lastCompressionFailure time.Time
}

// ContextManagerConfig holds the configuration for creating a ContextManager.
type ContextManagerConfig struct {
	CompressionCfg  CompressionConfig
	Model           string
	SystemPrompt    string
	Completer       chatCompleter
	SummaryCompleter chatCompleter
	Stream          *StreamHandler
}

// NewContextManager creates a ContextManager from configuration.
func NewContextManager(cfg ContextManagerConfig) *ContextManager {
	return &ContextManager{
		cfg:              cfg.CompressionCfg,
		model:            cfg.Model,
		systemPrompt:     cfg.SystemPrompt,
		completer:        cfg.Completer,
		summaryCompleter: cfg.SummaryCompleter,
		stream:           cfg.Stream,
	}
}

// ShouldCompress returns true if the conversation should be compressed.
func (cm *ContextManager) ShouldCompress(messages []llm.Message) bool {
	if cm.isInCompressionCooldown() {
		return false
	}

	meta := llm.GetModelMeta(cm.model)
	totalTokens := estimateConversationTokens(messages, cm.systemPrompt)

	threshold := int(float64(meta.ContextLength) * cm.cfg.Threshold)
	return totalTokens > threshold
}

// CompressContext applies the configured compression strategy to free context space.
func (cm *ContextManager) CompressContext(ctx context.Context, messages []llm.Message) ([]llm.Message, error) {
	cfg := cm.cfg
	keepCount := cm.tailKeepCount(messages)

	if len(messages) <= keepCount {
		return messages, nil
	}

	slog.Info("Compressing context",
		"strategy", string(cfg.Strategy),
		"message_count", len(messages),
		"keep_count", keepCount,
	)
	cm.stream.FireStatus("Compressing context...")

	// Attempt compression with the summary completer first; if it fails and
	// retryWithMain is enabled, fall back to the main client (iterative, not
	// recursive, to avoid unbounded stack growth).
	type attempt struct {
		completer chatCompleter
		label     string
	}

	attempts := []attempt{{completer: cm.compressionCompleter(), label: "summary"}}
	if cfg.retryWithMainEnabled() && cm.summaryCompleter != nil {
		attempts = append(attempts, attempt{completer: cm.client(), label: "main"})
	}

	var lastErr error
	for _, a := range attempts {
		result, err := cm.compressWith(ctx, messages, keepCount, cfg, a.completer)
		if err == nil {
			return result, nil
		}
		lastErr = err
		slog.Warn("Compression attempt failed", "label", a.label, "error", err)
	}

	cm.recordCompressionFailure()
	return messages, lastErr
}

// client returns the primary (non-summary) LLM completer.
func (cm *ContextManager) client() chatCompleter {
	return cm.completer
}

// compressWith runs the configured strategy using a specific completer.
func (cm *ContextManager) compressWith(ctx context.Context, messages []llm.Message, keepCount int, cfg CompressionConfig, completer chatCompleter) ([]llm.Message, error) {
	switch cfg.Strategy {
	case StrategySlidingWindow:
		return compressSlidingWindow(messages, keepCount), nil
	case StrategySummarize:
		return cm.compressSummarizeWith(ctx, messages, keepCount, cfg, completer)
	case StrategyHybrid:
		return cm.compressHybridWith(ctx, messages, keepCount, cfg, completer)
	default:
		return cm.compressHybridWith(ctx, messages, keepCount, cfg, completer)
	}
}

// compressSummarize uses LLM to summarize older messages.
func (cm *ContextManager) compressSummarize(ctx context.Context, messages []llm.Message, keepCount int, cfg CompressionConfig) ([]llm.Message, error) {
	return cm.compressSummarizeWith(ctx, messages, keepCount, cfg, cm.compressionCompleter())
}

// compressSummarizeWith uses a specific completer to summarize older messages.
func (cm *ContextManager) compressSummarizeWith(ctx context.Context, messages []llm.Message, keepCount int, cfg CompressionConfig, completer chatCompleter) ([]llm.Message, error) {
	tail := messages[len(messages)-keepCount:]
	toSummarize := messages[:len(messages)-keepCount]

	summary, err := cm.generateSummaryWith(ctx, completer, toSummarize, cfg.SummaryMaxWords)
	if err != nil {
		return nil, err
	}

	result := make([]llm.Message, 0, keepCount+1)
	result = append(result, llm.Message{
		Role:    "user",
		Content: summaryHeader + "\n\n" + summary,
	})
	result = append(result, tail...)
	return result, nil
}

// compressHybrid keeps key messages + summarizes the rest.
func (cm *ContextManager) compressHybrid(ctx context.Context, messages []llm.Message, keepCount int, cfg CompressionConfig) ([]llm.Message, error) {
	return cm.compressHybridWith(ctx, messages, keepCount, cfg, cm.compressionCompleter())
}

// compressHybridWith keeps key messages + summarizes the rest using a specific completer.
func (cm *ContextManager) compressHybridWith(ctx context.Context, messages []llm.Message, keepCount int, cfg CompressionConfig, completer chatCompleter) ([]llm.Message, error) {
	// Extract key messages (system, first user, tool results with errors).
	keyIndices := extractKeyMessages(messages, keepCount)
	tail := messages[len(messages)-keepCount:]

	var toSummarize []llm.Message
	for i, msg := range messages {
		if i >= len(messages)-keepCount {
			break
		}
		if keyIndices[i] {
			continue
		}
		toSummarize = append(toSummarize, msg)
	}

	if len(toSummarize) == 0 {
		return messages, nil
	}

	summary, err := cm.generateSummaryWith(ctx, completer, toSummarize, cfg.SummaryMaxWords)
	if err != nil {
		return nil, err
	}

	// Extract tool spine from compressed messages for structural preservation.
	spine := ExtractToolSpine(toSummarize)
	spineText := FormatToolSpine(spine)

	content := fmt.Sprintf("[Context Summary -- %d messages compressed, %d key messages preserved]\n%s",
		len(toSummarize), len(keyIndices), summary)
	if spineText != "" {
		content += "\n" + spineText
	}

	result := make([]llm.Message, 0, len(keyIndices)+keepCount+1)
	result = append(result, llm.Message{
		Role:    "user",
		Content: content,
	})

	// Insert preserved key messages before the tail.
	for i, msg := range messages {
		if i >= len(messages)-keepCount {
			break
		}
		if keyIndices[i] {
			result = append(result, msg)
		}
	}

	result = append(result, tail...)
	return result, nil
}

// generateSummary produces a summary of the given messages.
func (cm *ContextManager) generateSummary(ctx context.Context, messages []llm.Message, maxWords int) (string, error) {
	completer := cm.compressionCompleter()
	return cm.generateSummaryWith(ctx, completer, messages, maxWords)
}

// generateSummaryWith produces a summary using a specific completer.
func (cm *ContextManager) generateSummaryWith(ctx context.Context, completer chatCompleter, messages []llm.Message, maxWords int) (string, error) {
	pruned := pruneToolResults(messages)
	summaryPrompt := buildSummaryPrompt(pruned, maxWords)

	req := llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: summaryPrompt}},
		MaxTokens: maxWords * 2,
	}

	resp, err := completer.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

// compressionCompleter returns the LLM client to use for summaries.
func (cm *ContextManager) compressionCompleter() chatCompleter {
	if cm.summaryCompleter != nil {
		return cm.summaryCompleter
	}
	return cm.completer
}

// tailKeepCount computes how many recent messages to keep based on token budget.
func (cm *ContextManager) tailKeepCount(messages []llm.Message) int {
	if cm.cfg.KeepCount > 0 {
		return cm.cfg.KeepCount
	}

	meta := llm.GetModelMeta(cm.model)
	contextWindow := meta.ContextLength
	if cm.cfg.ContextWindow > 0 {
		contextWindow = cm.cfg.ContextWindow
	}
	if contextWindow == 0 {
		return 6
	}

	tailBudget := int(float64(contextWindow) * tailBudgetFraction)
	keepCount := 0
	tokenCount := 0

	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := estimateTokens(messages[i].Content)
		if tokenCount+msgTokens > tailBudget {
			break
		}
		tokenCount += msgTokens
		keepCount++
	}

	if keepCount < 2 {
		keepCount = 2
	}
	return keepCount
}

// isInCompressionCooldown returns true if a recent compression failure
// should prevent retrying.
func (cm *ContextManager) isInCompressionCooldown() bool {
	if cm.lastCompressionFailure.IsZero() {
		return false
	}
	return time.Since(cm.lastCompressionFailure) < compressionFailureCooldown
}

// recordCompressionFailure records the time of a compression failure.
func (cm *ContextManager) recordCompressionFailure() {
	cm.lastCompressionFailure = time.Now()
}
