package metering

import (
	"context"
	"time"
)

// UsageRecord represents a single LLM usage event.
type UsageRecord struct {
	TenantID         string
	SessionID        string
	UserID           string
	Model            string
	Provider         string
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	CostUSD          float64
	Degraded         bool
	CreatedAt        time.Time
}

// UsageSummary represents aggregated usage for a time bucket.
type UsageSummary struct {
	Date         string  `json:"date"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// TenantUsageSummary represents cross-tenant aggregate usage for one tenant.
type TenantUsageSummary struct {
	TenantID     string  `json:"tenant_id"`
	SessionCount int64   `json:"session_count"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// TenantUsageQuery controls platform-level tenant usage aggregation.
type TenantUsageQuery struct {
	From   *time.Time
	To     *time.Time
	Limit  int
	Offset int
}

// UsageStore defines the persistence interface for usage records.
type UsageStore interface {
	BatchInsert(ctx context.Context, records []UsageRecord) error
	QueryByTenant(ctx context.Context, tenantID string, from, to time.Time, granularity string) ([]UsageSummary, error)
	QueryBySession(ctx context.Context, tenantID, sessionID string) ([]UsageRecord, error)
}

// TenantUsageAggregator is an optional UsageStore extension for controlled
// platform-level aggregate reads. Implementations must return aggregate-only
// data and must not rely on disabling tenant isolation in request handlers.
type TenantUsageAggregator interface {
	QueryTenants(ctx context.Context, q TenantUsageQuery) ([]TenantUsageSummary, error)
}
