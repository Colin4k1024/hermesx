package transports

import (
	"encoding/json"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

func TestGeminiTransport_Name(t *testing.T) {
	tr := NewGeminiTransport("gemini-2.0-flash", "test-key")
	if tr.Name() != "gemini" {
		t.Errorf("Name() = %q, want %q", tr.Name(), "gemini")
	}
}

func TestNewGeminiTransport_StripsPrefix(t *testing.T) {
	tests := []struct {
		input    string
		wantModel string
	}{
		{"gemini-2.0-flash", "gemini-2.0-flash"},
		{"google/gemini-2.0-flash", "gemini-2.0-flash"},
		{"gemini/gemini-2.0-flash", "gemini-2.0-flash"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tr := NewGeminiTransport(tt.input, "key")
			if tr.model != tt.wantModel {
				t.Errorf("model = %q, want %q", tr.model, tt.wantModel)
			}
		})
	}
}

func TestGeminiTransport_buildRequest_BasicMessages(t *testing.T) {
	tr := NewGeminiTransport("gemini-2.0-flash", "test-key")
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
			{Role: "user", Content: "How are you?"},
		},
	}

	gemReq := tr.buildRequest(req)

	// System message goes to SystemInstruction.
	if gemReq.SystemInstruction == nil {
		t.Fatal("SystemInstruction is nil")
	}
	if gemReq.SystemInstruction.Parts[0].Text != "You are helpful." {
		t.Errorf("SystemInstruction text = %q", gemReq.SystemInstruction.Parts[0].Text)
	}

	// User/assistant messages go to Contents.
	if len(gemReq.Contents) != 3 {
		t.Fatalf("Contents len = %d, want 3", len(gemReq.Contents))
	}
	if gemReq.Contents[0].Role != "user" {
		t.Errorf("Contents[0].Role = %q", gemReq.Contents[0].Role)
	}
	if gemReq.Contents[1].Role != "model" {
		t.Errorf("Contents[1].Role = %q", gemReq.Contents[1].Role)
	}
}

func TestGeminiTransport_buildRequest_ToolMessage(t *testing.T) {
	tr := NewGeminiTransport("gemini-2.0-flash", "test-key")
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "user", Content: "What's the weather?"},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"city":"Beijing"}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				Content:    `{"temp": 25}`,
				ToolCallID: "call_1",
				ToolName:   "get_weather",
			},
		},
	}

	gemReq := tr.buildRequest(req)
	if len(gemReq.Contents) != 3 {
		t.Fatalf("Contents len = %d, want 3", len(gemReq.Contents))
	}

	// Second content should have functionCall part.
	assistantContent := gemReq.Contents[1]
	if len(assistantContent.Parts) != 1 {
		t.Fatalf("assistant parts len = %d, want 1", len(assistantContent.Parts))
	}
	if assistantContent.Parts[0].FunctionCall == nil {
		t.Fatal("FunctionCall is nil")
	}
	if assistantContent.Parts[0].FunctionCall.Name != "get_weather" {
		t.Errorf("FunctionCall.Name = %q", assistantContent.Parts[0].FunctionCall.Name)
	}

	// Third content should have functionResponse part.
	toolContent := gemReq.Contents[2]
	if len(toolContent.Parts) != 1 {
		t.Fatalf("tool parts len = %d, want 1", len(toolContent.Parts))
	}
	if toolContent.Parts[0].FunctionResponse == nil {
		t.Fatal("FunctionResponse is nil")
	}
	if toolContent.Parts[0].FunctionResponse.Name != "get_weather" {
		t.Errorf("FunctionResponse.Name = %q", toolContent.Parts[0].FunctionResponse.Name)
	}
}

func TestGeminiTransport_buildRequest_WithTools(t *testing.T) {
	tr := NewGeminiTransport("gemini-2.0-flash", "test-key")
	req := llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "test"}},
		Tools: []llm.ToolDef{
			{Name: "search", Description: "Search", Parameters: map[string]any{"type": "object"}},
		},
	}

	gemReq := tr.buildRequest(req)
	if len(gemReq.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(gemReq.Tools))
	}
	if len(gemReq.Tools[0].FunctionDeclarations) != 1 {
		t.Fatalf("FunctionDeclarations len = %d, want 1", len(gemReq.Tools[0].FunctionDeclarations))
	}
	if gemReq.Tools[0].FunctionDeclarations[0].Name != "search" {
		t.Errorf("Name = %q", gemReq.Tools[0].FunctionDeclarations[0].Name)
	}
}

