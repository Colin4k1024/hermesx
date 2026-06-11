package eino

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/Colin4k1024/hermesx/internal/eino/modeladapter"
	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/secrets"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

type mockTransport struct {
	callCount int
	responses []llm.ChatResponse
}

type stubAgenticModel struct {
	message *schema.AgenticMessage
	err     error
}

type einoTestMemoryProvider struct{}

func (einoTestMemoryProvider) ReadMemory() (string, error)      { return "", nil }
func (einoTestMemoryProvider) SaveMemory(string, string) error  { return nil }
func (einoTestMemoryProvider) DeleteMemory(string) error        { return nil }
func (einoTestMemoryProvider) ReadUserProfile() (string, error) { return "", nil }
func (einoTestMemoryProvider) SaveUserProfile(string) error     { return nil }

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
	deltaCh := make(chan llm.StreamDelta, 1)
	errCh := make(chan error, 1)
	idx := m.callCount
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	m.callCount++
	resp := m.responses[idx]
	go func() {
		defer close(deltaCh)
		if resp.Content != "" {
			deltaCh <- llm.StreamDelta{Content: resp.Content}
		}
		deltaCh <- llm.StreamDelta{Done: true}
	}()
	return deltaCh, errCh
}

func (s *stubAgenticModel) Generate(_ context.Context, _ []*schema.AgenticMessage, _ ...model.Option) (*schema.AgenticMessage, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.message, nil
}

func (s *stubAgenticModel) Stream(_ context.Context, _ []*schema.AgenticMessage, _ ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	reader, writer := schema.Pipe[*schema.AgenticMessage](1)
	go func() {
		defer writer.Close()
		if s.err != nil {
			writer.Send(nil, s.err)
			return
		}
		if s.message != nil {
			writer.Send(s.message, nil)
		}
	}()
	return reader, nil
}

func TestBuildChatModel_PrefersAgenticProviderModel(t *testing.T) {
	oldFactory := agenticProviderModelFactory
	defer func() { agenticProviderModelFactory = oldFactory }()

	agenticProviderModelFactory = func(context.Context, AgenticProviderConfig) (model.AgenticModel, error) {
		return &stubAgenticModel{}, nil
	}

	got := buildChatModelWithConfig(context.Background(), &mockTransport{}, "gpt-5", "openai", "https://example.com", "test-key", "responses", NewCapture())
	if _, ok := got.(*modeladapter.AgenticBridge); !ok {
		t.Fatalf("expected native agentic bridge, got %T", got)
	}
}

func TestBuildChatModel_OpenAIChatCompletionsUsesTransport(t *testing.T) {
	oldFactory := agenticProviderModelFactory
	defer func() { agenticProviderModelFactory = oldFactory }()

	agenticProviderModelFactory = func(context.Context, AgenticProviderConfig) (model.AgenticModel, error) {
		return &stubAgenticModel{}, nil
	}

	got := buildChatModelWithConfig(context.Background(), &mockTransport{}, "mimo-v2.5-pro", "openai", "https://example.com/v1", "test-key", "openai", NewCapture())
	if _, ok := got.(*modeladapter.WrappedModel); !ok {
		t.Fatalf("expected OpenAI chat completions to use legacy transport wrapper, got %T", got)
	}
}

func TestBuildChatModel_FallsBackToTransportWhenAgenticUnavailable(t *testing.T) {
	oldFactory := agenticProviderModelFactory
	defer func() { agenticProviderModelFactory = oldFactory }()

	agenticProviderModelFactory = func(context.Context, AgenticProviderConfig) (model.AgenticModel, error) {
		return nil, errors.New("boom")
	}

	got := buildChatModelWithConfig(context.Background(), &mockTransport{}, "gpt-5", "openai", "https://example.com", "test-key", "responses", NewCapture())
	if _, ok := got.(*modeladapter.WrappedModel); !ok {
		t.Fatalf("expected legacy transport wrapper, got %T", got)
	}
}

