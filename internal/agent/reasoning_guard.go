package agent

import "github.com/hermes-agent/hermes-agent-go/internal/llm"

// StripReasoningForProvider returns a copy of messages with reasoning content
// removed to prevent cross-provider reasoning leaks during fallback.
func StripReasoningForProvider(messages []llm.Message, targetProvider string) []llm.Message {
	result := make([]llm.Message, len(messages))
	for i, m := range messages {
		result[i] = m
		result[i].Reasoning = ""
		result[i].ReasoningContent = ""
	}
	return result
}
