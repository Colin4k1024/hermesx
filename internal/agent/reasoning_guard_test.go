package agent

import (
	"testing"

	"github.com/hermes-agent/hermes-agent-go/internal/llm"
)

func TestStripReasoningForProvider(t *testing.T) {
	messages := []llm.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "response", Reasoning: "internal thought", ReasoningContent: "detailed reasoning"},
		{Role: "user", Content: "follow up"},
	}

	stripped := StripReasoningForProvider(messages, "openai")

	if stripped[1].Reasoning != "" {
		t.Errorf("Expected Reasoning to be stripped, got '%s'", stripped[1].Reasoning)
	}
	if stripped[1].ReasoningContent != "" {
		t.Errorf("Expected ReasoningContent to be stripped, got '%s'", stripped[1].ReasoningContent)
	}
	if stripped[1].Content != "response" {
		t.Errorf("Expected Content preserved, got '%s'", stripped[1].Content)
	}

	// original unmodified
	if messages[1].Reasoning != "internal thought" {
		t.Error("Original message should not be modified")
	}
}

func TestStripReasoningForProvider_NoReasoning(t *testing.T) {
	messages := []llm.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "response"},
	}

	stripped := StripReasoningForProvider(messages, "anthropic")
	if len(stripped) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(stripped))
	}
	if stripped[0].Content != "hello" {
		t.Errorf("Expected content preserved")
	}
}

func TestStripReasoningForProvider_Empty(t *testing.T) {
	stripped := StripReasoningForProvider(nil, "openai")
	if len(stripped) != 0 {
		t.Errorf("Expected empty slice for nil input")
	}
}