func TestAgenticBridge_CapturesNativeOnlyBlocks(t *testing.T) {
	capture := NewCapture()
	bridge := modeladapter.WrapAgentic(&stubAgenticModel{
		message: &schema.AgenticMessage{
			Role: schema.AgenticRoleTypeAssistant,
			ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.AssistantGenText{Text: "hello"}),
				schema.NewContentBlock(&schema.ServerToolCall{Name: "web_search", CallID: "srv_1", Arguments: map[string]any{"query": "weather"}}),
			},
		},
	}, capture)

	out, err := bridge.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out.Content != "hello" {
		t.Fatalf("expected assistant text to remain in converted message, got %q", out.Content)
	}
	blocks := capture.Blocks()
	if len(blocks) != 1 {
		t.Fatalf("expected exactly one native-only block, got %d", len(blocks))
	}
	if blocks[0].Type != string(schema.ContentBlockTypeServerToolCall) {
		t.Fatalf("expected server tool call block, got %q", blocks[0].Type)
	}
	serverToolCall, _ := blocks[0].Data["server_tool_call"].(map[string]any)
	if name, _ := serverToolCall["name"].(string); name != "web_search" {
		t.Fatalf("expected native block payload to be preserved, got %#v", blocks[0].Data)
	}
}

func TestEinoAgent_SingleToolLoop(t *testing.T) {
	toolArgs, _ := json.Marshal(map[string]any{"command": "echo hello"})
	transport := &mockTransport{
		responses: []llm.ChatResponse{
			{
				ToolCalls: []llm.ToolCall{
					{ID: "c1", Type: "function", Function: llm.FunctionCall{Name: "terminal", Arguments: string(toolArgs)}},
				},
				FinishReason: "tool_calls",
			},
			{Content: "Done: hello", FinishReason: "stop"},
		},
	}

	entries := []*tools.ToolEntry{
		{
			Name:        "terminal",
			Description: "Execute command",
			Schema: map[string]any{
				"name": "terminal", "description": "Execute command",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{"type": "string"},
					},
					"required": []string{"command"},
				},
			},
			Handler: func(_ context.Context, args map[string]any, _ *tools.ToolContext) string {
				cmd, _ := args["command"].(string)
				return "output: " + cmd
			},
		},
	}

	ctx := context.Background()
	agent, err := NewEinoAgent(ctx,
		WithTransport(transport),
		WithModel("test-model"),
		WithTools(entries),
		WithMaxIterations(10),
		WithSystemPrompt("You are helpful."),
		WithSessionID("test-session"),
	)
	if err != nil {
		t.Fatalf("NewEinoAgent: %v", err)
	}

	result, err := agent.RunConversation(ctx, "run echo hello", nil)
	if err != nil {
		t.Fatalf("RunConversation: %v", err)
	}

	if result.FinalResponse != "Done: hello" {
		t.Errorf("unexpected response: %q", result.FinalResponse)
	}
	if !result.Completed {
		t.Error("expected Completed=true")
	}
	if transport.callCount != 2 {
		t.Errorf("expected 2 LLM calls, got %d", transport.callCount)
	}
}

