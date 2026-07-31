package observability

import (
	"context"
	"log/slog"
	"testing"
)

func TestContextLogger_Fallback(t *testing.T) {
	// Without any enriched logger in context, should return default.
	ctx := context.Background()
	logger := ContextLogger(ctx)
	if logger == nil {
		t.Fatal("ContextLogger returned nil")
	}
	// Should be the default logger.
	if logger != slog.Default() {
		t.Error("ContextLogger should return slog.Default() when no logger in context")
	}
}

func TestWithLogger_And_ContextLogger(t *testing.T) {
	// Create a custom logger and store it.
	customLogger := slog.New(slog.NewTextHandler(nil, nil))
	ctx := context.Background()
	ctx = WithLogger(ctx, customLogger)

	logger := ContextLogger(ctx)
	if logger == nil {
		t.Fatal("ContextLogger returned nil")
	}
	if logger != customLogger {
		t.Error("ContextLogger should return the enriched logger")
	}
}

func TestWithLogger_NilLogger(t *testing.T) {
	// Storing nil should cause fallback to default.
	ctx := WithLogger(context.Background(), nil)
	logger := ContextLogger(ctx)
	if logger != slog.Default() {
		t.Error("ContextLogger should fall back to default when nil is stored")
	}
}

func TestWithLogger_Overwrite(t *testing.T) {
	logger1 := slog.New(slog.NewTextHandler(nil, nil))
	logger2 := slog.New(slog.NewTextHandler(nil, nil))

	ctx := WithLogger(context.Background(), logger1)
	ctx = WithLogger(ctx, logger2)

	if ContextLogger(ctx) != logger2 {
		t.Error("ContextLogger should return the last stored logger")
	}
}
