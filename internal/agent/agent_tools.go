package agent

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Colin4k1024/hermesx/internal/egress"
	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

// executeToolCalls runs tool calls, parallelizing when safe.
// Uses smart path-based overlap detection for file-scoped tools.
func (a *AIAgent) executeToolCalls(ctx context.Context, toolCalls []llm.ToolCall) []llm.Message {
	if len(toolCalls) == 1 || !ShouldParallelizeToolBatch(toolCalls) {
		// Sequential execution
		var results []llm.Message
		for _, tc := range toolCalls {
			if a.isInterrupted() {
				break
			}
			results = append(results, a.executeSingleTool(ctx, tc))
		}
		return results
	}

	// Parallel execution with WaitGroup + timeout
	type indexedResult struct {
		index int
		msg   llm.Message
	}

	parallelCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var wg sync.WaitGroup
	resultCh := make(chan indexedResult, len(toolCalls))
	sem := make(chan struct{}, MaxParallelWorkers)

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, call llm.ToolCall) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Tool panicked", "tool", call.Function.Name, "panic", r)
					resultCh <- indexedResult{index: idx, msg: llm.Message{
						Role:       "tool",
						Content:    fmt.Sprintf(`{"error":"tool panicked: %v"}`, r),
						ToolCallID: call.ID,
						ToolName:   call.Function.Name,
					}}
				}
			}()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				msg := a.executeSingleTool(parallelCtx, call)
				resultCh <- indexedResult{index: idx, msg: msg}
			case <-parallelCtx.Done():
				resultCh <- indexedResult{index: idx, msg: llm.Message{
					Role:       "tool",
					Content:    `{"error":"tool execution timed out"}`,
					ToolCallID: call.ID,
					ToolName:   call.Function.Name,
				}}
			}
		}(i, tc)
	}

	go func() { wg.Wait(); close(resultCh) }()

	collected := make([]llm.Message, len(toolCalls))
	for ir := range resultCh {
		collected[ir.index] = ir.msg
	}

	return collected
}

// buildToolHTTPClient creates a per-call http.Client backed by the shared
// SecureTransport with redirect policy based on the tool's MaxRedirects.
func (a *AIAgent) buildToolHTTPClient(maxRedirects int) *http.Client {
	return &http.Client{
		Transport: a.sharedTransport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if maxRedirects == 0 {
				return http.ErrUseLastResponse
			}
			// Reject redirects whose Location target is a blocked IP literal.
			if err := egress.ValidateRedirectTarget(req); err != nil {
				return err
			}
			if len(via) >= maxRedirects {
				return egress.ErrNotAllowed
			}
			return nil
		},
	}
}

// buildToolContext constructs the ToolContext for a given tool call.
func (a *AIAgent) buildToolContext(tc llm.ToolCall) *tools.ToolContext {
	entry := tools.Registry().Lookup(tc.Function.Name)
	maxRedirects := 0
	if entry != nil {
		maxRedirects = entry.MaxRedirects
	}
	return &tools.ToolContext{
		SessionID:      a.sessionID,
		ToolCallID:     tc.ID,
		Platform:       a.platform,
		TenantID:       a.tenantID,
		UserID:         a.userID,
		MemoryProvider: a.memoryProvider,
		HTTPClient:     a.buildToolHTTPClient(maxRedirects),
		SecretResolver: a.secretResolver,
		Interceptor:    tools.WrapSafetyInterceptor(a.safetyInterceptor),
	}
}

func (a *AIAgent) executeSingleTool(ctx context.Context, tc llm.ToolCall) llm.Message {
	toolName := tc.Function.Name
	a.streamHandler.FireToolStart(toolName)
	a.streamHandler.FireToolProgress(toolName, truncate(tc.Function.Arguments, 100))

	args, err := llm.ParseToolArgs(tc.Function.Arguments)
	if err != nil {
		args = map[string]any{}
		slog.Warn("Failed to parse tool args", "tool", toolName, "error", err)
	}

	// Attach tenant ID to context for egress policy enforcement.
	ctx = egress.WithTenant(ctx, a.tenantID)

	toolCtx := a.buildToolContext(tc)
	toolResult := tools.Registry().Dispatch(ctx, toolName, args, toolCtx)

	// Redact secrets before the result enters conversation history.
	toolResult = RedactSecrets(toolResult)

	// Save oversized results to disk.
	if IsOversizedResult(toolResult) {
		slog.Info("Tool result oversized, saving to file", "tool", toolName, "chars", len(toolResult))
		toolResult = SaveOversizedResult(toolName, toolResult)
	}

	a.streamHandler.FireToolComplete(toolName)

	return llm.Message{
		Role:       "tool",
		Content:    toolResult,
		ToolCallID: tc.ID,
		ToolName:   toolName,
	}
}
