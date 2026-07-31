package transports

import (
	"testing"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

func TestDetectAPIMode(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		provider string
		baseURL  string
		want     llm.APIMode
	}{
		// Explicit mode always wins.
		{"explicit anthropic", "anthropic", "", "", llm.APIModeAnthropic},
		{"explicit anthropic_messages", "anthropic_messages", "", "", llm.APIModeAnthropic},
		{"explicit openai", "openai", "", "", llm.APIModeOpenAI},
		{"explicit chat_completions", "chat_completions", "", "", llm.APIModeOpenAI},

		// Provider-based detection.
		{"provider anthropic", "", "anthropic", "", llm.APIModeAnthropic},
		{"provider openai", "", "openai", "", llm.APIModeOpenAI},
		{"provider empty defaults openai", "", "", "", llm.APIModeOpenAI},

		// URL-based detection.
		{"url anthropic.com", "", "", "https://api.anthropic.com", llm.APIModeAnthropic},
		{"url anthropic variant", "", "", "https://anthropic.com/v1", llm.APIModeAnthropic},
		{"url openai", "", "", "https://api.openai.com/v1", llm.APIModeOpenAI},
		{"url empty", "", "", "", llm.APIModeOpenAI},

		// Case insensitivity.
		{"case explicit", "ANTHROPIC", "", "", llm.APIModeAnthropic},
		{"case provider", "", "Anthropic", "", llm.APIModeAnthropic},
		{"case url", "", "", "https://API.ANTHROPIC.COM", llm.APIModeAnthropic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectAPIMode(tt.explicit, tt.provider, tt.baseURL)
			if got != tt.want {
				t.Errorf("DetectAPIMode(%q, %q, %q) = %q, want %q",
					tt.explicit, tt.provider, tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestNewTransport_ReturnsCorrectType(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		baseURL  string
		mode     llm.APIMode
		wantName string
	}{
		{"openai default", "openai", "https://api.openai.com/v1", "", "openai"},
		{"anthropic explicit", "anthropic", "", llm.APIModeAnthropic, "anthropic"},
		{"anthropic auto-detect", "anthropic", "", "", "anthropic"},
		{"openai explicit", "", "", llm.APIModeOpenAI, "openai"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTransport(tt.provider, tt.baseURL, "test-key", "test-model", tt.mode)
			if tr == nil {
				t.Fatal("NewTransport returned nil")
			}
			if tr.Name() != tt.wantName {
				t.Errorf("Name() = %q, want %q", tr.Name(), tt.wantName)
			}
		})
	}
}