func TestEinoAgent_MultiToolChain(t *testing.T) {
	args1, _ := json.Marshal(map[string]any{"query": "weather"})
	args2, _ := json.Marshal(map[string]any{"command": "date"})
	args3, _ := json.Marshal(map[string]any{"query": "news"})

	transport := &mockTransport{
		responses: []llm.ChatResponse{
			{
				ToolCalls:    []llm.ToolCall{{ID: "c1", Type: "function", Function: llm.FunctionCall{Name: "search", Arguments: string(args1)}}},
				FinishReason: "tool_calls",
			},
			{
				ToolCalls:    []llm.ToolCall{{ID: "c2", Type: "function", Function: llm.FunctionCall{Name: "terminal", Arguments: string(args2)}}},
				FinishReason: "tool_calls",
			},
			{
				ToolCalls:    []llm.ToolCall{{ID: "c3", Type: "function", Function: llm.FunctionCall{Name: "search", Arguments: string(args3)}}},
				FinishReason: "tool_calls",
			},
			{Content: "Weather is sunny, today is Monday, news: stock up", FinishReason: "stop"},
		},
	}

	entries := []*tools.ToolEntry{
		{
			Name: "search", Description: "Search",
			Schema: map[string]any{
				"name": "search", "description": "Search",
				"parameters": map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}},
			},
			Handler: func(_ context.Context, args map[string]any, _ *tools.ToolContext) string {
				q, _ := args["query"].(string)
				return "result for: " + q
			},
		},
		{
			Name: "terminal", Description: "Run command",
			Schema: map[string]any{
				"name": "terminal", "description": "Run command",
				"parameters": map[string]any{"type": "object", "properties": map[string]any{"command": map[string]any{"type": "string"}}},
			},
			Handler: func(_ context.Context, args map[string]any, _ *tools.ToolContext) string {
				cmd, _ := args["command"].(string)
				return "exec: " + cmd
			},
		},
	}

	ctx := context.Background()
	agent, err := NewEinoAgent(ctx, WithTransport(transport), WithModel("m"), WithTools(entries), WithMaxIterations(10))
	if err != nil {
		t.Fatalf("NewEinoAgent: %v", err)
	}

	result, err := agent.RunConversation(ctx, "weather, date, news", nil)
	if err != nil {
		t.Fatalf("RunConversation: %v", err)
	}

	if transport.callCount != 4 {
		t.Errorf("expected 4 LLM calls (3 tool + 1 final), got %d", transport.callCount)
	}
	if result.FinalResponse == "" {
		t.Error("expected non-empty response")
	}
}

func TestEinoAgent_NoToolCall(t *testing.T) {
	transport := &mockTransport{
		responses: []llm.ChatResponse{
			{Content: "Hi there!", FinishReason: "stop"},
		},
	}

	entries := []*tools.ToolEntry{
		{
			Name: "search", Description: "Search",
			Schema: map[string]any{
				"name": "search", "description": "Search",
				"parameters": map[string]any{"type": "object", "properties": map[string]any{"q": map[string]any{"type": "string"}}},
			},
			Handler: func(_ context.Context, _ map[string]any, _ *tools.ToolContext) string { return "" },
		},
	}

	ctx := context.Background()
	agent, err := NewEinoAgent(ctx, WithTransport(transport), WithModel("m"), WithTools(entries))
	if err != nil {
		t.Fatalf("NewEinoAgent: %v", err)
	}

	result, err := agent.RunConversation(ctx, "hello", nil)
	if err != nil {
		t.Fatalf("RunConversation: %v", err)
	}

	if result.FinalResponse != "Hi there!" {
		t.Errorf("unexpected: %q", result.FinalResponse)
	}
	if transport.callCount != 1 {
		t.Errorf("expected 1 call, got %d", transport.callCount)
	}
}

func TestEinoAgent_WithHistory(t *testing.T) {
	transport := &mockTransport{
		responses: []llm.ChatResponse{
			{Content: "Your name is Alice.", FinishReason: "stop"},
		},
	}

	ctx := context.Background()
	agent, err := NewEinoAgent(ctx, WithTransport(transport), WithModel("m"), WithTools(nil))
	if err != nil {
		t.Fatalf("NewEinoAgent: %v", err)
	}

	history := []llm.Message{
		{Role: "user", Content: "My name is Alice"},
		{Role: "assistant", Content: "Nice to meet you, Alice!"},
	}

	result, err := agent.RunConversation(ctx, "What is my name?", history)
	if err != nil {
		t.Fatalf("RunConversation: %v", err)
	}

	if result.FinalResponse != "Your name is Alice." {
		t.Errorf("unexpected: %q", result.FinalResponse)
	}
}

