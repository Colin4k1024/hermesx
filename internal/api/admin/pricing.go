package admin

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"regexp"

	"github.com/hermes-agent/hermes-agent-go/internal/store"
)

var validModelKey = regexp.MustCompile(`^[a-zA-Z0-9._:/-]{1,128}$`)

func (h *AdminHandler) listPricingRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.store.PricingRules().List(r.Context())
	if err != nil {
		h.logger.Error("list pricing rules", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if rules == nil {
		rules = []store.PricingRule{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}

func (h *AdminHandler) upsertPricingRule(w http.ResponseWriter, r *http.Request) {
	modelKey := r.PathValue("model")
	if modelKey == "" {
		http.Error(w, "model path parameter required", http.StatusBadRequest)
		return
	}
	if !validModelKey.MatchString(modelKey) {
		http.Error(w, "invalid model key: must be 1-128 alphanumeric chars with . _ : / -", http.StatusBadRequest)
		return
	}

	var body struct {
		InputPer1K     float64 `json:"input_per_1k"`
		OutputPer1K    float64 `json:"output_per_1k"`
		CacheReadPer1K float64 `json:"cache_read_per_1k"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if !isValidPrice(body.InputPer1K) || !isValidPrice(body.OutputPer1K) || !isValidPrice(body.CacheReadPer1K) {
		http.Error(w, "pricing values must be non-negative finite numbers", http.StatusBadRequest)
		return
	}

	rule := &store.PricingRule{
		ModelKey:       modelKey,
		InputPer1K:     body.InputPer1K,
		OutputPer1K:    body.OutputPer1K,
		CacheReadPer1K: body.CacheReadPer1K,
	}
	if err := h.store.PricingRules().Upsert(r.Context(), rule); err != nil {
		h.logger.Error("upsert pricing rule", "model", modelKey, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Invalidate cache if a PricingStore is wired.
	if h.pricingCache != nil {
		h.pricingCache.Invalidate()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "model_key": modelKey})
}

func (h *AdminHandler) deletePricingRule(w http.ResponseWriter, r *http.Request) {
	modelKey := r.PathValue("model")
	if modelKey == "" {
		http.Error(w, "model path parameter required", http.StatusBadRequest)
		return
	}

	if err := h.store.PricingRules().Delete(r.Context(), modelKey); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		h.logger.Error("delete pricing rule", "model", modelKey, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if h.pricingCache != nil {
		h.pricingCache.Invalidate()
	}

	w.WriteHeader(http.StatusNoContent)
}

func isValidPrice(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0
}
