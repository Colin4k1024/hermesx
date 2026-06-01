package safety

import (
	"context"
	"log/slog"
)

type SafetyAction int

const (
	ActionAllow SafetyAction = iota
	ActionLog
	ActionBlock
	ActionMask
)

func (a SafetyAction) String() string {
	switch a {
	case ActionAllow:
		return "allow"
	case ActionLog:
		return "log"
	case ActionBlock:
		return "block"
	case ActionMask:
		return "mask"
	default:
		return "unknown"
	}
}

type PatternMatch struct {
	Category string
	Pattern  string
	Match    string
	Severity int
}

type SafetyResult struct {
	Allowed bool
	Reason  string
	Action  SafetyAction
	Matches []PatternMatch
}

type Message struct {
	Role    string
	Content string
}

type SafetyInterceptor interface {
	CheckInput(ctx context.Context, tenantID string, messages []Message) (*SafetyResult, error)
	CheckOutput(ctx context.Context, tenantID string, output string) (*SafetyResult, error)
	// IsModeEnforce reports whether the tenant's active policy is ModeEnforce.
	// Used by the agent loop to decide fail-closed vs. fail-open on timeout.
	IsModeEnforce(ctx context.Context, tenantID string) bool
}

type InterceptorChain struct {
	inputGuard  *InputGuard
	outputGuard *OutputGuard
	canary      *CanaryDetector
	policyStore PolicyStore
}

func NewInterceptorChain(policyStore PolicyStore) *InterceptorChain {
	return NewInterceptorChainWithCanary(policyStore, nil)
}

// NewInterceptorChainWithCanary wires a caller-owned detector into the safety
// chain so admin token management and runtime output checks share one source of
// truth across the process.
func NewInterceptorChainWithCanary(policyStore PolicyStore, canary *CanaryDetector) *InterceptorChain {
	if canary == nil {
		canary = NewCanaryDetector()
	}
	return &InterceptorChain{
		inputGuard:  NewInputGuard(),
		outputGuard: NewOutputGuard(),
		canary:      canary,
		policyStore: policyStore,
	}
}

func (ic *InterceptorChain) CheckInput(ctx context.Context, tenantID string, messages []Message) (*SafetyResult, error) {
	policy, err := ic.resolvePolicy(ctx, tenantID)
	if err != nil {
		slog.Warn("failed to resolve safety policy, using default", "tenant_id", tenantID, "error", err)
		p := DefaultPolicy()
		policy = &p
	}

	if policy.Mode == ModeDisabled {
		return &SafetyResult{Allowed: true, Action: ActionAllow}, nil
	}

	var allMatches []PatternMatch
	for _, msg := range messages {
		if msg.Role == "system" {
			continue
		}
		matches := ic.inputGuard.Scan(msg.Content, policy.InputPatterns)
		allMatches = append(allMatches, matches...)
	}

	if len(allMatches) == 0 {
		return &SafetyResult{Allowed: true, Action: ActionAllow}, nil
	}

	result := &SafetyResult{
		Matches: allMatches,
		Reason:  "prompt injection pattern detected",
	}

	switch policy.Mode {
	case ModeEnforce:
		result.Allowed = false
		result.Action = ActionBlock
	case ModeLogOnly:
		result.Allowed = true
		result.Action = ActionLog
		slog.Warn("safety: input injection detected (log_only)",
			"tenant_id", tenantID,
			"match_count", len(allMatches),
			"first_match", allMatches[0].Pattern,
		)
	default:
		result.Allowed = true
		result.Action = ActionLog
	}

	return result, nil
}

func (ic *InterceptorChain) CheckOutput(ctx context.Context, tenantID string, output string) (*SafetyResult, error) {
	policy, err := ic.resolvePolicy(ctx, tenantID)
	if err != nil {
		slog.Warn("failed to resolve safety policy, using default", "tenant_id", tenantID, "error", err)
		p := DefaultPolicy()
		policy = &p
	}

	if policy.Mode == ModeDisabled {
		return &SafetyResult{Allowed: true, Action: ActionAllow}, nil
	}

	var allMatches []PatternMatch

	outputMatches := ic.outputGuard.Scan(output, policy.OutputRules)
	allMatches = append(allMatches, outputMatches...)

	canaryMatches := ic.canary.Detect(output)
	allMatches = append(allMatches, canaryMatches...)

	if len(allMatches) == 0 {
		return &SafetyResult{Allowed: true, Action: ActionAllow}, nil
	}

	result := &SafetyResult{
		Matches: allMatches,
		Reason:  "output policy violation detected",
	}

	switch policy.Mode {
	case ModeEnforce:
		result.Allowed = false
		result.Action = ActionBlock
	case ModeLogOnly:
		result.Allowed = true
		result.Action = ActionLog
		slog.Warn("safety: output violation detected (log_only)",
			"tenant_id", tenantID,
			"match_count", len(allMatches),
		)
	default:
		result.Allowed = true
		result.Action = ActionLog
	}

	return result, nil
}

