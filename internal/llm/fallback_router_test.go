package llm

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/sony/gobreaker/v2"
)

// mockTransport is a test double for Transport.
type mockTransport struct {
	name       string
	chatResp   *ChatResponse
	chatErr    error
	streamData []StreamDelta
	streamErr  error
}

func (m *mockTransport) Name() string { return m.name }

func (m *mockTransport) Chat(_ context.Context, _ ChatRequest) (*ChatResponse, error) {
	if m.chatErr != nil {
		return nil, m.chatErr
	}
	return m.chatResp, nil
}

func (m *mockTransport) ChatStream(_ context.Context, _ ChatRequest) (<-chan StreamDelta, <-chan error) {
	dCh := make(chan StreamDelta, len(m.streamData))
	eCh := make(chan error, 1)

	for _, d := range m.streamData {
		dCh <- d
	}
	close(dCh)

	if m.streamErr != nil {
		eCh <- m.streamErr
	}
	close(eCh)

	return dCh, eCh
}

func TestFallbackRouter_PrimarySucceeds(t *testing.T) {
	primary := &mockTransport{
		name:     "primary",
		chatResp: &ChatResponse{Content: "hello from primary"},
	}
	fallback := &mockTransport{
		name:     "fallback",
		chatResp: &ChatResponse{Content: "hello from fallback"},
	}
	router := NewFallbackRouter(primary, fallback)

	resp, err := router.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello from primary" {
		t.Errorf("expected primary response, got: %s", resp.Content)
	}
	if resp.Degraded {
		t.Error("expected Degraded=false when primary succeeds")
	}
}

func TestFallbackRouter_BreakerOpen_FallbackUsed(t *testing.T) {
	primary := &mockTransport{
		name:    "primary",
		chatErr: fmt.Errorf("circuit breaker [llm-model]: %w", gobreaker.ErrOpenState),
	}
	fallback := &mockTransport{
		name:     "fallback",
		chatResp: &ChatResponse{Content: "fallback response"},
	}
	router := NewFallbackRouter(primary, fallback)

	resp, err := router.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "fallback response" {
		t.Errorf("expected fallback response, got: %s", resp.Content)
	}
	if !resp.Degraded {
		t.Error("expected Degraded=true when fallback used")
	}
}

func TestFallbackRouter_Timeout_FallbackUsed(t *testing.T) {
	primary := &mockTransport{
		name:    "primary",
		chatErr: context.DeadlineExceeded,
	}
	fallback := &mockTransport{
		name:     "fallback",
		chatResp: &ChatResponse{Content: "timeout fallback"},
	}
	router := NewFallbackRouter(primary, fallback)

	resp, err := router.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "timeout fallback" {
		t.Errorf("expected fallback response, got: %s", resp.Content)
	}
	if !resp.Degraded {
		t.Error("expected Degraded=true on timeout fallback")
	}
}

func TestFallbackRouter_5xx_FallbackUsed(t *testing.T) {
	primary := &mockTransport{
		name:    "primary",
		chatErr: errors.New("request failed: status 503 service unavailable"),
	}
	fallback := &mockTransport{
		name:     "fallback",
		chatResp: &ChatResponse{Content: "5xx fallback"},
	}
	router := NewFallbackRouter(primary, fallback)

	resp, err := router.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "5xx fallback" {
		t.Errorf("expected fallback response, got: %s", resp.Content)
	}
	if !resp.Degraded {
		t.Error("expected Degraded=true on 5xx fallback")
	}
}

func TestFallbackRouter_4xx_NoFallback(t *testing.T) {
	primary := &mockTransport{
		name:    "primary",
		chatErr: errors.New("request failed: status 401 unauthorized"),
	}
	fallback := &mockTransport{
		name:     "fallback",
		chatResp: &ChatResponse{Content: "should not reach"},
	}
	router := NewFallbackRouter(primary, fallback)

	resp, err := router.Chat(context.Background(), ChatRequest{})
	if err == nil {
		t.Fatal("expected error for 4xx, got nil")
	}
	if resp != nil {
		t.Error("expected nil response for 4xx error")
	}
	if err.Error() != "request failed: status 401 unauthorized" {
		t.Errorf("expected original error, got: %v", err)
	}
}

func TestFallbackRouter_BothFail(t *testing.T) {
	primary := &mockTransport{
		name:    "primary",
		chatErr: fmt.Errorf("circuit breaker open for model"),
	}
	fallback := &mockTransport{
		name:    "fallback",
		chatErr: errors.New("fallback also failed"),
	}
	router := NewFallbackRouter(primary, fallback)

	resp, err := router.Chat(context.Background(), ChatRequest{})
	if err == nil {
		t.Fatal("expected error when both fail")
	}
	if resp != nil {
		t.Error("expected nil response when both fail")
	}
	if !errors.Is(err, fallback.chatErr) {
		t.Errorf("expected wrapped fallback error, got: %v", err)
	}
}

