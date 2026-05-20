package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Colin4k1024/hermesx/internal/metering"
)

// usageAggregationItem represents a single time-bucket in the admin usage response.
type usageAggregationItem struct {
	Date             string  `json:"date"`
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

// adminUsageAggregation serves GET /admin/v1/usage.
//
// Query params:
//
//	tenant_id    UUID of the tenant (required)
//	granularity  "daily" or "monthly" (default: "daily")
//	from         date string YYYY-MM-DD or RFC3339 (default: 30 days ago)
//	to           date string YYYY-MM-DD or RFC3339 (default: now)
func (h *AdminHandler) adminUsageAggregation(w http.ResponseWriter, r *http.Request) {
	if h.usageStore == nil {
		http.Error(w, "usage store not configured", http.StatusServiceUnavailable)
		return
	}

	q := r.URL.Query()

	tenantID := q.Get("tenant_id")
	if tenantID == "" {
		http.Error(w, "tenant_id query parameter is required", http.StatusBadRequest)
		return
	}

	granularity := q.Get("granularity")
	if granularity == "" {
		granularity = "daily"
	}
	// Normalize granularity to what the metering store expects.
	switch granularity {
	case "daily", "day":
		granularity = "day"
	case "monthly", "month":
		granularity = "month"
	default:
		http.Error(w, "granularity must be 'daily' or 'monthly'", http.StatusBadRequest)
		return
	}

	from, err := parseAdminTimeParam(q.Get("from"))
	if err != nil {
		from = time.Now().AddDate(0, -1, 0) // default: last 30 days
	}

	to, err := parseAdminTimeParam(q.Get("to"))
	if err != nil {
		to = time.Now()
	}

	summaries, err := h.usageStore.QueryByTenant(r.Context(), tenantID, from, to, granularity)
	if err != nil {
		h.logger.Error("admin usage aggregation query failed", "tenant_id", tenantID, "error", err)
		http.Error(w, "failed to query usage", http.StatusInternalServerError)
		return
	}
	if summaries == nil {
		summaries = []metering.UsageSummary{}
	}

	// Convert to response format with total_tokens and estimated_cost_usd.
	items := make([]usageAggregationItem, 0, len(summaries))
	for _, s := range summaries {
		items = append(items, usageAggregationItem{
			Date:             s.Date,
			InputTokens:      s.InputTokens,
			OutputTokens:     s.OutputTokens,
			TotalTokens:      s.InputTokens + s.OutputTokens,
			EstimatedCostUSD: s.CostUSD,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": items,
	})
}

// parseAdminTimeParam parses a time string in RFC3339 or YYYY-MM-DD format.
func parseAdminTimeParam(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time param")
	}
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse("2006-01-02", s)
	if err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid time format: %s", s)
}