func TestEinoAgent_ContextPropagation(t *testing.T) {
	toolArgs, _ := json.Marshal(map[string]any{})
	transport := &mockTransport{
		responses: []llm.ChatResponse{
			{
				ToolCalls:    []llm.ToolCall{{ID: "c1", Type: "function", Function: llm.FunctionCall{Name: "check_ctx", Arguments: string(toolArgs)}}},
				FinishReason: "tool_calls",
			},
			{Content: "ok", FinishReason: "stop"},
		},
	}

	var capturedTenantID, capturedSessionID string
	entries := []*tools.ToolEntry{
		{
			Name: "check_ctx", Description: "Check context",
			Schema: map[string]any{
				"name": "check_ctx", "description": "Check context",
				"parameters": map[string]any{"type": "object", "properties": map[string]any{}},
			},
			Handler: func(_ context.Context, _ map[string]any, tctx *tools.ToolContext) string {
				capturedTenantID = tctx.TenantID
				capturedSessionID = tctx.SessionID
				return "captured"
			},
		},
	}

	ctx := context.Background()
	agent, err := NewEinoAgent(ctx,
		WithTransport(transport),
		WithModel("m"),
		WithTools(entries),
		WithTenantID("tenant-123"),
		WithSessionID("session-456"),
	)
	if err != nil {
		t.Fatalf("NewEinoAgent: %v", err)
	}

	_, err = agent.RunConversation(ctx, "check", nil)
	if err != nil {
		t.Fatalf("RunConversation: %v", err)
	}

	if capturedTenantID != "tenant-123" {
		t.Errorf("expected tenant-123, got %q", capturedTenantID)
	}
	if capturedSessionID != "session-456" {
		t.Errorf("expected session-456, got %q", capturedSessionID)
	}
}

