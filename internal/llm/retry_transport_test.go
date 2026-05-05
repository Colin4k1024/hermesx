package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sony/gobreaker/v2"
)

// retryMockTransport is a test double that can return different results per call.
type retryMockTransport struct {
	name      string
	chatCalls atomic.Int32
	chatFn    func(attempt int) (*ChatResponse, error)
	streamFn  func(attempt int) ([]StreamDelta, error)
}

func (m *retryMockTransport) Name() string { return m.name }

func (m *retryMockTransport) Chat(_ context.Context, _ ChatRequest) (*ChatResponse, error) {
	attempt := int(m.chatCalls.Add(1)) - 1
	return m.chatFn(attempt)
}

func (m *retryMockTransport) ChatStream(_ context.Context, _ ChatRequest) (<-chan StreamDelta, <-chan error) {
	attempt := int(m.chatCalls.Add(1)) - 1
	deltas, err := m.streamFn(attempt)

	dCh := make(chan StreamDelta, len(deltas))
	eCh := make(chan error, 1)

	for _, d := range deltas {
		dCh <- d
	}
	close(dCh)

	if err != nil {
		eCh <- err
	}
	close(eCh)

	return dCh, eCh
}

// noopSleep replaces time.Sleep for instant test execution.
func noopSleep(_ time.Duration) {}

func TestRetryTransport_SucceedsFirstTry(t *testing.T) {
	mock := &retryMockTransport{
		name: "test",
		chatFn: func(_ int) (*ChatResponse, error) {
			return &ChatResponse{Content: "ok"}, nil
		},
	}

	rt := NewRetryTransport(mock, withSleepFn(noopSleep))
	resp, err := rt.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("expected 'ok', got %q", resp.Content)
	}
	if mock.chatCalls.Load() != 1 {
		t.Errorf("expected 1 call, got %d", mock.chatCalls.Load())
	}
}

func TestRetryTransport_FailsTwiceThenSucceeds(t *testing.T) {
	mock := &retryMockTransport{
		name: "test",
		chatFn: func(attempt int) (*ChatResponse, error) {
			if attempt < 2 {
				return nil, errors.New("status 503 service unavailable")
			}
			return &ChatResponse{Content: "recovered"}, nil
		},
	}

	var delays []time.Duration
	trackSleep := func(d time.Duration) { delays = append(delays, d) }

	rt := NewRetryTransport(mock, withSleepFn(trackSleep))
	resp, err := rt.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "recovered" {
		t.Errorf("expected 'recovered', got %q", resp.Content)
	}
	if mock.chatCalls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", mock.chatCalls.Load())
	}
	if len(delays) != 2 {
		t.Errorf("expected 2 backoff sleeps, got %d", len(delays))
	}
	// Second delay should be roughly double the first (exponential backoff).
	if len(delays) == 2 && delays[1] < delays[0] {
		t.Errorf("expected second delay >= first, got %v then %v", delays[0], delays[1])
	}
}

func TestRetryTransport_AllRetriesFail(t *testing.T) {
	mock := &retryMockTransport{
		name: "test",
		chatFn: func(_ int) (*ChatResponse, error) {
			return nil, errors.New("status 500 internal server error")
		},
	}

	rt := NewRetryTransport(mock, WithMaxRetries(2), withSleepFn(noopSleep))
	_, err := rt.Chat(context.Background(), ChatRequest{})
	if err == nil {
		t.Fatal("expected error after all retries exhausted")
	}
	if !strings.Contains(err.Error(), "retry exhausted") {
		t.Errorf("expected 'retry exhausted' in error, got: %v", err)
	}
	// 1 initial + 2 retries = 3 total calls.
	if mock.chatCalls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", mock.chatCalls.Load())
	}
}

func TestRetryTransport_4xxNoRetry(t *testing.T) {
	mock := &retryMockTransport{
		name: "test",
		chatFn: func(_ int) (*ChatResponse, error) {
			return nil, errors.New("status 401 unauthorized")
		},
	}

	rt := NewRetryTransport(mock, withSleepFn(noopSleep))
	_, err := rt.Chat(context.Background(), ChatRequest{})
	if err == nil {
		t.Fatal("expected error for 4xx")
	}
	if mock.chatCalls.Load() != 1 {
		t.Errorf("expected 1 call (no retry for 4xx), got %d", mock.chatCalls.Load())
	}
}

