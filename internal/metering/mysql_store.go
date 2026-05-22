package metering

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// MySQLUsageStore implements UsageStore backed by MySQL.
type MySQLUsageStore struct {
	db *sql.DB
}

// NewMySQLUsageStore creates a new MySQL usage store.
func NewMySQLUsageStore(db *sql.DB) *MySQLUsageStore {
	return &MySQLUsageStore{db: db}
}

// BatchInsert inserts multiple usage records in a single statement.
func (s *MySQLUsageStore) BatchInsert(ctx context.Context, records []UsageRecord) error {
	if len(records) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString(`INSERT INTO usage_records (tenant_id, session_id, user_id, model, provider, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, cost_usd, degraded, created_at) VALUES `)

	args := make([]any, 0, len(records)*12)
	for i, r := range records {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		createdAt := r.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		args = append(args, r.TenantID, r.SessionID, r.UserID, r.Model, r.Provider,
			r.InputTokens, r.OutputTokens, r.CacheReadTokens, r.CacheWriteTokens,
			r.CostUSD, r.Degraded, createdAt)
	}

	_, err := s.db.ExecContext(ctx, b.String(), args...)
	return err
}

// QueryByTenant returns aggregated usage for a tenant within a time range.
func (s *MySQLUsageStore) QueryByTenant(ctx context.Context, tenantID string, from, to time.Time, granularity string) ([]UsageSummary, error) {
	var dateExpr string
	switch granularity {
	case "hour":
		dateExpr = "DATE_FORMAT(created_at, '%Y-%m-%d %H:00:00')"
	case "week":
		dateExpr = "DATE_FORMAT(DATE_SUB(created_at, INTERVAL WEEKDAY(created_at) DAY), '%Y-%m-%d')"
	case "month":
		dateExpr = "DATE_FORMAT(created_at, '%Y-%m-01')"
	case "day", "":
		dateExpr = "DATE_FORMAT(created_at, '%Y-%m-%d')"
	default:
		return nil, fmt.Errorf("invalid granularity: %q (allowed: hour, day, week, month)", granularity)
	}

	query := fmt.Sprintf(`
		SELECT %s AS bucket,
		       COALESCE(SUM(input_tokens), 0),
		       COALESCE(SUM(output_tokens), 0),
		       COALESCE(SUM(cost_usd), 0)
		FROM usage_records
		WHERE tenant_id = ? AND created_at >= ? AND created_at < ?
		GROUP BY bucket
		ORDER BY bucket
	`, dateExpr)

	rows, err := s.db.QueryContext(ctx, query, tenantID, from, to)
	if err != nil {
		return nil, fmt.Errorf("mysql query usage by tenant: %w", err)
	}
	defer rows.Close()

	var results []UsageSummary
	for rows.Next() {
		var us UsageSummary
		if err := rows.Scan(&us.Date, &us.InputTokens, &us.OutputTokens, &us.CostUSD); err != nil {
			return nil, fmt.Errorf("mysql scan usage summary: %w", err)
		}
		results = append(results, us)
	}
	return results, rows.Err()
}

// QueryBySession returns all usage records for a specific session.
func (s *MySQLUsageStore) QueryBySession(ctx context.Context, tenantID, sessionID string) ([]UsageRecord, error) {
	query := `
		SELECT tenant_id, session_id, user_id, model, provider,
		       input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
		       cost_usd, degraded, created_at
		FROM usage_records
		WHERE tenant_id = ? AND session_id = ?
		ORDER BY created_at
	`

	rows, err := s.db.QueryContext(ctx, query, tenantID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("mysql query usage by session: %w", err)
	}
	defer rows.Close()

	var results []UsageRecord
	for rows.Next() {
		var r UsageRecord
		if err := rows.Scan(&r.TenantID, &r.SessionID, &r.UserID, &r.Model, &r.Provider,
			&r.InputTokens, &r.OutputTokens, &r.CacheReadTokens, &r.CacheWriteTokens,
			&r.CostUSD, &r.Degraded, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("mysql scan usage record: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// QueryTenants returns aggregate-only usage across tenants for platform
// governance views.
func (s *MySQLUsageStore) QueryTenants(ctx context.Context, q TenantUsageQuery) ([]TenantUsageSummary, error) {
	limit := q.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT tenant_id,
		       COUNT(DISTINCT NULLIF(session_id, '')) AS session_count,
		       COALESCE(SUM(input_tokens), 0),
		       COALESCE(SUM(output_tokens), 0),
		       COALESCE(SUM(cost_usd), 0)
		FROM usage_records
		WHERE (? IS NULL OR created_at >= ?)
		  AND (? IS NULL OR created_at < ?)
		GROUP BY tenant_id
		ORDER BY COALESCE(SUM(cost_usd), 0) DESC, tenant_id ASC
		LIMIT ? OFFSET ?`

	rows, err := s.db.QueryContext(ctx, query, q.From, q.From, q.To, q.To, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("mysql query tenant usage: %w", err)
	}
	defer rows.Close()

	var results []TenantUsageSummary
	for rows.Next() {
		var item TenantUsageSummary
		if err := rows.Scan(&item.TenantID, &item.SessionCount, &item.InputTokens, &item.OutputTokens, &item.CostUSD); err != nil {
			return nil, fmt.Errorf("mysql scan tenant usage: %w", err)
		}
		results = append(results, item)
	}
	return results, rows.Err()
}