func TestEinoAgent_ToolContextParity(t *testing.T) {
	toolArgs, _ := json.Marshal(map[string]any{})
	transport := &mockTransport{
		responses: []llm.ChatResponse{
			{
				ToolCalls:    []llm.ToolCall{{ID: "c1", Type: "function", Function: llm.FunctionCall{Name: "check_parity", Arguments: string(toolArgs)}}},
				FinishReason: "tool_calls",
			},
			{Content: "ok", FinishReason: "stop"},
		},
	}
	resolver := secrets.NewEnvSecretResolver(secrets.NewEnvSecretStore("HERMES_TEST_"))

	var captured *tools.ToolContext
	entries := []*tools.ToolEntry{
		{
			Name:         "check_parity",
			Description:  "Check context parity",
			MaxRedirects: 1,
			Schema: map[string]any{
				"name": "check_parity", "description": "Check context parity",
				"parameters": map[string]any{"type": "object", "properties": map[string]any{}},
			},
			Handler: func(_ context.Context, _ map[string]any, tctx *tools.ToolContext) string {
				copied := *tctx
				captured = &copied
				return "captured"
			},
		},
	}

	agent, err := NewEinoAgent(context.Background(),
		WithTransport(transport),
		WithModel("m"),
		WithTools(entries),
		WithTenantID("tenant-123"),
		WithUserID("user-456"),
		WithSessionID("session-789"),
		WithPlatform("gateway"),
		WithMemoryProvider(einoTestMemoryProvider{}),
		WithSecretResolver(resolver),
	)
	if err != nil {
		t.Fatalf("NewEinoAgent: %v", err)
	}

	if _, err := agent.RunConversation(context.Background(), "check", nil); err != nil {
		t.Fatalf("RunConversation: %v", err)
	}
	if captured == nil {
		t.Fatal("tool context was not captured")
	}
	if captured.TenantID != "tenant-123" || captured.UserID != "user-456" || captured.SessionID != "session-789" || captured.Platform != "gateway" {
		t.Fatalf("unexpected context identity: %#v", captured)
	}
	if captured.MemoryProvider == nil {
		t.Fatal("expected memory provider")
	}
	if captured.SecretResolver != resolver {
		t.Fatal("expected secret resolver to be propagated")
	}
	if captured.HTTPClient == nil {
		t.Fatal("expected egress-aware HTTP client")
	}
	if captured.HTTPClient.CheckRedirect == nil {
		t.Fatal("expected per-tool redirect policy")
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	if err := captured.HTTPClient.CheckRedirect(req, []*http.Request{req}); err == nil {
		t.Fatal("expected redirect limit error")
	}
}

func TestEinoAgent_StreamCallbacks(t *testing.T) {
	transport := &mockTransport{
		responses: []llm.ChatResponse{
			{Content: "Hello world", FinishReason: "stop"},
		},
	}

	ctx := context.Background()
	agent, err := NewEinoAgent(ctx, WithTransport(transport), WithModel("m"), WithTools(nil))
	if err != nil {
		t.Fatalf("NewEinoAgent: %v", err)
	}

	var deltas []string
	agent.SetCallbacks(&StreamCallbacks{
		OnStreamDelta: func(text string) {
			deltas = append(deltas, text)
		},
	})

	result, err := agent.Stream(ctx, "hi", nil)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	if result.FinalResponse != "Hello world" {
		t.Errorf("unexpected response: %q", result.FinalResponse)
	}
	if len(deltas) == 0 {
		t.Error("expected at least one delta callback")
	}
}

func TestEinoAgent_RunConversationWithCallbacks(t *testing.T) {
	toolArgs, _ := json.Marshal(map[string]any{"command": "ls"})
	transport := &mockTransport{
		responses: []llm.ChatResponse{
			{
				ToolCalls:    []llm.ToolCall{{ID: "c1", Type: "function", Function: llm.FunctionCall{Name: "terminal", Arguments: string(toolArgs)}}},
				FinishReason: "tool_calls",
			},
			{Content: "files listed", FinishReason: "stop"},
		},
	}

	entries := []*tools.ToolEntry{
		{
			Name: "terminal", Description: "Run command",
			Schema: map[string]any{
				"name": "terminal", "description": "Run command",
				"parameters": map[string]any{"type": "object", "properties": map[string]any{"command": map[string]any{"type": "string"}}},
			},
			Handler: func(_ context.Context, args map[string]any, _ *tools.ToolContext) string {
				return "output: " + args["command"].(string)
			},
		},
	}

	ctx := context.Background()
	agent, err := NewEinoAgent(ctx, WithTransport(transport), WithModel("m"), WithTools(entries), WithTenantID("t1"), WithSessionID("s1"))
	if err != nil {
		t.Fatalf("NewEinoAgent: %v", err)
	}

	var toolStarted, toolCompleted []string
	var stepCalled bool
	var errors []error

	agent.SetCallbacks(&StreamCallbacks{
		OnToolStart: func(name string) {
			toolStarted = append(toolStarted, name)
		},
		OnToolComplete: func(name string) {
			toolCompleted = append(toolCompleted, name)
		},
		OnStep: func(_ int, _ []string) {
			stepCalled = true
		},
		OnError: func(err error) {
			errors = append(errors, err)
		},
	})

	result, err := agent.RunConversationWithCallbacks(ctx, "list files", nil)
	if err != nil {
		t.Fatalf("RunConversationWithCallbacks: %v", err)
	}

	if result.FinalResponse != "files listed" {
		t.Errorf("unexpected response: %q", result.FinalResponse)
	}
	if !result.Completed {
		t.Error("expected Completed=true")
	}
	if !stepCalled {
		t.Error("expected OnStep callback to fire")
	}
	if len(toolStarted) == 0 {
		t.Error("expected OnToolStart callback to fire")
	}
	if len(toolCompleted) == 0 {
		t.Error("expected OnToolComplete callback to fire")
	}
	if len(errors) != 0 {
		t.Errorf("unexpected errors: %v", errors)
	}
}

func TestEinoAgent_RunConversationWithCallbacks_NilCallbacks(t *testing.T) {
	transport := &mockTransport{
		responses: []llm.ChatResponse{
			{Content: "direct response", FinishReason: "stop"},
		},
	}

	ctx := context.Background()
	agent, err := NewEinoAgent(ctx, WithTransport(transport), WithModel("m"), WithTools(nil))
	if err != nil {
		t.Fatalf("NewEinoAgent: %v", err)
	}

	result, err := agent.RunConversationWithCallbacks(ctx, "hello", nil)
	if err != nil {
		t.Fatalf("RunConversationWithCallbacks: %v", err)
	}

	if result.FinalResponse != "direct response" {
		t.Errorf("unexpected: %q", result.FinalResponse)
	}
}

func TestEinoAgent_MissingTransport(t *testing.T) {
	_, err := NewEinoAgent(context.Background(), WithModel("m"))
	if err == nil {
		t.Fatal("expected error for missing transport")
	}
}
