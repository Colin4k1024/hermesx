# Go SSE Streaming + Middleware Compatibility

SSE streaming breaks silently when ResponseWriter is wrapped by middleware (metrics, audit, logging) that doesn't implement http.Flusher.

## When This Applies

- `w.(http.Flusher)` returns false in a handler behind middleware
- SSE endpoint returns "streaming not supported" or 500
- Multi-turn chat creates new sessions per message (session ID mismatch due to prefix in SSE chunk ID)

## Solution

### Problem 1: Flusher not accessible through wrappers

**Wrong (breaks behind middleware):**
```go
flusher, ok := w.(http.Flusher)
if !ok {
    http.Error(w, "streaming not supported", 500)
    return
}
flusher.Flush()
```

**Correct (Go 1.20+ — traverses wrapper chain):**
```go
rc := http.NewResponseController(w)
// ... write headers, set status ...
if err := rc.Flush(); err != nil {
    log.Error("flush failed", "error", err)
    return
}
```

### Problem 2: Custom wrappers must expose Unwrap()

If you write a custom `ResponseWriter` wrapper (e.g., for metrics), add `Unwrap()` and `Flush()` so `http.NewResponseController` can traverse:

```go
type statusWriter struct {
    http.ResponseWriter
    status int
}

func (sw *statusWriter) Flush() {
    if f, ok := sw.ResponseWriter.(http.Flusher); ok {
        f.Flush()
    }
}

func (sw *statusWriter) Unwrap() http.ResponseWriter {
    return sw.ResponseWriter
}
```

### Problem 3: SSE chunk ID must match stored session ID

**Wrong (client sends back prefixed ID, DB lookup fails):**
```go
chunkID := "chatcmpl-" + sessionID  // Client captures "chatcmpl-sess_xxx"
```

**Correct (return raw session ID):**
```go
chunkID := sessionID  // Client captures "sess_xxx" — matches DB
```

### Problem 4: Prometheus /metrics must be public

Prometheus cannot authenticate. If `/metrics` is behind an auth middleware stack, the scrape target shows as DOWN.

```go
// Wrong — behind auth middleware
mux.Handle("GET /metrics", stack.Wrap(promhttp.Handler()))

// Correct — public route
mux.Handle("GET /metrics", promhttp.Handler())
```

## Notes

- `http.NewResponseController` was added in Go 1.20 — use type assertions for older versions
- The Unwrap pattern follows `errors.Unwrap` convention — Go's HTTP package uses it to find interfaces through nested wrappers
- Always test SSE through the full middleware stack, not just the handler in isolation
- MySQL `CHAR(36)` is too short for session IDs with prefixes like `sess_` + 32 hex chars — use `VARCHAR(64)`
- `*time.Time` pointer fields will panic on `.IsZero()` if nil — always nil-check first

## Origin

Extracted from HermesX K8s deployment session (2026-05-09). All issues discovered during E2E browser testing with Playwright across multi-tenant scenarios.
