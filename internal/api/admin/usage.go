package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
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

// tenantUsageQuery aggregates session-level token and cost metrics per tenant.
// It runs inside a transaction with row_security = off so it reads across all
// tenants regardless of the RLS policies on the sessions table.
// The application DB user must be SUPERUSER or hold BYPASSRLS to enable this.
const tenantUsageQuery = `
SELECT
    tenant_id::text,
    COUNT(*)                                AS session_count,
    COALESCE(SUM(input_tokens),  0)::bigint AS input_tokens,
    COALESCE(SUM(output_tokens), 0)::bigint AS output_tokens,
    COALESCE(SUM(estimated_cost_usd), 0)    AS estimated_cost_usd
FROM sessions
WHERE ($1::timestamptz IS NULL OR started_at >= $1)
  AND ($2::timestamptz IS NULL OR started_at <  $2)
GROUP BY tenant_id
ORDER BY estimated_cost_usd DESC
LIMIT $3 OFFSET $4`

// listTenantUsage serves GET /admin/v1/usage/tenants.
//
// Query params:
//
//	from   RFC3339 lower bound on session start time (inclusive, optional)
//	to     RFC3339 upper bound on session start time (exclusive, optional)
//	limit  max rows (default 100, max 500)
//	offset pagination offset (default 0)
func (h *AdminHandler) listTenantUsage(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		http.Error(w, "database pool not available", http.StatusServiceUnavailable)
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

	ctx := r.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		h.logger.Error("admin tenant-usage: begin tx", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Bypass RLS so the aggregate spans all tenants.
	// Requires the application DB role to hold BYPASSRLS or SUPERUSER.
	if _, err := tx.Exec(ctx, "SET LOCAL row_security = off"); err != nil {
		h.logger.Error("admin tenant-usage: set row_security", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows, err := tx.Query(ctx, tenantUsageQuery, from, to, limit, offset)
	if err != nil {
		h.logger.Error("admin tenant-usage: query", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	summaries := make([]TenantUsageSummary, 0)
	for rows.Next() {
		var s TenantUsageSummary
		if err := rows.Scan(
			&s.TenantID, &s.SessionCount,
			&s.InputTokens, &s.OutputTokens,
			&s.EstimatedCostUSD,
		); err != nil {
			h.logger.Error("admin tenant-usage: scan", "error", err)
			continue
		}
		s.TotalTokens = s.InputTokens + s.OutputTokens
		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		h.logger.Error("admin tenant-usage: rows error", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Commit is a no-op for a read-only tx but needed to release the connection cleanly.
	_ = tx.Commit(ctx)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tenants": summaries,
		"limit":   limit,
		"offset":  offset,
	})
}
