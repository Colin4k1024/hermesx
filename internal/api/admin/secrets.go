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

var globalLeakScanner *secrets.LeakScanner

func SetLeakScanner(scanner *secrets.LeakScanner) {
	globalLeakScanner = scanner
}

func (h *AdminHandler) listSecretPatterns(w http.ResponseWriter, r *http.Request) {
	if globalLeakScanner == nil {
		http.Error(w, `{"error":"leak scanner not configured"}`, http.StatusServiceUnavailable)
		return
	}

	patterns := globalLeakScanner.Patterns()
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
	if globalLeakScanner == nil {
		http.Error(w, `{"error":"leak scanner not configured"}`, http.StatusServiceUnavailable)
		return
	}

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

	globalLeakScanner.AddPattern(req.Name, compiled, severity)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(secretPatternResponse{
		Name:     req.Name,
		Pattern:  compiled.String(),
		Severity: string(severity),
	})
}
