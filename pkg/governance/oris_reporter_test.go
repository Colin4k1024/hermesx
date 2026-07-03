package governance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOrisReporter_Report(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/ingest/metrics" {
			t.Errorf("expected /api/v1/ingest/metrics, got %s", r.URL.Path)
		}

		var event OrisMetricEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Fatalf("failed to decode event: %v", err)
		}

		if event.ToolName != "test_tool" {
			t.Errorf("expected tool_name 'test_tool', got %s", event.ToolName)
		}
		if event.SuccessRate != 1.0 {
			t.Errorf("expected success_rate 1.0, got %f", event.SuccessRate)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reporter := NewOrisReporter(server.URL, nil)

	event := &OrisMetricEvent{
		AgentID:      "test-agent",
		ToolName:     "test_tool",
		SuccessRate:  1.0,
		AvgLatencyMs: 100,
		TenantID:     "test-tenant",
		SessionID:    "test-session",
		Timestamp:    time.Now(),
	}

	err := reporter.Report(context.Background(), event)
	if err != nil {
		t.Fatalf("Report failed: %v", err)
	}
}

func TestOrisReporter_ReportFromReceipt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event OrisMetricEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Fatalf("failed to decode event: %v", err)
		}

		if event.ToolName != "terminal" {
			t.Errorf("expected tool_name 'terminal', got %s", event.ToolName)
		}
		if event.SuccessRate != 1.0 {
			t.Errorf("expected success_rate 1.0, got %f", event.SuccessRate)
		}
		if event.AvgLatencyMs != 50 {
			t.Errorf("expected avg_latency_ms 50, got %d", event.AvgLatencyMs)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reporter := NewOrisReporter(server.URL, nil)

	receipt := &ExecutionReceipt{
		ID:         "test-id",
		TenantID:   "test-tenant",
		SessionID:  "test-session",
		ToolName:   "terminal",
		Status:     "success",
		DurationMs: 50,
		CreatedAt:  time.Now(),
	}

	err := reporter.ReportFromReceipt(context.Background(), receipt, "test-agent")
	if err != nil {
		t.Fatalf("ReportFromReceipt failed: %v", err)
	}
}

func TestOrisReporter_ReportFromReceipt_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event OrisMetricEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Fatalf("failed to decode event: %v", err)
		}

		if event.SuccessRate != 0.0 {
			t.Errorf("expected success_rate 0.0, got %f", event.SuccessRate)
		}
		if len(event.ErrorTypes) != 1 || event.ErrorTypes[0] != "error" {
			t.Errorf("expected error_types ['error'], got %v", event.ErrorTypes)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reporter := NewOrisReporter(server.URL, nil)

	receipt := &ExecutionReceipt{
		ID:         "test-id",
		TenantID:   "test-tenant",
		SessionID:  "test-session",
		ToolName:   "terminal",
		Status:     "error",
		DurationMs: 50,
		CreatedAt:  time.Now(),
	}

	err := reporter.ReportFromReceipt(context.Background(), receipt, "test-agent")
	if err != nil {
		t.Fatalf("ReportFromReceipt failed: %v", err)
	}
}

func TestOrisReporter_NilReporter(t *testing.T) {
	var reporter *OrisReporter

	err := reporter.Report(context.Background(), &OrisMetricEvent{})
	if err != nil {
		t.Fatalf("nil reporter should not return error: %v", err)
	}
}

func TestOrisReporter_EmptyEndpoint(t *testing.T) {
	reporter := &OrisReporter{endpoint: ""}

	err := reporter.Report(context.Background(), &OrisMetricEvent{})
	if err != nil {
		t.Fatalf("empty endpoint should not return error: %v", err)
	}
}
