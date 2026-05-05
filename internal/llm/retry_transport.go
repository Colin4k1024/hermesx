package llm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sony/gobreaker/v2"
)

// RetryTransport is a Transport decorator that adds exponential backoff retry
// with jitter. It retries on transient errors (5xx, timeout, 429 rate limit)
// but does not retry on client errors (4xx except 429), context cancellation,
// or circuit breaker open errors.
type RetryTransport struct {
	inner      Transport
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
	logger     *slog.Logger
	sleepFn    func(time.Duration) // injectable for testing; nil means use real timer
}

// RetryOption configures a RetryTransport.
type RetryOption func(*RetryTransport)

// WithMaxRetries sets the maximum number of retry attempts. Default: 3.
func WithMaxRetries(n int) RetryOption {
	return func(rt *RetryTransport) {
		if n >= 0 {
			rt.maxRetries = n
		}
	}
}

// WithBaseDelay sets the initial backoff delay. Default: 500ms.
func WithBaseDelay(d time.Duration) RetryOption {
	return func(rt *RetryTransport) {
		if d > 0 {
			rt.baseDelay = d
		}
	}
}

// WithMaxDelay sets the maximum backoff delay cap. Default: 10s.
func WithMaxDelay(d time.Duration) RetryOption {
	return func(rt *RetryTransport) {
		if d > 0 {
			rt.maxDelay = d
		}
	}
}

// withSleepFn is an internal option for testing to replace time.Sleep.
func withSleepFn(fn func(time.Duration)) RetryOption {
	return func(rt *RetryTransport) {
		rt.sleepFn = fn
	}
}

// NewRetryTransport creates a RetryTransport wrapping the given inner transport.
func NewRetryTransport(inner Transport, opts ...RetryOption) *RetryTransport {
	rt := &RetryTransport{
		inner:      inner,
		maxRetries: 3,
		baseDelay:  500 * time.Millisecond,
		maxDelay:   10 * time.Second,
		logger:     slog.Default(),
		sleepFn:    nil, // nil means use real timer with context support
	}
	for _, opt := range opts {
		opt(rt)
	}
	return rt
}

// Chat sends a chat request, retrying on transient errors with exponential backoff.
func (rt *RetryTransport) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= rt.maxRetries; attempt++ {
		if attempt > 0 {
			delay := rt.calcDelay(attempt - 1)
			rt.logger.Info("retry_transport_backoff",
				"transport", rt.inner.Name(),
				"attempt", attempt,
				"delay", delay.String(),
			)
			if err := rt.sleepWithContext(ctx, delay); err != nil {
				return nil, fmt.Errorf("retry cancelled during backoff: %w", lastErr)
			}
		}

		resp, err := rt.inner.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		if !isRetryEligible(err) {
			return nil, err
		}

		rt.logger.Warn("retry_transport_attempt_failed",
			"transport", rt.inner.Name(),
			"attempt", attempt,
			"error", err.Error(),
		)
	}

	return nil, fmt.Errorf("retry exhausted after %d attempts: %w", rt.maxRetries+1, lastErr)
}

// ChatStream sends a streaming chat request. Only retries if the error arrives
// before any delta is emitted. Once streaming has started, errors are not retried.
func (rt *RetryTransport) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamDelta, <-chan error) {
	proxyDelta := make(chan StreamDelta, 1)
	proxyErr := make(chan error, 1)

	go func() {
		defer close(proxyDelta)
		defer close(proxyErr)

		rt.streamWithRetry(ctx, req, proxyDelta, proxyErr)
	}()

	return proxyDelta, proxyErr
}

