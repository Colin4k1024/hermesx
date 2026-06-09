package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/Colin4k1024/hermesx/internal/metering"
	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// AlertRuleStore
// ---------------------------------------------------------------------------

type pgAlertRuleStore struct {
	pool *pgxpool.Pool
}

func (s *pgAlertRuleStore) Create(ctx context.Context, rule *metering.AlertRule) error {
	return withTenantTx(ctx, s.pool, rule.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO alert_rules (id, tenant_id, metric, threshold, alert_window, enabled, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			rule.ID, rule.TenantID, string(rule.Metric), rule.Threshold,
			rule.Window, rule.Enabled, rule.CreatedAt, rule.UpdatedAt)
		if err != nil {
			return fmt.Errorf("pg create alert rule: %w", err)
		}
		return nil
	})
}

func (s *pgAlertRuleStore) Get(ctx context.Context, tenantID, ruleID string) (*metering.AlertRule, error) {
	var r metering.AlertRule
	var metric string
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, metric, threshold, alert_window, enabled, created_at, updated_at
		 FROM alert_rules WHERE tenant_id = $1 AND id = $2`,
		tenantID, ruleID).Scan(
		&r.ID, &r.TenantID, &metric, &r.Threshold, &r.Window,
		&r.Enabled, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("alert rule %s: %w", ruleID, store.ErrNotFound)
		}
		return nil, fmt.Errorf("pg get alert rule: %w", err)
	}
	r.Metric = metering.AlertMetric(metric)
	return &r, nil
}

func (s *pgAlertRuleStore) List(ctx context.Context, tenantID string) ([]*metering.AlertRule, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, metric, threshold, alert_window, enabled, created_at, updated_at
		 FROM alert_rules WHERE tenant_id = $1 ORDER BY created_at DESC`,
		tenantID)
	if err != nil {
		return nil, fmt.Errorf("pg list alert rules: %w", err)
	}
	defer rows.Close()

	var rules []*metering.AlertRule
	for rows.Next() {
		var r metering.AlertRule
		var metric string
		if err := rows.Scan(&r.ID, &r.TenantID, &metric, &r.Threshold, &r.Window,
			&r.Enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("pg scan alert rule: %w", err)
		}
		r.Metric = metering.AlertMetric(metric)
		rules = append(rules, &r)
	}
	return rules, rows.Err()
}

func (s *pgAlertRuleStore) Update(ctx context.Context, rule *metering.AlertRule) error {
	return withTenantTx(ctx, s.pool, rule.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE alert_rules SET metric=$3, threshold=$4, alert_window=$5, enabled=$6, updated_at=$7
			 WHERE tenant_id = $1 AND id = $2`,
			rule.TenantID, rule.ID, string(rule.Metric), rule.Threshold,
			rule.Window, rule.Enabled, rule.UpdatedAt)
		if err != nil {
			return fmt.Errorf("pg update alert rule: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("alert rule %s: %w", rule.ID, store.ErrNotFound)
		}
		return nil
	})
}

func (s *pgAlertRuleStore) Delete(ctx context.Context, tenantID, ruleID string) error {
	return withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`DELETE FROM alert_rules WHERE tenant_id = $1 AND id = $2`,
			tenantID, ruleID)
		if err != nil {
			return fmt.Errorf("pg delete alert rule: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("alert rule %s: %w", ruleID, store.ErrNotFound)
		}
		return nil
	})
}

// ListAllEnabled returns all enabled alert rules across all tenants for the AlertChecker.
// tenant_sql_check:skip -- intentional cross-tenant query for background checker.
func (s *pgAlertRuleStore) ListAllEnabled(ctx context.Context) ([]*metering.AlertRule, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, metric, threshold, alert_window, enabled, created_at, updated_at
		 FROM alert_rules WHERE enabled = true ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("pg list all enabled alert rules: %w", err)
	}
	defer rows.Close()

	var rules []*metering.AlertRule
	for rows.Next() {
		var r metering.AlertRule
		var metric string
		if err := rows.Scan(&r.ID, &r.TenantID, &metric, &r.Threshold, &r.Window,
			&r.Enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("pg scan alert rule: %w", err)
		}
		r.Metric = metering.AlertMetric(metric)
		rules = append(rules, &r)
	}
	return rules, rows.Err()
}

// ---------------------------------------------------------------------------
// AlertEventStore
// ---------------------------------------------------------------------------

type pgAlertEventStore struct {
	pool *pgxpool.Pool
}

func (s *pgAlertEventStore) Record(ctx context.Context, event *metering.AlertEvent) error {
	return withTenantTx(ctx, s.pool, event.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO alert_events (id, tenant_id, rule_id, metric, threshold, current_val, percentage, fired_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			event.ID, event.TenantID, event.RuleID, string(event.Metric),
			event.Threshold, event.Current, event.Percentage, event.FiredAt)
		if err != nil {
			return fmt.Errorf("pg record alert event: %w", err)
		}
		return nil
	})
}

func (s *pgAlertEventStore) ListByTenant(ctx context.Context, tenantID string, limit int) ([]*metering.AlertEvent, error) {
	return s.ListByTenantPage(ctx, tenantID, limit, 0)
}

func (s *pgAlertEventStore) ListByTenantPage(ctx context.Context, tenantID string, limit, offset int) ([]*metering.AlertEvent, error) {
	if limit <= 0 || limit > 10000 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, rule_id, metric, threshold, current_val, percentage, fired_at
		 FROM alert_events WHERE tenant_id = $1 ORDER BY fired_at DESC LIMIT $2 OFFSET $3`,
		tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("pg list alert events: %w", err)
	}
	defer rows.Close()

	var events []*metering.AlertEvent
	for rows.Next() {
		var e metering.AlertEvent
		var metric string
		if err := rows.Scan(&e.ID, &e.TenantID, &e.RuleID, &metric,
			&e.Threshold, &e.Current, &e.Percentage, &e.FiredAt); err != nil {
			return nil, fmt.Errorf("pg scan alert event: %w", err)
		}
		e.Metric = metering.AlertMetric(metric)
		events = append(events, &e)
	}
	return events, rows.Err()
}

// ---------------------------------------------------------------------------
// Compile-time interface checks
// ---------------------------------------------------------------------------

var _ metering.AlertRuleStore = (*pgAlertRuleStore)(nil)
var _ metering.AlertEventStore = (*pgAlertEventStore)(nil)
