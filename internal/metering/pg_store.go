package metering

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PGUsageStore implements UsageStore backed by PostgreSQL.
type PGUsageStore struct {
	pool *pgxpool.Pool
}

// NewPGUsageStore creates a new PostgreSQL usage store.
func NewPGUsageStore(pool *pgxpool.Pool) *PGUsageStore {
	return &PGUsageStore{pool: pool}
}

// BatchInsert inserts multiple usage records in a single statement.
func (s *PGUsageStore) BatchInsert(ctx context.Context, records []UsageRecord) error {
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
		base := i * 12
		fmt.Fprintf(&b, "($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6,
			base+7, base+8, base+9, base+10, base+11, base+12)
		createdAt := r.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		args = append(args, r.TenantID, r.SessionID, r.UserID, r.Model, r.Provider,
			r.InputTokens, r.OutputTokens, r.CacheReadTokens, r.CacheWriteTokens,
			r.CostUSD, r.Degraded, createdAt)
	}

	_, err := s.pool.Exec(ctx, b.String(), args...)
	return err
}

// QueryByTenant returns aggregated usage for a tenant within a time range.
func (s *PGUsageStore) QueryByTenant(ctx context.Context, tenantID string, from, to time.Time, granularity string) ([]UsageSummary, error) {
	// Whitelist-only approach: reject unknown granularity to prevent any injection path.
	var truncExpr string
	switch granularity {
	case "hour":
		truncExpr = "hour"
	case "week":
		truncExpr = "week"
	case "month":
		truncExpr = "month"
	case "day", "":
		truncExpr = "day"
	default:
		return nil, fmt.Errorf("invalid granularity: %q (allowed: hour, day, week, month)", granularity)
	}

	query := fmt.Sprintf(`
		SELECT date_trunc('%s', created_at)::date::text AS bucket,
		       COALESCE(SUM(input_tokens), 0),
		       COALESCE(SUM(output_tokens), 0),
		       COALESCE(SUM(cost_usd), 0)
		FROM usage_records
		WHERE tenant_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY bucket
		ORDER BY bucket
	`, truncExpr)

	rows, err := s.pool.Query(ctx, query, tenantID, from, to)
	if err != nil {
		return nil, fmt.Errorf("query usage by tenant: %w", err)
	}
	defer rows.Close()

	var results []UsageSummary
	for rows.Next() {
		var us UsageSummary
		if err := rows.Scan(&us.Date, &us.InputTokens, &us.OutputTokens, &us.CostUSD); err != nil {
			return nil, fmt.Errorf("scan usage summary: %w", err)
		}
		results = append(results, us)
	}
	return results, rows.Err()
}

// QueryBySession returns all usage records for a specific session.
func (s *PGUsageStore) QueryBySession(ctx context.Context, tenantID, sessionID string) ([]UsageRecord, error) {
	query := `
		SELECT tenant_id, session_id, user_id, model, provider,
		       input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
		       cost_usd, degraded, created_at
		FROM usage_records
		WHERE tenant_id = $1 AND session_id = $2
		ORDER BY created_at
	`

	rows, err := s.pool.Query(ctx, query, tenantID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query usage by session: %w", err)
	}
	defer rows.Close()

	var results []UsageRecord
	for rows.Next() {
		var r UsageRecord
		if err := rows.Scan(&r.TenantID, &r.SessionID, &r.UserID, &r.Model, &r.Provider,
			&r.InputTokens, &r.OutputTokens, &r.CacheReadTokens, &r.CacheWriteTokens,
			&r.CostUSD, &r.Degraded, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan usage record: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