func (ic *InterceptorChain) Canary() *CanaryDetector {
	return ic.canary
}

// IsModeEnforce returns true when the tenant's active policy is ModeEnforce.
func (ic *InterceptorChain) IsModeEnforce(ctx context.Context, tenantID string) bool {
	policy, err := ic.resolvePolicy(ctx, tenantID)
	if err != nil || policy == nil {
		return false
	}
	return policy.Mode == ModeEnforce
}

func (ic *InterceptorChain) resolvePolicy(ctx context.Context, tenantID string) (*Policy, error) {
	if ic.policyStore == nil {
		p := DefaultPolicy()
		return &p, nil
	}
	pol, err := ic.policyStore.GetPolicy(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if pol == nil {
		p := DefaultPolicy()
		return &p, nil
	}
	return pol, nil
}

// ScanPoint labels an additional pipeline stage that should be safety-checked.
// It is used only for log annotation and does not affect the action taken.
type ScanPoint string

const (
	ScanPointToolOutput    ScanPoint = "tool_output"
	ScanPointSkillContent  ScanPoint = "skill_content"
	ScanPointMemoryContent ScanPoint = "memory_content"
)

// scanContent is the shared implementation for additional scan points.
// It applies the output guard and canary detector, honours the tenant policy,
// and labels every log entry with the supplied scanPoint.
func (ic *InterceptorChain) scanContent(ctx context.Context, tenantID string, point ScanPoint, content string) (*SafetyResult, error) {
	policy, err := ic.resolvePolicy(ctx, tenantID)
	if err != nil {
		slog.Warn("safety: failed to resolve policy, using default",
			"tenant_id", tenantID, "scan_point", point, "error", err)
		p := DefaultPolicy()
		policy = &p
	}

	if policy.Mode == ModeDisabled {
		return &SafetyResult{Allowed: true, Action: ActionAllow}, nil
	}

	var allMatches []PatternMatch

	outputMatches := ic.outputGuard.Scan(content, policy.OutputRules)
	allMatches = append(allMatches, outputMatches...)

	canaryMatches := ic.canary.Detect(content)
	allMatches = append(allMatches, canaryMatches...)

	if len(allMatches) == 0 {
		return &SafetyResult{Allowed: true, Action: ActionAllow}, nil
	}

	result := &SafetyResult{
		Matches: allMatches,
		Reason:  string(point) + " safety violation detected",
	}

	switch policy.Mode {
	case ModeEnforce:
		result.Allowed = false
		result.Action = ActionBlock
	case ModeLogOnly:
		result.Allowed = true
		result.Action = ActionLog
		slog.Warn("safety: scan point violation (log_only)",
			"tenant_id", tenantID,
			"scan_point", point,
			"match_count", len(allMatches),
		)
	default:
		result.Allowed = true
		result.Action = ActionLog
	}

	return result, nil
}

// ScanToolOutput checks the content returned by a tool invocation for safety
// violations (prompt leakage, indirect injection, canary tokens). Used by the
// agent loop after every tool call.
func (ic *InterceptorChain) ScanToolOutput(ctx context.Context, tenantID, toolName, content string) (*SafetyResult, error) {
	_ = toolName // reserved for future per-tool policy lookups
	return ic.scanContent(ctx, tenantID, ScanPointToolOutput, content)
}

// ScanSkillContent checks a skill's SKILL.md body before it is installed into
// a tenant namespace. Malicious skills could embed indirect injection payloads.
func (ic *InterceptorChain) ScanSkillContent(ctx context.Context, tenantID, skillName, content string) (*SafetyResult, error) {
	_ = skillName // reserved for audit log enrichment
	return ic.scanContent(ctx, tenantID, ScanPointSkillContent, content)
}

// ScanMemoryContent checks recalled memory fragments before they are injected
// into the context window. Memory can be a vector for stored injection attacks.
func (ic *InterceptorChain) ScanMemoryContent(ctx context.Context, tenantID, content string) (*SafetyResult, error) {
	return ic.scanContent(ctx, tenantID, ScanPointMemoryContent, content)
}