func TestGeminiTransport_buildRequest_GenerationConfig(t *testing.T) {
	tr := NewGeminiTransport("gemini-2.0-flash", "test-key")
	temp := float32(0.5)
	req := llm.ChatRequest{
		Messages:    []llm.Message{{Role: "user", Content: "test"}},
		MaxTokens:   2048,
		Temperature: &temp,
	}

	gemReq := tr.buildRequest(req)
	if gemReq.GenerationConfig == nil {
		t.Fatal("GenerationConfig is nil")
	}
	if *gemReq.GenerationConfig.MaxOutputTokens != 2048 {
		t.Errorf("MaxOutputTokens = %d", *gemReq.GenerationConfig.MaxOutputTokens)
	}
	if *gemReq.GenerationConfig.Temperature != 0.5 {
		t.Errorf("Temperature = %f", *gemReq.GenerationConfig.Temperature)
	}
}

func TestGeminiTransport_parseResponse_Empty(t *testing.T) {
	tr := NewGeminiTransport("gemini-2.0-flash", "test-key")
	resp := &geminiResponse{}

	result, err := tr.parseResponse(resp)
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if result.Content != "" {
		t.Errorf("Content = %q, want empty", result.Content)
	}
	if result.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want stop", result.FinishReason)
	}
}

func TestGeminiTransport_parseResponse_WithContent(t *testing.T) {
	tr := NewGeminiTransport("gemini-2.0-flash", "test-key")
	resp := &geminiResponse{
		Candidates: []geminiCandidate{
			{
				Content: geminiContent{
					Parts: []geminiPart{
						{Text: "Hello "},
						{Text: "world!"},
					},
				},
				FinishReason: "STOP",
			},
		},
		UsageMetadata: &geminiUsage{
			PromptTokenCount:     10,
			CandidatesTokenCount: 5,
			TotalTokenCount:      15,
		},
	}

	result, err := tr.parseResponse(resp)
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if result.Content != "Hello world!" {
		t.Errorf("Content = %q", result.Content)
	}
	if result.FinishReason != "stop" {
		t.Errorf("FinishReason = %q", result.FinishReason)
	}
	if result.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d", result.Usage.PromptTokens)
	}
	if result.Usage.CompletionTokens != 5 {
		t.Errorf("CompletionTokens = %d", result.Usage.CompletionTokens)
	}
}

func TestGeminiTransport_parseResponse_WithToolCall(t *testing.T) {
	tr := NewGeminiTransport("gemini-2.0-flash", "test-key")
	resp := &geminiResponse{
		Candidates: []geminiCandidate{
			{
				Content: geminiContent{
					Parts: []geminiPart{
						{
							FunctionCall: &geminiFunctionCall{
								Name: "get_weather",
								Args: map[string]any{"city": "Beijing"},
							},
						},
					},
				},
				FinishReason: "STOP",
			},
		},
	}

	result, err := tr.parseResponse(resp)
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if result.FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q, want tool_calls", result.FinishReason)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("ToolCalls len = %d, want 1", len(result.ToolCalls))
	}
	tc := result.ToolCalls[0]
	if tc.Function.Name != "get_weather" {
		t.Errorf("Name = %q", tc.Function.Name)
	}
	// Verify arguments contain the city.
	var args map[string]any
	json.Unmarshal([]byte(tc.Function.Arguments), &args)
	if args["city"] != "Beijing" {
		t.Errorf("args[city] = %v", args["city"])
	}
}

func TestGeminiTransport_parseResponse_MaxTokensReason(t *testing.T) {
	tr := NewGeminiTransport("gemini-2.0-flash", "test-key")
	resp := &geminiResponse{
		Candidates: []geminiCandidate{
			{
				Content:      geminiContent{Parts: []geminiPart{{Text: "truncated"}}},
				FinishReason: "MAX_TOKENS",
			},
		},
	}

	result, err := tr.parseResponse(resp)
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if result.FinishReason != "length" {
		t.Errorf("FinishReason = %q, want length", result.FinishReason)
	}
}
