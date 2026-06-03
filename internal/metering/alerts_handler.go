package metering

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/middleware"
)

// AlertHandler provides HTTP endpoints for managing usage alert rules.
type AlertHandler struct {
	rules  AlertRuleStore
	events AlertEventStore
}

func NewAlertHandler(rules AlertRuleStore, events AlertEventStore) *AlertHandler {
	return &AlertHandler{rules: rules, events: events}
}

func (h *AlertHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.TenantFromContext(r.Context())
	if tenantID == "" {
		http.Error(w, "tenant context required", http.StatusBadRequest)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/usage-alerts")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "events" && r.Method == http.MethodGet:
		h.listEvents(w, r, tenantID)
	case path == "" && r.Method == http.MethodGet:
		h.listRules(w, r, tenantID)
	case path == "" && r.Method == http.MethodPost:
		h.createRule(w, r, tenantID)
	case path != "" && r.Method == http.MethodDelete:
		h.deleteRule(w, r, tenantID, path)
	case path != "" && r.Method == http.MethodPut:
		h.updateRule(w, r, tenantID, path)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *AlertHandler) listRules(w http.ResponseWriter, r *http.Request, tenantID string) {
	rules, err := h.rules.List(r.Context(), tenantID)
	if err != nil {
		http.Error(w, "failed to list rules", http.StatusInternalServerError)
		return
	}
	if rules == nil {
		rules = []*AlertRule{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}

func (h *AlertHandler) createRule(w http.ResponseWriter, r *http.Request, tenantID string) {
	var req struct {
		Metric    AlertMetric `json:"metric"`
		Threshold float64     `json:"threshold"`
		Window    string      `json:"window"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Metric == "" || req.Threshold <= 0 {
		http.Error(w, "metric and positive threshold required", http.StatusBadRequest)
		return
	}
	if req.Window == "" {
		req.Window = "daily"
	}
	if req.Window != "daily" && req.Window != "monthly" {
		http.Error(w, "window must be 'daily' or 'monthly'", http.StatusBadRequest)
		return
	}

	switch req.Metric {
	case MetricInputTokens, MetricOutputTokens, MetricTotalTokens, MetricCostUSD:
	default:
		http.Error(w, "invalid metric", http.StatusBadRequest)
		return
	}

	now := time.Now()
	rule := &AlertRule{
		ID:        "ar-" + now.Format("20060102150405"),
		TenantID:  tenantID,
		Metric:    req.Metric,
		Threshold: req.Threshold,
		Window:    req.Window,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.rules.Create(r.Context(), rule); err != nil {
		http.Error(w, "failed to create rule", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}

func (h *AlertHandler) updateRule(w http.ResponseWriter, r *http.Request, tenantID, ruleID string) {
	existing, err := h.rules.Get(r.Context(), tenantID, ruleID)
	if err != nil || existing == nil {
		http.Error(w, "rule not found", http.StatusNotFound)
		return
	}

	var req struct {
		Threshold *float64 `json:"threshold"`
		Enabled   *bool    `json:"enabled"`
		Window    *string  `json:"window"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Threshold != nil {
		if *req.Threshold <= 0 {
			http.Error(w, "threshold must be positive", http.StatusBadRequest)
			return
		}
		existing.Threshold = *req.Threshold
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Window != nil {
		if *req.Window != "daily" && *req.Window != "monthly" {
			http.Error(w, "window must be 'daily' or 'monthly'", http.StatusBadRequest)
			return
		}
		existing.Window = *req.Window
	}
	existing.UpdatedAt = time.Now()

	if err := h.rules.Update(r.Context(), existing); err != nil {
		http.Error(w, "failed to update rule", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}

func (h *AlertHandler) deleteRule(w http.ResponseWriter, r *http.Request, tenantID, ruleID string) {
	if err := h.rules.Delete(r.Context(), tenantID, ruleID); err != nil {
		http.Error(w, "failed to delete rule", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AlertHandler) listEvents(w http.ResponseWriter, r *http.Request, tenantID string) {
	if h.events == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{})
		return
	}
	events, err := h.events.ListByTenant(r.Context(), tenantID, 50)
	if err != nil {
		http.Error(w, "failed to list events", http.StatusInternalServerError)
		return
	}
	if events == nil {
		events = []*AlertEvent{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}
