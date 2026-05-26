package egress

import (
	"context"
	"strings"
	"sync"
)

var builtinAllowlist = []string{
	"api.openai.com",
	"api.anthropic.com",
}

type RuleStore interface {
	LoadRules(ctx context.Context, tenantID string) ([]EgressRule, error)
}

type RuleCache interface {
	Get(ctx context.Context, tenantID string) ([]EgressRule, bool)
	Set(ctx context.Context, tenantID string, rules []EgressRule)
	Invalidate(ctx context.Context, tenantID string)
}

type AllowlistPolicy struct {
	store         RuleStore
	cache         RuleCache
	defaultPolicy DefaultPolicy
	mu            sync.RWMutex
	globalRules   map[string][]EgressRule
}

func NewAllowlistPolicy(store RuleStore, cache RuleCache, defaultPolicy DefaultPolicy) *AllowlistPolicy {
	if store == nil {
		store = EmptyRuleStore{}
	}
	return &AllowlistPolicy{
		store:         store,
		cache:         cache,
		defaultPolicy: defaultPolicy,
		globalRules:   make(map[string][]EgressRule),
	}
}

func (p *AllowlistPolicy) IsAllowed(ctx context.Context, tenantID string, host string, path string) (bool, error) {
	rules, err := p.getRules(ctx, tenantID)
	if err != nil {
		return false, err
	}

	decision, matched := evaluateRules(rules, host, path)
	if matched {
		return decision == ActionAllow, nil
	}

	if p.defaultPolicy != DefaultDenyAll && isBuiltinAllowed(host) {
		return true, nil
	}

	switch p.defaultPolicy {
	case DefaultAllowAll, DefaultLogOnly:
		return true, nil
	default:
		return false, nil
	}
}

func (p *AllowlistPolicy) Reload(ctx context.Context) error {
	p.mu.Lock()
	p.globalRules = make(map[string][]EgressRule)
	p.mu.Unlock()
	return nil
}

func (p *AllowlistPolicy) getRules(ctx context.Context, tenantID string) ([]EgressRule, error) {
	if p.cache != nil {
		if rules, ok := p.cache.Get(ctx, tenantID); ok {
			return rules, nil
		}
	}

	p.mu.RLock()
	if rules, ok := p.globalRules[tenantID]; ok {
		p.mu.RUnlock()
		return rules, nil
	}
	p.mu.RUnlock()

	rules, err := p.store.LoadRules(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	if p.cache != nil {
		p.cache.Set(ctx, tenantID, rules)
	}

	p.mu.Lock()
	p.globalRules[tenantID] = rules
	p.mu.Unlock()

	return rules, nil
}

func evaluateRules(rules []EgressRule, host string, path string) (Action, bool) {
	var bestMatch *EgressRule
	for i := range rules {
		r := &rules[i]
		if !matchHost(r.HostPattern, host) {
			continue
		}
		if !matchPath(r.PathPrefix, path) {
			continue
		}
		if bestMatch == nil || r.Priority > bestMatch.Priority {
			bestMatch = r
		}
	}
	if bestMatch == nil {
		return "", false
	}
	return bestMatch.Action, true
}

func matchHost(pattern string, host string) bool {
	pattern = strings.ToLower(pattern)
	host = strings.ToLower(host)

	if pattern == host {
		return true
	}

	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:]
		if strings.HasSuffix(host, suffix) && len(host) > len(suffix) {
			return true
		}
	}

	return false
}

func matchPath(prefix string, path string) bool {
	if prefix == "" || prefix == "/" {
		return true
	}
	return strings.HasPrefix(path, prefix)
}

func isBuiltinAllowed(host string) bool {
	h := strings.ToLower(host)
	for _, allowed := range builtinAllowlist {
		if h == allowed {
			return true
		}
	}
	return false
}
