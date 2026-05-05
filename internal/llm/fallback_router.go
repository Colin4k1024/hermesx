package llm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sony/gobreaker/v2"
)

// FallbackRouter wraps a primary and fallback Transport. When the primary
// fails with a retriable infrastructure error (circuit breaker open, timeout,
// or server-side 5xx), the request is retried on the fallback transport.
// Client errors (4xx) are never retried.
type FallbackRouter struct {
	primary  Transport
	fallback Transport
	logger   *slog.Logger
}

// NewFallbackRouter creates a FallbackRouter with the given primary and fallback transports.
func NewFallbackRouter(primary, fallback Transport) *FallbackRouter {
	return &FallbackRouter{
		primary:  primary,
		fallback: fallback,
		logger:   slog.Default(),
	}
}

// Chat sends a chat request to the primary transport. If it fails with a
// retriable error, the request is sent to the fallback transport and the
// response is marked as Degraded.
func (f *FallbackRouter) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	resp, err := f.primary.Chat(ctx, req)
	if err == nil {
		return resp, nil
	}

	if !isFallbackEligible(err) {
		return nil, err
	}

	f.logger.Warn("fallback_router_triggered",
		"primary", f.primary.Name(),
		"fallback", f.fallback.Name(),
		"primary_error", err.Error(),
	)

	resp, fallbackErr := f.fallback.Chat(ctx, req)
	if fallbackErr != nil {
		return nil, fmt.Errorf("fallback [%s] also failed: %w (primary error: %v)", f.fallback.Name(), fallbackErr, err)
	}
	resp.Degraded = true
	return resp, nil
}

// ChatStream sends a streaming chat request to the primary transport. If the
// primary returns an error before any data arrives, the request is retried on
// the fallback transport.
func (f *FallbackRouter) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamDelta, <-chan error) {
	deltaCh, errCh := f.primary.ChatStream(ctx, req)

	// We wrap in proxy channels so we can intercept errors and decide
	// whether to fall back before any data reaches the caller.
	proxyDelta := make(chan StreamDelta, 1)
	proxyErr := make(chan error, 1)

	go func() {
		defer close(proxyDelta)
		defer close(proxyErr)

		dataReceived := false

		// First, drain the delta channel, forwarding data to the proxy.
		for d := range deltaCh {
			dataReceived = true
			proxyDelta <- d
		}

		// Delta channel is closed. Now check for an error.
		if e, ok := <-errCh; ok && e != nil {
			if !dataReceived && isFallbackEligible(e) {
				f.doStreamFallback(ctx, req, proxyDelta, proxyErr)
				return
			}
			proxyErr <- e
		}
	}()

	return proxyDelta, proxyErr
}

// doStreamFallback executes the fallback stream and pipes results to proxy channels.
func (f *FallbackRouter) doStreamFallback(ctx context.Context, req ChatRequest, proxyDelta chan<- StreamDelta, proxyErr chan<- error) {
	f.logger.Warn("fallback_router_stream_triggered",
		"primary", f.primary.Name(),
		"fallback", f.fallback.Name(),
	)

	fbDelta, fbErr := f.fallback.ChatStream(ctx, req)
	for d := range fbDelta {
		proxyDelta <- d
	}
	if e, ok := <-fbErr; ok && e != nil {
		proxyErr <- fmt.Errorf("fallback stream [%s]: %w", f.fallback.Name(), e)
	}
}

// Name returns a composite name identifying the router and its transports.
func (f *FallbackRouter) Name() string {
	return fmt.Sprintf("fallback(%s→%s)", f.primary.Name(), f.fallback.Name())
}

// isFallbackEligible determines whether an error from the primary transport
// warrants falling back to the secondary. We trigger on:
//   - Circuit breaker open (gobreaker.ErrOpenState or message contains "circuit breaker open")
//   - Context deadline exceeded / timeout
//   - HTTP 5xx (status code >= 500 in error message)
//
// We do NOT trigger on 4xx client errors.
func isFallbackEligible(err error) bool {
	if err == nil {
		return false
	}

	// Circuit breaker open.
	if errors.Is(err, gobreaker.ErrOpenState) {
		return true
	}
	if strings.Contains(err.Error(), "circuit breaker open") {
		return true
	}

	// Context timeout / deadline exceeded.
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if strings.Contains(err.Error(), "deadline exceeded") {
		return true
	}

	// Server-side errors (5xx). Check for common patterns.
	msg := err.Error()
	if strings.Contains(msg, "status 5") ||
		strings.Contains(msg, "500") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "504") ||
		strings.Contains(msg, "internal server error") ||
		strings.Contains(msg, "bad gateway") ||
		strings.Contains(msg, "service unavailable") ||
		strings.Contains(msg, "gateway timeout") {
		// Make sure it's not a 4xx that happens to contain these digits elsewhere.
		if isLikely4xx(msg) {
			return false
		}
		return true
	}

	return false
}

// isLikely4xx checks if the error message indicates a 4xx client error.
func isLikely4xx(msg string) bool {
	return strings.Contains(msg, "status 4") ||
		strings.Contains(msg, "400") ||
		strings.Contains(msg, "401") ||
		strings.Contains(msg, "403") ||
		strings.Contains(msg, "404") ||
		strings.Contains(msg, "422") ||
		strings.Contains(msg, "429")
}

var _ Transport = (*FallbackRouter)(nil)
