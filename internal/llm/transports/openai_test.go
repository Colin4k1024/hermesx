package transports

import (
	"encoding/json"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

func TestOpenAITransport_Name(t *testing.T) {
	tr := NewOpenAITransport("gpt-4", "https://api.openai.com/v1", "test-key")
	if tr.Name() != "openai" {
		t.Errorf("Name() = %q, want %q", tr.Name(), "openai")
	}
}

func TestOpenAITransport_buildRequest_BasicMessage(t *testing.T) {
	tr := NewOpenAITransport("gpt-4", "https://api.openai.com/v1", "test-key")
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello"},
		},
	}

	apiReq := tr.buildRequest(req)

	if apiReq.Model != "gpt-4" {
		t.Errorf("Model = %q, want %q", apiReq.Model, "gpt-4")
	}
	if len(apiReq.Messages) != 2 {
		t.Fatalf("Messages len = %d, want 2", len(apiReq.Messages))
	}
	if apiReq.Messages[0].Role != "system" || apiReq.Messages[0].Content != "You are a helpful assistant." {
		t.Errorf("Messages[0] = %+v", apiReq.Messages[0])
	}
	if apiReq.Messages[1].Role != "user" || apiReq.Messages[1].Content != "Hello" {
		t.Errorf("Messages[1] = %+v", apiReq.Messages[1])
	}
}

func TestOpenAITransport_buildRequest_WithToolCalls(t *testing.T) {
	tr := NewOpenAITransport("gpt-4", "https://api.openai.com/v1", "test-key")
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_123",
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
				ToolCallID: "call_123",
			},
		},
	}

	apiReq := tr.buildRequest(req)
	if len(apiReq.Messages) != 2 {
		t.Fatalf("Messages len = %d, want 2", len(apiReq.Messages))
	}

	// Assistant message with tool calls.
	if len(apiReq.Messages[0].ToolCalls) != 1 {
		t.Fatalf("ToolCalls len = %d, want 1", len(apiReq.Messages[0].ToolCalls))
	}
	if apiReq.Messages[0].ToolCalls[0].ID != "call_123" {
		t.Errorf("ToolCall ID = %q", apiReq.Messages[0].ToolCalls[0].ID)
	}
	if apiReq.Messages[0].ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("ToolCall Function Name = %q", apiReq.Messages[0].ToolCalls[0].Function.Name)
	}

	// Tool response message.
	if apiReq.Messages[1].Role != "tool" {
		t.Errorf("Role = %q, want %q", apiReq.Messages[1].Role, "tool")
	}
	if apiReq.Messages[1].ToolCallID != "call_123" {
		t.Errorf("ToolCallID = %q", apiReq.Messages[1].ToolCallID)
	}
}

func TestOpenAITransport_buildRequest_WithImages(t *testing.T) {
	tr := NewOpenAITransport("gpt-4o", "https://api.openai.com/v1", "test-key")
	req := llm.ChatRequest{
		Messages: []llm.Message{
			{
				Role:     "user",
				Content:  "What's in this image?",
				ImageURLs: []string{"https://example.com/cat.jpg"},
			},
		},
	}

	apiReq := tr.buildRequest(req)
	if len(apiReq.Messages[0].MultiContent) != 2 {
		t.Fatalf("MultiContent len = %d, want 2", len(apiReq.Messages[0].MultiContent))
	}
	if apiReq.Messages[0].MultiContent[0].Type != "text" {
		t.Errorf("MultiContent[0].Type = %q", apiReq.Messages[0].MultiContent[0].Type)
	}
	if apiReq.Messages[0].MultiContent[1].Type != "image_url" {
		t.Errorf("MultiContent[1].Type = %q", apiReq.Messages[0].MultiContent[1].Type)
	}
}

func TestOpenAITransport_buildRequest_WithTools(t *testing.T) {
	tr := NewOpenAITransport("gpt-4", "https://api.openai.com/v1", "test-key")
	req := llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "test"}},
		Tools: []llm.ToolDef{
			{
				Name:        "get_weather",
				Description: "Get weather for a city",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	apiReq := tr.buildRequest(req)
	if len(apiReq.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(apiReq.Tools))
	}
	if apiReq.Tools[0].Function.Name != "get_weather" {
		t.Errorf("Tool Name = %q", apiReq.Tools[0].Function.Name)
	}
	if apiReq.Tools[0].Function.Description != "Get weather for a city" {
		t.Errorf("Tool Description = %q", apiReq.Tools[0].Function.Description)
	}
}

func TestOpenAITransport_buildRequest_MaxTokensAndTemperature(t *testing.T) {
	tr := NewOpenAITransport("gpt-4", "https://api.openai.com/v1", "test-key")
	temp := float32(0.7)
	req := llm.ChatRequest{
		Messages:    []llm.Message{{Role: "user", Content: "test"}},
		MaxTokens:   1024,
		Temperature: &temp,
	}

	apiReq := tr.buildRequest(req)
	if apiReq.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %d, want 1024", apiReq.MaxTokens)
	}
	if apiReq.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", apiReq.Temperature)
	}
}

func TestConvertToolDefsToJSON(t *testing.T) {
	tools := []llm.ToolDef{
		{
			Name:        "search",
			Description: "Search the web",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
			},
		},
	}

	result := ConvertToolDefsToJSON(tools)
	if len(result) != 1 {
		t.Fatalf("result len = %d, want 1", len(result))
	}
	if result[0]["type"] != "function" {
		t.Errorf("type = %q", result[0]["type"])
	}

	fn, ok := result[0]["function"].(map[string]any)
	if !ok {
		t.Fatal("function is not a map")
	}
	if fn["name"] != "search" {
		t.Errorf("name = %q", fn["name"])
	}

	// Verify round-trip through JSON.
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var roundTrip []map[string]any
	if err := json.Unmarshal(b, &roundTrip); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(roundTrip) != 1 {
		t.Errorf("round-trip len = %d", len(roundTrip))
	}
}
