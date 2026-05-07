package pg

import (
	"context"
	"errors"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgPricingRuleStore struct{ pool *pgxpool.Pool }

func (s *pgPricingRuleStore) List(ctx context.Context) ([]store.PricingRule, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT model_key, input_per_1k, output_per_1k, cache_read_per_1k, updated_at
		FROM pricing_rules ORDER BY model_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []store.PricingRule
	for rows.Next() {
		var r store.PricingRule
		if err := rows.Scan(&r.ModelKey, &r.InputPer1K, &r.OutputPer1K, &r.CacheReadPer1K, &r.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *pgPricingRuleStore) Get(ctx context.Context, modelKey string) (*store.PricingRule, error) {
	var r store.PricingRule
	err := s.pool.QueryRow(ctx, `
		SELECT model_key, input_per_1k, output_per_1k, cache_read_per_1k, updated_at
		FROM pricing_rules WHERE model_key = $1`, modelKey).
		Scan(&r.ModelKey, &r.InputPer1K, &r.OutputPer1K, &r.CacheReadPer1K, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *pgPricingRuleStore) Upsert(ctx context.Context, rule *store.PricingRule) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO pricing_rules (model_key, input_per_1k, output_per_1k, cache_read_per_1k, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (model_key) DO UPDATE SET
			input_per_1k = EXCLUDED.input_per_1k,
			output_per_1k = EXCLUDED.output_per_1k,
			cache_read_per_1k = EXCLUDED.cache_read_per_1k,
			updated_at = NOW()`,
		rule.ModelKey, rule.InputPer1K, rule.OutputPer1K, rule.CacheReadPer1K)
	return err
}

func (s *pgPricingRuleStore) Delete(ctx context.Context, modelKey string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM pricing_rules WHERE model_key = $1`, modelKey)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}
