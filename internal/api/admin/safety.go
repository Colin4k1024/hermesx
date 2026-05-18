package admin

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/Colin4k1024/hermesx/internal/safety"
)

// ── request/response types ────────────────────────────────────────────────────

type inputPatternResponse struct {
	Text     string `json:"text,omitempty"`
	Regex    string `json:"regex,omitempty"`
	Severity int    `json:"severity"`
}

type outputRuleResponse struct {
	Description string `json:"description"`
	Contains    string `json:"contains,omitempty"`
	Regex       string `json:"regex,omitempty"`
	Severity    int    `json:"severity"`
}

type policyResponse struct {
	ID            string                 `json:"id,omitempty"`
	TenantID      string                 `json:"tenant_id"`
	Mode          string                 `json:"mode"`
	InputPatterns []inputPatternResponse `json:"input_patterns"`
	OutputRules   []outputRuleResponse   `json:"output_rules"`
}

type updatePolicyRequest struct {
	Mode          string `json:"mode"`
	InputPatterns []struct {
		Text     string `json:"text,omitempty"`
		Regex    string `json:"regex,omitempty"`
		Severity int    `json:"severity"`
	} `json:"input_patterns,omitempty"`
	OutputRules []struct {
		Description string `json:"description"`
		Contains    string `json:"contains,omitempty"`
		Regex       string `json:"regex,omitempty"`
		Severity    int    `json:"severity"`
	} `json:"output_rules,omitempty"`
}

type scanRequest struct {
	TenantID string `json:"tenant_id"`
	Input    string `json:"input"`
}

type scanResponse struct {
	TenantID string `json:"tenant_id"`
	Blocked  bool   `json:"blocked"`
	Reason   string `json:"reason,omitempty"`
}

// ── helpers ───────────────────────────────────────────────────────────────────

func policyToResponse(p safety.Policy) policyResponse {
	resp := policyResponse{
		ID:            p.ID,
		TenantID:      p.TenantID,
		Mode:          string(p.Mode),
		InputPatterns: make([]inputPatternResponse, 0, len(p.InputPatterns)),
		OutputRules:   make([]outputRuleResponse, 0, len(p.OutputRules)),
	}
	for _, ip := range p.InputPatterns {
		r := inputPatternResponse{Text: ip.Text, Severity: ip.Severity}
		if ip.Regex != nil {
			r.Regex = ip.Regex.String()
		}
		resp.InputPatterns = append(resp.InputPatterns, r)
	}
	for _, or_ := range p.OutputRules {
		r := outputRuleResponse{Description: or_.Description, Contains: or_.Contains, Severity: or_.Severity}
		if or_.Regex != nil {
			r.Regex = or_.Regex.String()
		}
		resp.OutputRules = append(resp.OutputRules, r)
	}
	return resp
}

// ── handlers ─────────────────────────────────────────────────────────────────

// GET /admin/v1/safety/rules — list all safety policies.
func (h *AdminHandler) listSafetyRules(w http.ResponseWriter, r *http.Request) {
	if h.policyStore == nil {
		jsonError(w, "safety policy store not configured", http.StatusServiceUnavailable)
		return
	}

	policies, err := h.policyStore.ListPolicies(r.Context())
	if err != nil {
		h.logger.Error("listSafetyRules: list failed", "err", err)
		jsonError(w, "failed to list safety policies", http.StatusInternalServerError)
		return
	}

	resp := make([]policyResponse, 0, len(policies))
	for _, p := range policies {
		resp = append(resp, policyToResponse(p))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// PUT /admin/v1/safety/rules/{id} — update (upsert) a safety policy by tenant ID.
func (h *AdminHandler) updateSafetyRule(w http.ResponseWriter, r *http.Request) {
	if h.policyStore == nil {
		jsonError(w, "safety policy store not configured", http.StatusServiceUnavailable)
		return
	}

	tenantID := r.PathValue("id")
	if tenantID == "" {
		jsonError(w, "tenant id is required", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // M-7: 64 KB limit
	var req updatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate mode.
	mode := safety.PolicyMode(req.Mode)
	switch mode {
	case safety.ModeEnforce, safety.ModeLogOnly, safety.ModeDisabled:
	default:
		jsonError(w, "mode must be one of: enforce, log_only, disabled", http.StatusBadRequest)
		return
	}

	// Build the updated policy.
	policy := safety.Policy{
		TenantID:      tenantID,
		Mode:          mode,
		InputPatterns: make([]safety.InputPattern, 0, len(req.InputPatterns)),
		OutputRules:   make([]safety.OutputRule, 0, len(req.OutputRules)),
	}

	for _, ip := range req.InputPatterns {
		p := safety.InputPattern{Text: ip.Text, Severity: ip.Severity}
		if ip.Regex != "" {
			compiled, err := regexp.Compile(ip.Regex)
			if err != nil {
				jsonError(w, "invalid input_pattern regex: "+ip.Regex, http.StatusBadRequest)
				return
			}
			p.Regex = compiled
		}
		policy.InputPatterns = append(policy.InputPatterns, p)
	}

	for _, or_ := range req.OutputRules {
		rule := safety.OutputRule{
			Description: or_.Description,
			Contains:    or_.Contains,
			Severity:    or_.Severity,
		}
		if or_.Regex != "" {
			compiled, err := regexp.Compile(or_.Regex)
			if err != nil {
				jsonError(w, "invalid output_rule regex: "+or_.Regex, http.StatusBadRequest)
				return
			}
			rule.Regex = compiled
		}
		policy.OutputRules = append(policy.OutputRules, rule)
	}

	if err := h.policyStore.UpsertPolicy(r.Context(), &policy); err != nil {
		h.logger.Error("updateSafetyRule: upsert failed", "tenant_id", tenantID, "err", err)
		jsonError(w, "failed to update safety policy", http.StatusInternalServerError)
		return
	}

	// Return the updated policy (re-fetch to get server-assigned ID).
	updated, err := h.policyStore.GetPolicy(r.Context(), tenantID)
	if err != nil || updated == nil {
		// Upsert succeeded; return what we submitted.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(policyToResponse(policy))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(policyToResponse(*updated))
}

// POST /admin/v1/safety/scan — manually trigger a safety scan (debug/test use).
func (h *AdminHandler) safetyManualScan(w http.ResponseWriter, r *http.Request) {
	if h.policyStore == nil {
		jsonError(w, "safety policy store not configured", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // M-7: 64 KB limit
	var req scanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.TenantID == "" || req.Input == "" {
		jsonError(w, "tenant_id and input are required", http.StatusBadRequest)
		return
	}

	policy, err := h.policyStore.GetPolicy(r.Context(), req.TenantID)
	if err != nil || policy == nil {
		def := safety.DefaultPolicy()
		policy = &def
	}

	blocked := false
	reason := ""
	for _, ip := range policy.InputPatterns {
		if ip.Text != "" && len(req.Input) > 0 {
			// Simple contains check for text patterns.
			if containsString(req.Input, ip.Text) {
				blocked = policy.Mode == safety.ModeEnforce
				reason = "matched input pattern: " + ip.Text
				break
			}
		}
		if ip.Regex != nil && ip.Regex.MatchString(req.Input) {
			blocked = policy.Mode == safety.ModeEnforce
			reason = "matched input regex: " + ip.Regex.String()
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scanResponse{
		TenantID: req.TenantID,
		Blocked:  blocked,
		Reason:   reason,
	})
}

// containsString is a simple case-sensitive substring check.
func containsString(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		func() bool {
			for i := 0; i <= len(haystack)-len(needle); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		}()
}

// jsonError writes a JSON error body with the given HTTP status.
func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
