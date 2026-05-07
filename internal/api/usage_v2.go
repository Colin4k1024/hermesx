package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Colin4k1024/hermesx/internal/metering"
	"github.com/Colin4k1024/hermesx/internal/middleware"
)

// UsageV2Handler serves the enhanced usage API endpoints.
type UsageV2Handler struct {
	store metering.UsageStore
}

// NewUsageV2Handler creates a usage v2 handler.
func NewUsageV2Handler(store metering.UsageStore) *UsageV2Handler {
	return &UsageV2Handler{store: store}
}

// ServeHTTP routes to the appropriate handler based on path.
func (h *UsageV2Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tenantID := middleware.TenantFromContext(r.Context())
	if tenantID == "" {
		http.Error(w, "tenant context required", http.StatusBadRequest)
		return
	}

	// Route: /v1/usage/details?session_id=...
	if r.URL.Path == "/v1/usage/details" {
		h.handleDetails(w, r, tenantID)
		return
	}

	// Default: /v1/usage?from=&to=&granularity=
	h.handleSummary(w, r, tenantID)
}

func (h *UsageV2Handler) handleSummary(w http.ResponseWriter, r *http.Request, tenantID string) {
	q := r.URL.Query()

	granularity := q.Get("granularity")
	if granularity == "" {
		granularity = "day"
	}

	from, err := parseTimeParam(q.Get("from"))
	if err != nil {
		from = time.Now().AddDate(0, -1, 0) // default: last 30 days
	}

	to, err := parseTimeParam(q.Get("to"))
	if err != nil {
		to = time.Now()
	}

	data, err := h.store.QueryByTenant(r.Context(), tenantID, from, to, granularity)
	if err != nil {
		http.Error(w, "failed to query usage", http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = []metering.UsageSummary{}
	}

	resp := map[string]any{
		"tenant_id":   tenantID,
		"period":      map[string]string{"from": from.Format(time.RFC3339), "to": to.Format(time.RFC3339)},
		"granularity": granularity,
		"data":        data,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *UsageV2Handler) handleDetails(w http.ResponseWriter, r *http.Request, tenantID string) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "session_id query parameter required", http.StatusBadRequest)
		return
	}

	records, err := h.store.QueryBySession(r.Context(), tenantID, sessionID)
	if err != nil {
		http.Error(w, "failed to query usage details", http.StatusInternalServerError)
		return
	}
	if records == nil {
		records = []metering.UsageRecord{}
	}

	// Convert to response format
	type recordResponse struct {
		Model            string  `json:"model"`
		Provider         string  `json:"provider"`
		InputTokens      int     `json:"input_tokens"`
		OutputTokens     int     `json:"output_tokens"`
		CacheReadTokens  int     `json:"cache_read_tokens"`
		CacheWriteTokens int     `json:"cache_write_tokens"`
		CostUSD          float64 `json:"cost_usd"`
		Degraded         bool    `json:"degraded"`
		CreatedAt        string  `json:"created_at"`
	}

	items := make([]recordResponse, 0, len(records))
	for _, rec := range records {
		items = append(items, recordResponse{
			Model:            rec.Model,
			Provider:         rec.Provider,
			InputTokens:      rec.InputTokens,
			OutputTokens:     rec.OutputTokens,
			CacheReadTokens:  rec.CacheReadTokens,
			CacheWriteTokens: rec.CacheWriteTokens,
			CostUSD:          rec.CostUSD,
			Degraded:         rec.Degraded,
			CreatedAt:        rec.CreatedAt.Format(time.RFC3339),
		})
	}

	resp := map[string]any{
		"tenant_id":  tenantID,
		"session_id": sessionID,
		"records":    items,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func parseTimeParam(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time param")
	}
	// Try RFC3339 first
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}
	// Try date-only format
	t, err = time.Parse("2006-01-02", s)
	if err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid time format: %s", s)
}
