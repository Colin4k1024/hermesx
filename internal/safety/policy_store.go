package safety

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PolicyStore interface {
	GetPolicy(ctx context.Context, tenantID string) (*Policy, error)
	UpsertPolicy(ctx context.Context, policy *Policy) error
	ListPolicies(ctx context.Context) ([]Policy, error)
}

type PostgresPolicyStore struct {
	pool *pgxpool.Pool
}

func NewPostgresPolicyStore(pool *pgxpool.Pool) *PostgresPolicyStore {
	return &PostgresPolicyStore{pool: pool}
}

type storedInputPattern struct {
	Text     string `json:"text"`
	Regex    string `json:"regex,omitempty"`
	Severity int    `json:"severity"`
}

type storedOutputRule struct {
	Description string `json:"description"`
	Contains    string `json:"contains,omitempty"`
	Regex       string `json:"regex,omitempty"`
	Severity    int    `json:"severity"`
}

func (ps *PostgresPolicyStore) GetPolicy(ctx context.Context, tenantID string) (*Policy, error) {
	query := `SELECT id, tenant_id, mode, input_patterns, output_rules FROM safety_policies WHERE tenant_id = $1`

	var (
		id               string
		tid              string
		mode             string
		inputPatternsRaw []byte
		outputRulesRaw   []byte
	)

	err := ps.pool.QueryRow(ctx, query, tenantID).Scan(&id, &tid, &mode, &inputPatternsRaw, &outputRulesRaw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query safety policy: %w", err)
	}

	policy := &Policy{
		ID:       id,
		TenantID: tid,
		Mode:     PolicyMode(mode),
	}

	if len(inputPatternsRaw) > 0 {
		var stored []storedInputPattern
		if err := json.Unmarshal(inputPatternsRaw, &stored); err == nil {
			for _, s := range stored {
				ip := InputPattern{Text: s.Text, Severity: s.Severity}
				if s.Regex != "" {
					if r, err := regexp.Compile(s.Regex); err == nil {
						ip.Regex = r
					} else {
						slog.Warn("safety policy: skipping input pattern with invalid regex", "tenant_id", tid, "regex", s.Regex, "error", err)
					}
				}
				policy.InputPatterns = append(policy.InputPatterns, ip)
			}
		}
	}

	if len(outputRulesRaw) > 0 {
		var stored []storedOutputRule
		if err := json.Unmarshal(outputRulesRaw, &stored); err == nil {
			for _, s := range stored {
				or := OutputRule{Description: s.Description, Contains: s.Contains, Severity: s.Severity}
				if s.Regex != "" {
					if r, err := regexp.Compile(s.Regex); err == nil {
						or.Regex = r
					} else {
						slog.Warn("safety policy: skipping output rule with invalid regex", "tenant_id", tid, "regex", s.Regex, "error", err)
					}
				}
				policy.OutputRules = append(policy.OutputRules, or)
			}
		}
	}

	return policy, nil
}

func (ps *PostgresPolicyStore) UpsertPolicy(ctx context.Context, policy *Policy) error {
	inputPatternsJSON, err := marshalInputPatterns(policy.InputPatterns)
	if err != nil {
		return fmt.Errorf("marshal input patterns: %w", err)
	}

	outputRulesJSON, err := marshalOutputRules(policy.OutputRules)
	if err != nil {
		return fmt.Errorf("marshal output rules: %w", err)
	}

	query := `
		INSERT INTO safety_policies (tenant_id, mode, input_patterns, output_rules, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id)
		DO UPDATE SET mode = $2, input_patterns = $3, output_rules = $4, updated_at = $5`

	_, err = ps.pool.Exec(ctx, query,
		policy.TenantID,
		string(policy.Mode),
		inputPatternsJSON,
		outputRulesJSON,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("upsert safety policy: %w", err)
	}

	return nil
}

func (ps *PostgresPolicyStore) ListPolicies(ctx context.Context) ([]Policy, error) {
	query := `SELECT id, tenant_id, mode, input_patterns, output_rules FROM safety_policies ORDER BY created_at DESC`

	rows, err := ps.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list safety policies: %w", err)
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		var (
			id               string
			tid              string
			mode             string
			inputPatternsRaw []byte
			outputRulesRaw   []byte
		)

		if err := rows.Scan(&id, &tid, &mode, &inputPatternsRaw, &outputRulesRaw); err != nil {
			return nil, fmt.Errorf("scan safety policy: %w", err)
		}

		p := Policy{
			ID:       id,
			TenantID: tid,
			Mode:     PolicyMode(mode),
		}

		if len(inputPatternsRaw) > 0 {
			var stored []storedInputPattern
			if err := json.Unmarshal(inputPatternsRaw, &stored); err == nil {
				for _, s := range stored {
					ip := InputPattern{Text: s.Text, Severity: s.Severity}
					if s.Regex != "" {
						if r, err := regexp.Compile(s.Regex); err == nil {
							ip.Regex = r
						} else {
							slog.Warn("safety policy: skipping input pattern with invalid regex", "tenant_id", tid, "regex", s.Regex, "error", err)
						}
					}
					p.InputPatterns = append(p.InputPatterns, ip)
				}
			}
		}

		if len(outputRulesRaw) > 0 {
			var stored []storedOutputRule
			if err := json.Unmarshal(outputRulesRaw, &stored); err == nil {
				for _, s := range stored {
					or := OutputRule{Description: s.Description, Contains: s.Contains, Severity: s.Severity}
					if s.Regex != "" {
						if r, err := regexp.Compile(s.Regex); err == nil {
							or.Regex = r
						} else {
							slog.Warn("safety policy: skipping output rule with invalid regex", "tenant_id", tid, "regex", s.Regex, "error", err)
						}
					}
					p.OutputRules = append(p.OutputRules, or)
				}
			}
		}

		policies = append(policies, p)
	}

	return policies, nil
}

func marshalInputPatterns(patterns []InputPattern) ([]byte, error) {
	stored := make([]storedInputPattern, 0, len(patterns))
	for _, p := range patterns {
		sp := storedInputPattern{Text: p.Text, Severity: p.Severity}
		if p.Regex != nil {
			sp.Regex = p.Regex.String()
		}
		stored = append(stored, sp)
	}
	return json.Marshal(stored)
}

func marshalOutputRules(rules []OutputRule) ([]byte, error) {
	stored := make([]storedOutputRule, 0, len(rules))
	for _, r := range rules {
		sr := storedOutputRule{Description: r.Description, Contains: r.Contains, Severity: r.Severity}
		if r.Regex != nil {
			sr.Regex = r.Regex.String()
		}
		stored = append(stored, sr)
	}
	return json.Marshal(stored)
}

type InMemoryPolicyStore struct {
	mu       sync.RWMutex
	policies map[string]*Policy
}

func NewInMemoryPolicyStore() *InMemoryPolicyStore {
	return &InMemoryPolicyStore{
		policies: make(map[string]*Policy),
	}
}

func (s *InMemoryPolicyStore) GetPolicy(_ context.Context, tenantID string) (*Policy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.policies[tenantID]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (s *InMemoryPolicyStore) UpsertPolicy(_ context.Context, policy *Policy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[policy.TenantID] = policy
	return nil
}

func (s *InMemoryPolicyStore) ListPolicies(_ context.Context) ([]Policy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Policy
	for _, p := range s.policies {
		result = append(result, *p)
	}
	return result, nil
}
