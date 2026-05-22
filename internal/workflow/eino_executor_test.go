package workflow

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/store"
)

type workflowEinoTransport struct {
	mu       sync.Mutex
	requests []llm.ChatRequest
}

func (t *workflowEinoTransport) Name() string { return "workflow-eino-test" }

func (t *workflowEinoTransport) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	t.mu.Lock()
	t.requests = append(t.requests, req)
	t.mu.Unlock()
	return &llm.ChatResponse{
		Content:      "workflow answer",
		FinishReason: "stop",
		Usage:        llm.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5},
	}, nil
}

func (t *workflowEinoTransport) ChatStream(context.Context, llm.ChatRequest) (<-chan llm.StreamDelta, <-chan error) {
	panic("ChatStream not used by workflow executor test")
}

func (t *workflowEinoTransport) LastRequest() llm.ChatRequest {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.requests) == 0 {
		return llm.ChatRequest{}
	}
	return t.requests[len(t.requests)-1]
}

func TestEinoAgentExecutor_UsesTurnLoopMainPath(t *testing.T) {
	transport := &workflowEinoTransport{}
	executor := NewEinoAgentExecutor(transport, nil, nil, nil)

	out, err := executor.Execute(context.Background(), "tenant-1", "user-1", store.WorkflowNode{
		ID:   "agent-review",
		Type: store.WorkflowNodeAgentTask,
		Config: map[string]any{
			"prompt":         "Review the workflow input.",
			"model":          "test-model",
			"max_iterations": float64(99),
		},
	}, map[string]any{"run_id": "run-123", "input": map[string]any{"ticket": "T-1"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["response"] != "workflow answer" {
		t.Fatalf("response = %#v, want workflow answer", out["response"])
	}

	req := transport.LastRequest()
	if req.Stream {
		t.Fatal("workflow executor should use non-streaming TurnLoop path")
	}
	if len(req.Messages) == 0 {
		t.Fatal("expected request messages")
	}
	gotPrompt := req.Messages[len(req.Messages)-1].Content
	if !strings.Contains(gotPrompt, "Review the workflow input.") {
		t.Fatalf("prompt missing node prompt: %q", gotPrompt)
	}
	if !strings.Contains(gotPrompt, `"run_id":"run-123"`) {
		t.Fatalf("prompt missing workflow payload: %q", gotPrompt)
	}
}

func TestEinoAgentExecutor_RequiresPromptAndTransport(t *testing.T) {
	executor := NewEinoAgentExecutor(&workflowEinoTransport{}, nil, nil, nil)
	if _, err := executor.Execute(context.Background(), "tenant-1", "user-1", store.WorkflowNode{}, nil); err == nil {
		t.Fatal("expected missing prompt error")
	}

	executor = NewEinoAgentExecutor(nil, nil, nil, nil)
	_, err := executor.Execute(context.Background(), "tenant-1", "user-1", store.WorkflowNode{
		ID:     "agent",
		Type:   store.WorkflowNodeAgentTask,
		Config: map[string]any{"prompt": "hello"},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "transport is required") {
		t.Fatalf("expected transport error, got %v", err)
	}
}
