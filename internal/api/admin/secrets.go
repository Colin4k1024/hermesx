package admin

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/Colin4k1024/hermesx/internal/secrets"
)

type secretPatternRequest struct {
	Name     string `json:"name"`
	Pattern  string `json:"pattern"`
	Severity string `json:"severity"`
}

type secretPatternResponse struct {
	Name     string `json:"name"`
	Pattern  string `json:"pattern"`
	Severity string `json:"severity"`
}

func (h *AdminHandler) listSecretPatterns(w http.ResponseWriter, r *http.Request) {
	if h.leakScanner == nil {
		http.Error(w, `{"error":"leak scanner not configured"}`, http.StatusServiceUnavailable)
		return
	}

	patterns := h.leakScanner.Patterns()
	resp := make([]secretPatternResponse, 0, len(patterns))
	for _, p := range patterns {
		resp = append(resp, secretPatternResponse{
			Name:     p.Name,
			Pattern:  p.Pattern.String(),
			Severity: string(p.Severity),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *AdminHandler) createSecretPattern(w http.ResponseWriter, r *http.Request) {
	if h.leakScanner == nil {
		http.Error(w, `{"error":"leak scanner not configured"}`, http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // M-7: 64 KB limit
	var req secretPatternRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Pattern == "" {
		http.Error(w, `{"error":"name and pattern are required"}`, http.StatusBadRequest)
		return
	}

	compiled, err := regexp.Compile(req.Pattern)
	if err != nil {
		http.Error(w, `{"error":"invalid regex pattern"}`, http.StatusBadRequest)
		return
	}

	severity := secrets.Severity(req.Severity)
	switch severity {
	case secrets.SeverityCritical, secrets.SeverityHigh, secrets.SeverityMedium, secrets.SeverityLow:
	default:
		severity = secrets.SeverityHigh
	}

	h.leakScanner.AddPattern(req.Name, compiled, severity)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(secretPatternResponse{
		Name:     req.Name,
		Pattern:  compiled.String(),
		Severity: string(severity),
	})
}

// GET /admin/v1/secrets/canary-tokens — list all active canary tokens.
func (h *AdminHandler) listCanaryTokens(w http.ResponseWriter, r *http.Request) {
	if h.canaryDetector == nil {
		http.Error(w, `{"error":"canary detector not configured"}`, http.StatusServiceUnavailable)
		return
	}

	tokens := h.canaryDetector.ListTokens()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

// DELETE /admin/v1/secrets/canary-tokens/{id} — revoke an active canary token.
func (h *AdminHandler) deleteCanaryToken(w http.ResponseWriter, r *http.Request) {
	if h.canaryDetector == nil {
		http.Error(w, `{"error":"canary detector not configured"}`, http.StatusServiceUnavailable)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"token id is required"}`, http.StatusBadRequest)
		return
	}

	// B-2: accept opaque handle, never the raw token value.
	h.canaryDetector.RemoveTokenByID(id)
	w.WriteHeader(http.StatusNoContent)
}