func TestFallbackRouter_Name(t *testing.T) {
	primary := &mockTransport{name: "openai"}
	fallback := &mockTransport{name: "anthropic"}
	router := NewFallbackRouter(primary, fallback)

	expected := "fallback(openai→anthropic)"
	if router.Name() != expected {
		t.Errorf("expected Name()=%q, got %q", expected, router.Name())
	}
}

func TestFallbackRouter_Stream_PrimarySucceeds(t *testing.T) {
	primary := &mockTransport{
		name:       "primary",
		streamData: []StreamDelta{{Content: "chunk1"}, {Content: "chunk2", Done: true}},
	}
	fallback := &mockTransport{
		name:       "fallback",
		streamData: []StreamDelta{{Content: "fb"}},
	}
	router := NewFallbackRouter(primary, fallback)

	deltaCh, errCh := router.ChatStream(context.Background(), ChatRequest{})

	var chunks []string
	for d := range deltaCh {
		chunks = append(chunks, d.Content)
	}
	if e, ok := <-errCh; ok && e != nil {
		t.Fatalf("unexpected stream error: %v", e)
	}
	if len(chunks) != 2 || chunks[0] != "chunk1" || chunks[1] != "chunk2" {
		t.Errorf("unexpected chunks: %v", chunks)
	}
}

func TestFallbackRouter_Stream_PrimaryFailsFallbackUsed(t *testing.T) {
	primary := &mockTransport{
		name:      "primary",
		streamErr: fmt.Errorf("circuit breaker open for primary"),
	}
	fallback := &mockTransport{
		name:       "fallback",
		streamData: []StreamDelta{{Content: "fb1"}, {Content: "fb2", Done: true}},
	}
	router := NewFallbackRouter(primary, fallback)

	deltaCh, errCh := router.ChatStream(context.Background(), ChatRequest{})

	var chunks []string
	for d := range deltaCh {
		chunks = append(chunks, d.Content)
	}
	if e, ok := <-errCh; ok && e != nil {
		t.Fatalf("unexpected stream error: %v", e)
	}
	if len(chunks) != 2 || chunks[0] != "fb1" || chunks[1] != "fb2" {
		t.Errorf("expected fallback chunks, got: %v", chunks)
	}
}

func TestFallbackRouter_Stream_4xxNoFallback(t *testing.T) {
	primary := &mockTransport{
		name:      "primary",
		streamErr: errors.New("status 400 bad request"),
	}
	fallback := &mockTransport{
		name:       "fallback",
		streamData: []StreamDelta{{Content: "should not appear"}},
	}
	router := NewFallbackRouter(primary, fallback)

	deltaCh, errCh := router.ChatStream(context.Background(), ChatRequest{})

	for range deltaCh {
	}
	e, ok := <-errCh
	if !ok || e == nil {
		t.Fatal("expected error for 4xx stream")
	}
	if !errors.Is(e, primary.streamErr) {
		t.Errorf("expected original error, got: %v", e)
	}
}

func TestIsFallbackEligible(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		eligible bool
	}{
		{"nil error", nil, false},
		{"gobreaker ErrOpenState", gobreaker.ErrOpenState, true},
		{"wrapped breaker error", fmt.Errorf("circuit breaker [x]: %w", gobreaker.ErrOpenState), true},
		{"breaker open string", errors.New("circuit breaker open for model"), true},
		{"deadline exceeded", context.DeadlineExceeded, true},
		{"wrapped deadline", fmt.Errorf("request: %w", context.DeadlineExceeded), true},
		{"500 error", errors.New("HTTP status 500"), true},
		{"503 error", errors.New("service unavailable 503"), true},
		{"502 bad gateway", errors.New("bad gateway"), true},
		{"504 gateway timeout", errors.New("gateway timeout"), true},
		{"401 unauthorized", errors.New("status 401 unauthorized"), false},
		{"403 forbidden", errors.New("status 403 forbidden"), false},
		{"422 unprocessable", errors.New("status 422 unprocessable"), false},
		{"random error", errors.New("something went wrong"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isFallbackEligible(tc.err)
			if got != tc.eligible {
				t.Errorf("isFallbackEligible(%v) = %v, want %v", tc.err, got, tc.eligible)
			}
		})
	}
}
