package transports

import (
	"strings"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

// NewTransport creates the appropriate transport based on provider/mode detection.
func NewTransport(provider, baseURL, apiKey, model string, mode llm.APIMode) llm.Transport {
	if mode == "" {
		mode = DetectAPIMode("", provider, baseURL)
	}

	switch mode {
	case llm.APIModeAnthropic:
		return NewAnthropicTransport(model, baseURL, apiKey, provider)
	default:
		return NewOpenAITransport(model, baseURL, apiKey)
	}
}

// DetectAPIMode auto-detects the API mode from provider and base URL.
func DetectAPIMode(explicit, provider, baseURL string) llm.APIMode {
	switch strings.ToLower(explicit) {
	case "anthropic", "anthropic_messages":
		return llm.APIModeAnthropic
	case "openai", "chat_completions", "":
		// fall through to auto-detect
	}

	if provider == "anthropic" {
		return llm.APIModeAnthropic
	}

	lower := strings.ToLower(baseURL)
	if strings.Contains(lower, "anthropic.com") {
		return llm.APIModeAnthropic
	}

	return llm.APIModeOpenAI
}
