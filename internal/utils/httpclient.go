package utils

import (
	"net"
	"net/http"
	"time"
)

// newDefaultTransport creates a fresh http.Transport with production-grade
// defaults. Each call returns an independent instance to prevent callers
// from mutating shared state.
func newDefaultTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
	}
}

// NewHTTPClient creates an *http.Client with the given timeout and
// independent transport defaults. Use this instead of &http.Client{Timeout: X}
// to get consistent connection pooling and dial settings across the codebase.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: newDefaultTransport(),
	}
}

// LLMClientTimeout is the default timeout for LLM API calls (5 minutes).
const LLMClientTimeout = 300 * time.Second

// NewLLMHTTPClient creates an HTTP client configured for LLM API calls.
func NewLLMHTTPClient() *http.Client {
	return NewHTTPClient(LLMClientTimeout)
}
