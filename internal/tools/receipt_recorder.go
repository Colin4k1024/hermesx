package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/observability"
	"github.com/Colin4k1024/hermesx/internal/store"
)

// ReceiptRecorder records tool executions as auditable ExecutionReceipts.
type ReceiptRecorder struct {
	store store.ExecutionReceiptStore
}

func NewReceiptRecorder(s store.ExecutionReceiptStore) *ReceiptRecorder {
	if s == nil {
		return nil
	}
	return &ReceiptRecorder{store: s}
}

// Record wraps a tool invocation, capturing timing and result as an ExecutionReceipt.
func (rr *ReceiptRecorder) Record(ctx context.Context, toolName string, args map[string]any, tctx *ToolContext, result string) {
	if rr == nil || rr.store == nil {
		return
	}
	if tctx == nil || tctx.TenantID == "" {
		return
	}

	inputBytes, _ := json.Marshal(args)
	input := string(inputBytes)
	if len(input) > 4096 {
		input = input[:4096]
	}

	output := result
	if len(output) > 4096 {
		output = output[:4096]
	}

	status := "success"
	if strings.Contains(result, `"error"`) {
		status = "error"
	}

	receipt := &store.ExecutionReceipt{
		TenantID:  tctx.TenantID,
		SessionID: tctx.SessionID,
		UserID:    tctx.UserID,
		ToolName:  toolName,
		Input:     input,
		Output:    output,
		Status:    status,
	}

	if tctx.Extra != nil {
		if id, ok := tctx.Extra["idempotency_id"].(string); ok {
			receipt.IdempotencyID = id
		}
		if tid, ok := tctx.Extra["trace_id"].(string); ok {
			receipt.TraceID = tid
		}
	}

	if err := rr.store.Create(ctx, receipt); err != nil {
		slog.Warn("failed to record execution receipt", "tool", toolName, "error", err)
	}
}

// CheckIdempotency returns a prior receipt if one exists with the same idempotency_id.
func (rr *ReceiptRecorder) CheckIdempotency(ctx context.Context, tenantID, idempotencyID string) (*store.ExecutionReceipt, bool) {
	if rr == nil || rr.store == nil || idempotencyID == "" {
		return nil, false
	}
	receipt, err := rr.store.GetByIdempotencyID(ctx, tenantID, idempotencyID)
	if err != nil {
		return nil, false
	}
	return receipt, true
}

// DispatchWithReceipt executes a tool and records the execution receipt.
func DispatchWithReceipt(callCtx context.Context, r *ToolRegistry, recorder *ReceiptRecorder, name string, args map[string]any, tctx *ToolContext) string {
	if recorder == nil {
		return r.Dispatch(callCtx, name, args, tctx)
	}

	// Idempotency check: if this exact call was already executed, return cached result.
	if tctx != nil && tctx.Extra != nil {
		if idKey, ok := tctx.Extra["idempotency_id"].(string); ok && idKey != "" && tctx.TenantID != "" {
			if existing, found := recorder.CheckIdempotency(callCtx, tctx.TenantID, idKey); found {
				return existing.Output
			}
		}
	}

	start := time.Now()
	result := r.Dispatch(callCtx, name, args, tctx)
	duration := time.Since(start)
	durationMs := int(duration.Milliseconds())

	status := "success"
	if strings.Contains(result, `"error"`) {
		status = "error"
	}

	tenantID := ""
	if tctx != nil {
		tenantID = tctx.TenantID
	}
	observability.ToolExecutionDuration.WithLabelValues(name, status, tenantID).Observe(duration.Seconds())
	observability.ToolExecutionsTotal.WithLabelValues(name, status, tenantID).Inc()

	receipt := buildReceipt(name, args, tctx, result, durationMs)
	if receipt != nil {
		if err := recorder.store.Create(context.Background(), receipt); err != nil {
			slog.Warn("failed to record execution receipt", "tool", name, "error", err)
		}
	}

	return result
}

func buildReceipt(toolName string, args map[string]any, tctx *ToolContext, result string, durationMs int) *store.ExecutionReceipt {
	if tctx == nil || tctx.TenantID == "" {
		return nil
	}

	inputBytes, _ := json.Marshal(args)
	input := string(inputBytes)
	if len(input) > 4096 {
		input = input[:4096]
	}

	output := result
	if len(output) > 4096 {
		output = output[:4096]
	}

	status := "success"
	if strings.Contains(result, `"error"`) {
		status = "error"
	}

	receipt := &store.ExecutionReceipt{
		TenantID:   tctx.TenantID,
		SessionID:  tctx.SessionID,
		UserID:     tctx.UserID,
		ToolName:   toolName,
		Input:      input,
		Output:     output,
		Status:     status,
		DurationMs: durationMs,
	}

	if tctx.Extra != nil {
		if id, ok := tctx.Extra["idempotency_id"].(string); ok {
			receipt.IdempotencyID = id
		}
		if tid, ok := tctx.Extra["trace_id"].(string); ok {
			receipt.TraceID = tid
		}
	}

	return receipt
}
