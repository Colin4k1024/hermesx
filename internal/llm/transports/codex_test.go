package transports

import (
	"testing"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

func TestCodexTransport_Name(t *testing.T) {
	tr := NewCodexTransport("o3", "", "test-key")
	if tr.Name() != "codex" {
		t.Errorf("Name() = %q, want %q", tr.Name(), "codex")
	}
}

func TestNewCodexTransport_BaseURLNormalization(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "https://api.openai.com"},
		{"https://api.openai.com/v1", "https://api.openai.com"},
		{"https://api.openai.com/", "https://api.openai.com"},
		{"https://custom.api.com/v1/", "https://custom.api.com"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tr := NewCodexTransport("o3", tt.input, "key")
			if tr.baseURL != tt.want {
				t.Errorf("baseURL = %q, want %q", tr.baseURL, tt.want)
			}
		})
	}
}

func TestCodexTransport_buildRequest_BasicMessages(t *testing.T) {
	tr := NewCodexTransport("o3", "https://api.openai.com", "key")
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
		},
	}

	apiReq := tr.buildRequest(req)
	if apiReq.Model != "o3" {
		t.Errorf("Model = %q", apiReq.Model)
	}
	if len(apiReq.Input) != 3 {
		t.Fatalf("Input len = %d, want 3", len(apiReq.Input))
	}
	if apiReq.Input[0].Role != "system" {
		t.Errorf("Input[0].Role = %q", apiReq.Input[0].Role)
	}
	if apiReq.Input[1].Role != "user" {
		t.Errorf("Input[1].Role = %q", apiReq.Input[1].Role)
	}
}

func TestCodexTransport_buildRequest_ToolCallAndResponse(t *testing.T) {
	tr := NewCodexTransport("o3", "https://api.openai.com", "key")
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "What's the weather?"},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_abc",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"city":"Shanghai"}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				Content:    `{"temp": 30}`,
				ToolCallID: "call_abc",
			},
		},
	}

	apiReq := tr.buildRequest(req)
	if len(apiReq.Input) != 3 {
		t.Fatalf("Input len = %d, want 3", len(apiReq.Input))
	}

	// Assistant message with tool call.
	if apiReq.Input[1].Type != "function_call" {
		t.Errorf("Input[1].Type = %q, want function_call", apiReq.Input[1].Type)
	}
	if apiReq.Input[1].CallID != "call_abc" {
		t.Errorf("Input[1].CallID = %q", apiReq.Input[1].CallID)
	}
	if apiReq.Input[1].Name != "get_weather" {
		t.Errorf("Input[1].Name = %q", apiReq.Input[1].Name)
	}

	// Tool response.
	if apiReq.Input[2].Type != "function_call_output" {
		t.Errorf("Input[2].Type = %q, want function_call_output", apiReq.Input[2].Type)
	}
	if apiReq.Input[2].CallID != "call_abc" {
		t.Errorf("Input[2].CallID = %q", apiReq.Input[2].CallID)
	}
}

func TestCodexTransport_buildRequest_WithTools(t *testing.T) {
	tr := NewCodexTransport("o3", "https://api.openai.com", "key")
	req := llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "test"}},
		Tools: []llm.ToolDef{
			{Name: "search", Description: "Search the web"},
		},
	}

	apiReq := tr.buildRequest(req)
	if len(apiReq.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(apiReq.Tools))
	}
	if apiReq.Tools[0].Name != "search" {
		t.Errorf("Name = %q", apiReq.Tools[0].Name)
	}
	if apiReq.Tools[0].Type != "function" {
		t.Errorf("Type = %q", apiReq.Tools[0].Type)
	}
}

func TestCodexTransport_buildRequest_MaxTokensAndTemperature(t *testing.T) {
	tr := NewCodexTransport("o3", "https://api.openai.com", "key")
	temp := float32(0.3)
	req := llm.ChatRequest{
		Messages:    []llm.Message{{Role: "user", Content: "test"}},
		MaxTokens:   4096,
		Temperature: &temp,
	}

	apiReq := tr.buildRequest(req)
	if apiReq.MaxOutputTokens != 4096 {
		t.Errorf("MaxOutputTokens = %d", apiReq.MaxOutputTokens)
	}
	if apiReq.Temperature == nil || *apiReq.Temperature != 0.3 {
		t.Errorf("Temperature = %v", apiReq.Temperature)
	}
}

func TestCodexTransport_parseResponse_TextOutput(t *testing.T) {
	tr := NewCodexTransport("o3", "https://api.openai.com", "key")
	resp := &responsesResponse{
		Output: []responsesItem{
			{Type: "message", Text: "Hello world!"},
		},
		Usage: &responsesUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}

	result := tr.parseResponse(resp)
	if result.Content != "Hello world!" {
		t.Errorf("Content = %q", result.Content)
	}
	if result.FinishReason != "stop" {
		t.Errorf("FinishReason = %q", result.FinishReason)
	}
	if result.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d", result.Usage.PromptTokens)
	}
}

func TestCodexTransport_parseResponse_ToolCallOutput(t *testing.T) {
	tr := NewCodexTransport("o3", "https://api.openai.com", "key")
	resp := &responsesResponse{
		Output: []responsesItem{
			{
				Type:      "function_call",
				CallID:    "call_xyz",
				Name:      "search",
				Arguments: `{"query":"test"}`,
			},
		},
	}

	result := tr.parseResponse(resp)
	if result.FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q, want tool_calls", result.FinishReason)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("ToolCalls len = %d, want 1", len(result.ToolCalls))
	}
	if result.ToolCalls[0].ID != "call_xyz" {
		t.Errorf("ID = %q", result.ToolCalls[0].ID)
	}
	if result.ToolCalls[0].Function.Name != "search" {
		t.Errorf("Name = %q", result.ToolCalls[0].Function.Name)
	}
}

func TestCodexTransport_parseResponse_EmptyOutput(t *testing.T) {
	tr := NewCodexTransport("o3", "https://api.openai.com", "key")
	resp := &responsesResponse{}

	result := tr.parseResponse(resp)
	if result.Content != "" {
		t.Errorf("Content = %q, want empty", result.Content)
	}
	if result.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want stop", result.FinishReason)
	}
}
