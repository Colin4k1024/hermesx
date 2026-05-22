package agentruntime

import (
	"context"
	"errors"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/agent"
	"github.com/Colin4k1024/hermesx/internal/llm"
)

type fakeTransport struct {
	requests  []llm.ChatRequest
	responses []llm.ChatResponse
	err       error
}

func (f *fakeTransport) Name() string { return "fake" }

func (f *fakeTransport) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}
	idx := len(f.requests) - 1
	if idx >= len(f.responses) {
		idx = len(f.responses) - 1
	}
	return &f.responses[idx], nil
}

func (f *fakeTransport) ChatStream(_ context.Context, req llm.ChatRequest) (<-chan llm.StreamDelta, <-chan error) {
	f.requests = append(f.requests, req)
	deltaCh := make(chan llm.StreamDelta, 1)
	errCh := make(chan error, 1)
	go func() {
		defer close(deltaCh)
		if f.err != nil {
			errCh <- f.err
			close(errCh)
			return
		}
		deltaCh <- llm.StreamDelta{Content: "streamed"}
		deltaCh <- llm.StreamDelta{Done: true}
		close(errCh)
	}()
	return deltaCh, errCh
}

func TestEinoRuntimeChatUsesEino(t *testing.T) {
	transport := &fakeTransport{responses: []llm.ChatResponse{{
		Content:      "ok",
		FinishReason: "stop",
		Usage:        llm.Usage{PromptTokens: 3, CompletionTokens: 2},
	}}}
	runtime, err := NewEino(context.Background(), Options{
		Transport:        transport,
		Model:            "test-model",
		Platform:         "cron",
		SessionID:        "cron-session",
		SkipMemory:       true,
		DisabledToolsets: []string{"cronjob", "messaging", "clarify"},
	})
	if err != nil {
		t.Fatalf("NewEino: %v", err)
	}

	result, err := runtime.Chat(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected ok, got %q", result)
	}
	if runtime.SessionID() != "cron-session" {
		t.Fatalf("unexpected session id: %q", runtime.SessionID())
	}
	if runtime.Model() != "test-model" {
		t.Fatalf("unexpected model: %q", runtime.Model())
	}
	if len(transport.requests) != 1 {
		t.Fatalf("expected one LLM call, got %d", len(transport.requests))
	}
	if got := transport.requests[0].Messages[0].Role; got != "system" {
		t.Fatalf("expected system prompt first, got %q", got)
	}
}

func TestResolveToolsDisablesToolsets(t *testing.T) {
	tools := resolveTools(nil, []string{"cronjob", "messaging", "clarify"})
	for _, name := range []string{"cronjob", "send_message", "clarify"} {
		if tools[name] {
			t.Fatalf("expected %s to be disabled", name)
		}
	}
	if !tools["terminal"] {
		t.Fatal("expected default hermesx-cli tools to remain enabled")
	}
}

func TestAdaptCallbacksMapsCommonSurface(t *testing.T) {
	var events []string
	cb := AdaptCallbacks(&agent.StreamCallbacks{
		OnStreamDelta:  func(string) { events = append(events, "delta") },
		OnReasoning:    func(string) { events = append(events, "reasoning") },
		OnToolStart:    func(string) { events = append(events, "start") },
		OnToolComplete: func(string) { events = append(events, "complete") },
		OnStep:         func(int, []string) { events = append(events, "step") },
		OnStatus:       func(string) { events = append(events, "status") },
		OnError:        func(error) { events = append(events, "error") },
	})
	cb.OnStreamDelta("")
	cb.OnReasoning("")
	cb.OnToolStart("")
	cb.OnToolComplete("")
	cb.OnStep(0, nil)
	cb.OnStatus("")
	cb.OnError(errors.New("boom"))
	if len(events) != 7 {
		t.Fatalf("expected all common callbacks to map, got %v", events)
	}
}