func TestRetryTransport_429IsRetried(t *testing.T) {
	mock := &retryMockTransport{
		name: "test",
		chatFn: func(attempt int) (*ChatResponse, error) {
			if attempt == 0 {
				return nil, errors.New("rate limit exceeded: 429 too many requests")
			}
			return &ChatResponse{Content: "ok"}, nil
		},
	}

	rt := NewRetryTransport(mock, withSleepFn(noopSleep))
	resp, err := rt.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("expected 'ok', got %q", resp.Content)
	}
	if mock.chatCalls.Load() != 2 {
		t.Errorf("expected 2 calls (one retry for 429), got %d", mock.chatCalls.Load())
	}
}

func TestRetryTransport_ContextCancelled_StopsRetrying(t *testing.T) {
	mock := &retryMockTransport{
		name: "test",
		chatFn: func(_ int) (*ChatResponse, error) {
			return nil, errors.New("status 503 service unavailable")
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	sleepFn := func(_ time.Duration) {
		cancel() // Cancel during first backoff sleep.
	}

	rt := NewRetryTransport(mock, withSleepFn(sleepFn))
	_, err := rt.Chat(ctx, ChatRequest{})
	if err == nil {
		t.Fatal("expected error after context cancellation")
	}
	if !strings.Contains(err.Error(), "retry cancelled") {
		t.Errorf("expected 'retry cancelled' in error, got: %v", err)
	}
	// Should have made 1 initial attempt, then cancelled during backoff.
	if mock.chatCalls.Load() != 1 {
		t.Errorf("expected 1 call before cancel, got %d", mock.chatCalls.Load())
	}
}

func TestRetryTransport_ContextCancelledError_NoRetry(t *testing.T) {
	mock := &retryMockTransport{
		name: "test",
		chatFn: func(_ int) (*ChatResponse, error) {
			return nil, context.Canceled
		},
	}

	rt := NewRetryTransport(mock, withSleepFn(noopSleep))
	_, err := rt.Chat(context.Background(), ChatRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	if mock.chatCalls.Load() != 1 {
		t.Errorf("expected 1 call (no retry for cancelled), got %d", mock.chatCalls.Load())
	}
}

func TestRetryTransport_CircuitBreakerOpen_NoRetry(t *testing.T) {
	mock := &retryMockTransport{
		name: "test",
		chatFn: func(_ int) (*ChatResponse, error) {
			return nil, fmt.Errorf("circuit breaker [model]: %w", gobreaker.ErrOpenState)
		},
	}

	rt := NewRetryTransport(mock, withSleepFn(noopSleep))
	_, err := rt.Chat(context.Background(), ChatRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if mock.chatCalls.Load() != 1 {
		t.Errorf("expected 1 call (no retry for breaker open), got %d", mock.chatCalls.Load())
	}
}

func TestRetryTransport_MaxDelayCap(t *testing.T) {
	mock := &retryMockTransport{
		name: "test",
		chatFn: func(attempt int) (*ChatResponse, error) {
			if attempt < 5 {
				return nil, errors.New("status 500")
			}
			return &ChatResponse{Content: "ok"}, nil
		},
	}

	var delays []time.Duration
	trackSleep := func(d time.Duration) { delays = append(delays, d) }

	maxDelay := 2 * time.Second
	rt := NewRetryTransport(mock,
		WithMaxRetries(5),
		WithBaseDelay(500*time.Millisecond),
		WithMaxDelay(maxDelay),
		withSleepFn(trackSleep),
	)
	_, _ = rt.Chat(context.Background(), ChatRequest{})

	for i, d := range delays {
		// With ±25% jitter, max possible is maxDelay * 1.25.
		if d > time.Duration(float64(maxDelay)*1.25)+time.Millisecond {
			t.Errorf("delay[%d] = %v exceeds max cap %v (with jitter)", i, d, maxDelay)
		}
	}
}

func TestRetryTransport_Name(t *testing.T) {
	mock := &retryMockTransport{name: "openai"}
	rt := NewRetryTransport(mock)
	expected := "retry(openai)"
	if rt.Name() != expected {
		t.Errorf("expected Name()=%q, got %q", expected, rt.Name())
	}
}

// --- Stream tests ---

func TestRetryTransport_Stream_SucceedsFirstTry(t *testing.T) {
	mock := &retryMockTransport{
		name: "test",
		streamFn: func(_ int) ([]StreamDelta, error) {
			return []StreamDelta{{Content: "hi"}, {Content: "!", Done: true}}, nil
		},
	}

	rt := NewRetryTransport(mock, withSleepFn(noopSleep))
	deltaCh, errCh := rt.ChatStream(context.Background(), ChatRequest{})

	var chunks []string
	for d := range deltaCh {
		chunks = append(chunks, d.Content)
	}
	if e, ok := <-errCh; ok && e != nil {
		t.Fatalf("unexpected stream error: %v", e)
	}
	if len(chunks) != 2 || chunks[0] != "hi" || chunks[1] != "!" {
		t.Errorf("unexpected chunks: %v", chunks)
	}
	if mock.chatCalls.Load() != 1 {
		t.Errorf("expected 1 call, got %d", mock.chatCalls.Load())
	}
}

func TestRetryTransport_Stream_FailsThenSucceeds(t *testing.T) {
	mock := &retryMockTransport{
		name: "test",
		streamFn: func(attempt int) ([]StreamDelta, error) {
			if attempt == 0 {
				return nil, errors.New("status 503 service unavailable")
			}
			return []StreamDelta{{Content: "ok", Done: true}}, nil
		},
	}

	rt := NewRetryTransport(mock, withSleepFn(noopSleep))
	deltaCh, errCh := rt.ChatStream(context.Background(), ChatRequest{})

	var chunks []string
	for d := range deltaCh {
		chunks = append(chunks, d.Content)
	}
	if e, ok := <-errCh; ok && e != nil {
		t.Fatalf("unexpected stream error: %v", e)
	}
	if len(chunks) != 1 || chunks[0] != "ok" {
		t.Errorf("unexpected chunks: %v", chunks)
	}
	if mock.chatCalls.Load() != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", mock.chatCalls.Load())
	}
}

func TestRetryTransport_Stream_DataReceivedNoRetry(t *testing.T) {
	mock := &retryMockTransport{
		name: "test",
		streamFn: func(_ int) ([]StreamDelta, error) {
			// Send data then fail — should NOT retry.
			return []StreamDelta{{Content: "partial"}}, errors.New("status 503 mid-stream")
		},
	}

	rt := NewRetryTransport(mock, withSleepFn(noopSleep))
	deltaCh, errCh := rt.ChatStream(context.Background(), ChatRequest{})

	var chunks []string
	for d := range deltaCh {
		chunks = append(chunks, d.Content)
	}
	e, ok := <-errCh
	if !ok || e == nil {
		t.Fatal("expected stream error after partial data")
	}
	if mock.chatCalls.Load() != 1 {
		t.Errorf("expected 1 call (no retry after data received), got %d", mock.chatCalls.Load())
	}
	if len(chunks) != 1 || chunks[0] != "partial" {
		t.Errorf("expected partial chunk forwarded, got: %v", chunks)
	}
}

func TestRetryTransport_Stream_4xxNoRetry(t *testing.T) {
	mock := &retryMockTransport{
		name: "test",
		streamFn: func(_ int) ([]StreamDelta, error) {
			return nil, errors.New("status 400 bad request")
		},
	}

	rt := NewRetryTransport(mock, withSleepFn(noopSleep))
	deltaCh, errCh := rt.ChatStream(context.Background(), ChatRequest{})

	for range deltaCh {
	}
	e, ok := <-errCh
	if !ok || e == nil {
		t.Fatal("expected error for 4xx stream")
	}
	if mock.chatCalls.Load() != 1 {
		t.Errorf("expected 1 call (no retry for 4xx), got %d", mock.chatCalls.Load())
	}
}

func TestIsRetryEligible(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		eligible bool
	}{
		{"nil error", nil, false},
		{"500 error", errors.New("status 500 internal server error"), true},
		{"503 error", errors.New("status 503 service unavailable"), true},
		{"502 bad gateway", errors.New("bad gateway"), true},
		{"504 gateway timeout", errors.New("gateway timeout"), true},
		{"429 rate limit", errors.New("429 too many requests"), true},
		{"rate limit text", errors.New("rate limit exceeded"), true},
		{"deadline exceeded", context.DeadlineExceeded, true},
		{"wrapped deadline", fmt.Errorf("request: %w", context.DeadlineExceeded), true},
		{"401 unauthorized", errors.New("status 401 unauthorized"), false},
		{"403 forbidden", errors.New("status 403 forbidden"), false},
		{"404 not found", errors.New("status 404 not found"), false},
		{"context cancelled", context.Canceled, false},
		{"breaker open", gobreaker.ErrOpenState, false},
		{"wrapped breaker", fmt.Errorf("breaker: %w", gobreaker.ErrOpenState), false},
		{"breaker open string", errors.New("circuit breaker open for model"), false},
		{"random error", errors.New("something went wrong"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isRetryEligible(tc.err)
			if got != tc.eligible {
				t.Errorf("isRetryEligible(%v) = %v, want %v", tc.err, got, tc.eligible)
			}
		})
	}
}
