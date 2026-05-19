package eino

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Colin4k1024/hermesx/internal/safety"
	"github.com/Colin4k1024/hermesx/internal/secrets"
)

// SafetyHook wraps HermesX SafetyInterceptor for use as pre/post agent hooks.
type SafetyHook struct {
	interceptor safety.SafetyInterceptor
}

func NewSafetyHook(interceptor safety.SafetyInterceptor) *SafetyHook {
	return &SafetyHook{interceptor: interceptor}
}

// CheckInput validates messages before passing to the LLM.
func (h *SafetyHook) CheckInput(ctx context.Context, tenantID string, userMessage string) error {
	if h.interceptor == nil {
		return nil
	}

	msgs := []safety.Message{{Role: "user", Content: userMessage}}
	result, err := h.interceptor.CheckInput(ctx, tenantID, msgs)
	if err != nil {
		if h.interceptor.IsModeEnforce(ctx, tenantID) {
			return fmt.Errorf("safety check failed: %w", err)
		}
		slog.Warn("safety input check error (non-enforce)", "error", err)
		return nil
	}

	if !result.Allowed {
		return fmt.Errorf("input blocked: %s", result.Reason)
	}
	return nil
}

// CheckOutput validates LLM output before returning to user.
func (h *SafetyHook) CheckOutput(ctx context.Context, tenantID string, output string) (string, error) {
	if h.interceptor == nil {
		return output, nil
	}

	result, err := h.interceptor.CheckOutput(ctx, tenantID, output)
	if err != nil {
		if h.interceptor.IsModeEnforce(ctx, tenantID) {
			return "", fmt.Errorf("output safety check failed: %w", err)
		}
		slog.Warn("safety output check error (non-enforce)", "error", err)
		return output, nil
	}

	if !result.Allowed {
		return "", fmt.Errorf("output blocked: %s", result.Reason)
	}
	return output, nil
}

// RedactionHook wraps LeakScanner to redact secrets from tool outputs.
type RedactionHook struct {
	scanner *secrets.LeakScanner
}

func NewRedactionHook(scanner *secrets.LeakScanner) *RedactionHook {
	return &RedactionHook{scanner: scanner}
}

// RedactToolOutput scans tool output for secrets and replaces them.
func (h *RedactionHook) RedactToolOutput(output string) string {
	if h.scanner == nil {
		return output
	}
	redacted, matches := h.scanner.Redact(output)
	if len(matches) > 0 {
		slog.Warn("secrets redacted from tool output", "count", len(matches))
	}
	return redacted
}

// BudgetHook tracks token/cost budget per session.
type BudgetHook struct {
	maxIterations int
	currentIter   int
}

func NewBudgetHook(maxIterations int) *BudgetHook {
	if maxIterations <= 0 {
		maxIterations = 20
	}
	return &BudgetHook{maxIterations: maxIterations}
}

// PreIteration checks if budget is exhausted before a new iteration.
func (h *BudgetHook) PreIteration() error {
	h.currentIter++
	if h.currentIter > h.maxIterations {
		return fmt.Errorf("budget exhausted: exceeded %d iterations", h.maxIterations)
	}
	return nil
}

// Reset resets the iteration counter for a new conversation.
func (h *BudgetHook) Reset() {
	h.currentIter = 0
}

// Iteration returns the current iteration count.
func (h *BudgetHook) Iteration() int {
	return h.currentIter
}
