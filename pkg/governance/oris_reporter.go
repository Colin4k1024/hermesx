package governance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// OrisReporter reports metric events to Oris (L3).
type OrisReporter struct {
	endpoint string
	client   *http.Client
	logger   *slog.Logger
}

// NewOrisReporter creates a new Oris metric reporter.
func NewOrisReporter(endpoint string, logger *slog.Logger) *OrisReporter {
	if logger == nil {
		logger = slog.Default()
	}
	return &OrisReporter{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 10 * time.Second},
		logger:   logger,
	}
}

// Report sends a metric event to Oris.
func (r *OrisReporter) Report(ctx context.Context, event *OrisMetricEvent) error {
	if r == nil || r.endpoint == "" {
		return nil
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal metric event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint+"/api/v1/ingest/metrics",
		bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.Warn("failed to report metric to oris", "error", err, "tool", event.ToolName)
		return fmt.Errorf("report to oris: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		r.logger.Warn("oris returned error", "status", resp.StatusCode, "tool", event.ToolName)
		return fmt.Errorf("oris returned status %d", resp.StatusCode)
	}

	return nil
}

// ReportFromReceipt creates and reports a metric event from an execution receipt.
func (r *OrisReporter) ReportFromReceipt(ctx context.Context, receipt *ExecutionReceipt, agentID string) error {
	if receipt == nil {
		return nil
	}

	event := &OrisMetricEvent{
		AgentID:      agentID,
		ToolName:     receipt.ToolName,
		SuccessRate:  1.0,
		AvgLatencyMs: int64(receipt.DurationMs),
		TraceID:      receipt.TraceID,
		TenantID:     receipt.TenantID,
		SessionID:    receipt.SessionID,
		Timestamp:    receipt.CreatedAt,
	}

	if receipt.Status != "success" {
		event.SuccessRate = 0.0
		event.ErrorTypes = []string{receipt.Status}
	}

	return r.Report(ctx, event)
}
