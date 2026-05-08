package mysql

import (
	"context"
	"database/sql"

	"github.com/Colin4k1024/hermesx/internal/store"
)

type myPricingRuleStore struct{ db *sql.DB }

func (s *myPricingRuleStore) List(ctx context.Context) ([]store.PricingRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT model_key, input_per_1k, output_per_1k, cache_read_per_1k, updated_at
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

func (s *myPricingRuleStore) Get(ctx context.Context, modelKey string) (*store.PricingRule, error) {
	var r store.PricingRule
	err := s.db.QueryRowContext(ctx,
		`SELECT model_key, input_per_1k, output_per_1k, cache_read_per_1k, updated_at
		 FROM pricing_rules WHERE model_key = ?`, modelKey).
		Scan(&r.ModelKey, &r.InputPer1K, &r.OutputPer1K, &r.CacheReadPer1K, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &r, err
}

func (s *myPricingRuleStore) Upsert(ctx context.Context, rule *store.PricingRule) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO pricing_rules (model_key, input_per_1k, output_per_1k, cache_read_per_1k)
		 VALUES (?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		   input_per_1k = VALUES(input_per_1k),
		   output_per_1k = VALUES(output_per_1k),
		   cache_read_per_1k = VALUES(cache_read_per_1k)`,
		rule.ModelKey, rule.InputPer1K, rule.OutputPer1K, rule.CacheReadPer1K)
	return err
}

func (s *myPricingRuleStore) Delete(ctx context.Context, modelKey string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM pricing_rules WHERE model_key = ?`, modelKey)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

var _ store.PricingRuleStore = (*myPricingRuleStore)(nil)