func (rt *RetryTransport) streamWithRetry(ctx context.Context, req ChatRequest, proxyDelta chan<- StreamDelta, proxyErr chan<- error) {
	var lastErr error

	for attempt := 0; attempt <= rt.maxRetries; attempt++ {
		if attempt > 0 {
			delay := rt.calcDelay(attempt - 1)
			rt.logger.Info("retry_transport_stream_backoff",
				"transport", rt.inner.Name(),
				"attempt", attempt,
				"delay", delay.String(),
			)
			if err := rt.sleepWithContext(ctx, delay); err != nil {
				proxyErr <- fmt.Errorf("retry cancelled during backoff: %w", lastErr)
				return
			}
		}

		deltaCh, errCh := rt.inner.ChatStream(ctx, req)

		dataReceived := false
		for d := range deltaCh {
			dataReceived = true
			proxyDelta <- d
		}

		// Check for error after delta channel closes.
		if e, ok := <-errCh; ok && e != nil {
			if dataReceived {
				// Data already sent to caller — cannot retry.
				proxyErr <- e
				return
			}

			lastErr = e
			if !isRetryEligible(e) {
				proxyErr <- e
				return
			}

			rt.logger.Warn("retry_transport_stream_attempt_failed",
				"transport", rt.inner.Name(),
				"attempt", attempt,
				"error", e.Error(),
			)
			continue
		}

		// Success — no error.
		return
	}

	proxyErr <- fmt.Errorf("retry exhausted after %d attempts: %w", rt.maxRetries+1, lastErr)
}

// Name returns the decorated transport name.
func (rt *RetryTransport) Name() string {
	return fmt.Sprintf("retry(%s)", rt.inner.Name())
}

// calcDelay computes the backoff delay for a given attempt with ±25% jitter.
func (rt *RetryTransport) calcDelay(attempt int) time.Duration {
	exp := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(rt.baseDelay) * exp)

	delay = min(delay, rt.maxDelay)

	// Add ±25% jitter.
	jitterRange := float64(delay) * 0.25
	jitter := (rand.Float64()*2 - 1) * jitterRange
	delay = time.Duration(float64(delay) + jitter)

	delay = max(delay, 0)

	return delay
}

// sleepWithContext sleeps for the given duration, respecting context cancellation.
// When sleepFn is set (for testing), it calls sleepFn directly.
func (rt *RetryTransport) sleepWithContext(ctx context.Context, d time.Duration) error {
	// Check if already cancelled.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// If sleepFn is set (test override), use it directly.
	if rt.sleepFn != nil {
		rt.sleepFn(d)
		// Re-check context after sleep (test may have cancelled during sleepFn).
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		return nil
	}

	// Production path: use timer with context support.
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// isRetryEligible determines whether an error is transient and worth retrying.
// Retry on: 5xx, timeout, rate limit (429).
// Do NOT retry on: 4xx (except 429), context cancelled, circuit breaker open.
func isRetryEligible(err error) bool {
	if err == nil {
		return false
	}

	// Circuit breaker open — do not retry (let caller handle fallback).
	if errors.Is(err, gobreaker.ErrOpenState) {
		return false
	}
	if strings.Contains(err.Error(), "circuit breaker open") {
		return false
	}

	// Context cancelled — do not retry.
	if errors.Is(err, context.Canceled) {
		return false
	}

	// Context deadline exceeded — retriable (the request timed out).
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if strings.Contains(err.Error(), "deadline exceeded") {
		return true
	}

	msg := err.Error()

	// Rate limit (429) — retriable.
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") || strings.Contains(msg, "too many requests") {
		return true
	}

	// Server-side errors (5xx) — retriable.
	if strings.Contains(msg, "status 5") ||
		strings.Contains(msg, "500") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "504") ||
		strings.Contains(msg, "internal server error") ||
		strings.Contains(msg, "bad gateway") ||
		strings.Contains(msg, "service unavailable") ||
		strings.Contains(msg, "gateway timeout") {
		// Exclude 4xx that might contain these substrings.
		if isLikelyClient4xx(msg) {
			return false
		}
		return true
	}

	return false
}

// isLikelyClient4xx checks if an error message looks like a non-retriable 4xx.
func isLikelyClient4xx(msg string) bool {
	return strings.Contains(msg, "status 4") ||
		strings.Contains(msg, "400") ||
		strings.Contains(msg, "401") ||
		strings.Contains(msg, "403") ||
		strings.Contains(msg, "404") ||
		strings.Contains(msg, "422")
	// Note: 429 is intentionally excluded — it is retriable.
}

// ParseRetryAfter extracts a Retry-After value from an HTTP response.
// Returns 0 if not present or unparseable.
func ParseRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	val := resp.Header.Get("Retry-After")
	if val == "" {
		return 0
	}

	// Try as seconds first.
	if secs, err := strconv.Atoi(val); err == nil {
		return time.Duration(secs) * time.Second
	}

	// Try as HTTP date.
	if t, err := http.ParseTime(val); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}

	return 0
}

var _ Transport = (*RetryTransport)(nil)
