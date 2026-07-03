// Package governance defines the client interface for hermesx L1 governance layer.
// This interface is consumed by superagent-base (L2) to interact with hermesx.
package governance

import (
	"context"
	"time"
)

// Client is the governance client interface for hermesx L1 layer.
// superagent-base (L2) implements an adapter that calls hermesx Admin API.
type Client interface {
	// GetTenant returns tenant information by ID.
	GetTenant(ctx context.Context, tenantID string) (*Tenant, error)

	// GetExecutionReceipts returns execution receipts for a session.
	GetExecutionReceipts(ctx context.Context, sessionID string) ([]*ExecutionReceipt, error)

	// GetTenantQuota returns the quota for a tenant.
	GetTenantQuota(ctx context.Context, tenantID string) (*Quota, error)

	// GetSandboxPolicy returns the sandbox policy for a tenant.
	GetSandboxPolicy(ctx context.Context, tenantID string) (*SandboxPolicy, error)

	// GetUsageSummary returns usage summary for a tenant within a time range.
	GetUsageSummary(ctx context.Context, tenantID string, from, to time.Time, granularity string) (*UsageSummary, error)
}

// Tenant represents a hermesx tenant.
type Tenant struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Plan         string    `json:"plan"`
	RateLimitRPM int       `json:"rate_limit_rpm"`
	MaxSessions  int       `json:"max_sessions"`
	CreatedAt    time.Time `json:"created_at"`
}

// ExecutionReceipt represents a tool execution record.
type ExecutionReceipt struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	SessionID     string    `json:"session_id"`
	UserID        string    `json:"user_id"`
	ToolName      string    `json:"tool_name"`
	Input         string    `json:"input"`
	Output        string    `json:"output"`
	Status        string    `json:"status"`
	DurationMs    int       `json:"duration_ms"`
	IdempotencyID string    `json:"idempotency_id,omitempty"`
	TraceID       string    `json:"trace_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Quota represents tenant quota limits.
type Quota struct {
	TenantID     string `json:"tenant_id"`
	MaxRPM       int    `json:"max_rpm"`
	MaxTokens    int    `json:"max_tokens"`
	MaxSessions  int    `json:"max_sessions"`
	MaxStorageMB int    `json:"max_storage_mb"`
}

// SandboxPolicy represents tenant sandbox configuration.
type SandboxPolicy struct {
	TenantID       string   `json:"tenant_id"`
	Mode           string   `json:"mode"` // "local", "docker", "k8s-job"
	AllowedImages  []string `json:"allowed_images,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

// UsageSummary represents aggregated usage for a tenant.
type UsageSummary struct {
	Date             string  `json:"date"`
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

// OrisMetricEvent represents a metric event to be reported to Oris (L3).
type OrisMetricEvent struct {
	AgentID      string    `json:"agent_id"`
	ToolName     string    `json:"tool_name"`
	SuccessRate  float64   `json:"success_rate"`
	AvgLatencyMs int64     `json:"avg_latency_ms"`
	ErrorTypes   []string  `json:"error_types,omitempty"`
	TraceID      string    `json:"trace_id,omitempty"`
	TenantID     string    `json:"tenant_id"`
	SessionID    string    `json:"session_id"`
	Timestamp    time.Time `json:"timestamp"`
}
