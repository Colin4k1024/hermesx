package poc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

// mockTransport simulates an LLM that performs a tool call then returns final answer.
type mockTransport struct {
	callCount int
	responses []llm.ChatResponse
}

func (m *mockTransport) Name() string { return "mock" }

func (m *mockTransport) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	idx := m.callCount
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	m.callCount++
	return &m.responses[idx], nil
}

func (m *mockTransport) ChatStream(_ context.Context, _ llm.ChatRequest) (<-chan llm.StreamDelta, <-chan error) {
	panic("not used in this test")
}

func TestReactAgentToolLoop(t *testing.T) {
	toolCallArgs, _ := json.Marshal(map[string]any{"command": "echo hello"})

	transport := &mockTransport{
		responses: []llm.ChatResponse{
			{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "terminal",
							Arguments: string(toolCallArgs),
						},
					},
				},
				FinishReason: "tool_calls",
			},
			{
				Content:      "The command output was: hello",
				FinishReason: "stop",
			},
		},
	}

	toolEntries := []*tools.ToolEntry{
		{
			Name:        "terminal",
			Description: "Execute a shell command",
			Schema: map[string]any{
				"name":        "terminal",
				"description": "Execute a shell command",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "string",
							"description": "The shell command to execute",
						},
					},
					"required": []string{"command"},
				},
			},
			Handler: func(_ context.Context, args map[string]any, _ *tools.ToolContext) string {
				cmd, _ := args["command"].(string)
				return "stdout: hello\nexit_code: 0\n(executed: " + cmd + ")"
			},
		},
	}

	ctx := context.Background()
	agent, err := NewReactAgent(ctx, ReactAgentConfig{
		Transport: transport,
		ModelName: "test-model",
		ToolSet:   toolEntries,
		MaxStep:   10,
		SystemMsg: "You are a helpful assistant.",
	})
	if err != nil {
		t.Fatalf("NewReactAgent: %v", err)
	}

	tctx := &tools.ToolContext{
		TaskID:    "test-task",
		SessionID: "test-session",
	}

	result, err := agent.Generate(ctx, "run echo hello", tctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Content == "" {
		t.Error("expected non-empty content in final response")
	}

	if transport.callCount != 2 {
		t.Errorf("expected 2 LLM calls (tool call + final), got %d", transport.callCount)
	}

	t.Logf("Final response: %s", result.Content)
}

func TestReactAgentNoToolCall(t *testing.T) {
	transport := &mockTransport{
		responses: []llm.ChatResponse{
			{
				Content:      "Hello! How can I help you?",
				FinishReason: "stop",
			},
		},
	}

	toolEntries := []*tools.ToolEntry{
		{
			Name:        "terminal",
			Description: "Execute a shell command",
			Schema: map[string]any{
				"name":        "terminal",
				"description": "Execute a shell command",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "string",
							"description": "The shell command to execute",
						},
					},
					"required": []string{"command"},
				},
			},
			Handler: func(_ context.Context, _ map[string]any, _ *tools.ToolContext) string {
				return "should not be called"
			},
		},
	}

	ctx := context.Background()
	agent, err := NewReactAgent(ctx, ReactAgentConfig{
		Transport: transport,
		ModelName: "test-model",
		ToolSet:   toolEntries,
	})
	if err != nil {
		t.Fatalf("NewReactAgent: %v", err)
	}

	result, err := agent.Generate(ctx, "hello", &tools.ToolContext{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if result.Content != "Hello! How can I help you?" {
		t.Errorf("unexpected content: %s", result.Content)
	}

	if transport.callCount != 1 {
		t.Errorf("expected 1 LLM call, got %d", transport.callCount)
	}
}
