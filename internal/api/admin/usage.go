package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Colin4k1024/hermesx/internal/metering"
)

// TenantUsageSummary holds aggregated token and cost metrics for one tenant.
type TenantUsageSummary struct {
	TenantID         string  `json:"tenant_id"`
	SessionCount     int64   `json:"session_count"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

// listTenantUsage serves GET /admin/v1/usage/tenants.
//
// Query params:
//
//	from   RFC3339 lower bound on session start time (inclusive, optional)
//	to     RFC3339 upper bound on session start time (exclusive, optional)
//	limit  max rows (default 100, max 500)
//	offset pagination offset (default 0)
func (h *AdminHandler) listTenantUsage(w http.ResponseWriter, r *http.Request) {
	aggregator, ok := h.usageStore.(metering.TenantUsageAggregator)
	if h.usageStore == nil || !ok {
		http.Error(w, "tenant usage aggregator not configured", http.StatusServiceUnavailable)
		return
	}

	q := r.URL.Query()

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var from, to *time.Time
	if s := q.Get("from"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			http.Error(w, "invalid 'from': use RFC3339", http.StatusBadRequest)
			return
		}
		from = &t
	}
	if s := q.Get("to"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			http.Error(w, "invalid 'to': use RFC3339", http.StatusBadRequest)
			return
		}
		to = &t
	}

	rows, err := aggregator.QueryTenants(r.Context(), metering.TenantUsageQuery{
		From:   from,
		To:     to,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		h.logger.Error("admin tenant-usage: query", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	summaries := make([]TenantUsageSummary, 0)
	for _, row := range rows {
		summaries = append(summaries, TenantUsageSummary{
			TenantID:         row.TenantID,
			SessionCount:     row.SessionCount,
			InputTokens:      row.InputTokens,
			OutputTokens:     row.OutputTokens,
			TotalTokens:      row.InputTokens + row.OutputTokens,
			EstimatedCostUSD: row.CostUSD,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tenants": summaries,
		"limit":   limit,
		"offset":  offset,
	})
}
