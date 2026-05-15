package llm

import "testing"

func TestResolveModelAlias(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"opus", "anthropic/claude-opus-4-20250514"},
		{"Opus", "anthropic/claude-opus-4-20250514"},
		{"SONNET", "anthropic/claude-sonnet-4-20250514"},
		{"haiku", "anthropic/claude-haiku-4-20250414"},
		{"gpt4o", "openai/gpt-4o"},
		{"flash", "google/gemini-2.5-flash"},
		{"r1", "deepseek/deepseek-r1"},
		{"deepseek", "deepseek/deepseek-chat"},
		// Non-aliases pass through unchanged
		{"anthropic/claude-sonnet-4-20250514", "anthropic/claude-sonnet-4-20250514"},
		{"openai/gpt-4o", "openai/gpt-4o"},
		{"unknown-model", "unknown-model"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ResolveModelAlias(tt.input)
			if got != tt.expected {
				t.Errorf("ResolveModelAlias(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsModelAlias(t *testing.T) {
	if !IsModelAlias("opus") {
		t.Error("expected 'opus' to be an alias")
	}
	if IsModelAlias("anthropic/claude-opus-4-20250514") {
		t.Error("expected full model name not to be an alias")
	}
}
