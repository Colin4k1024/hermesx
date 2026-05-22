package evolution

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

const (
	policyScopeGlobal = "global"
	policyScopeTenant = "tenant"
	globalScopeID     = "global"
	revokeBatchSize   = 200
)

type policyMeta struct {
	Version int64
}

type persistedPolicyState struct {
	HasGlobal      bool
	GlobalMode     string
	GlobalMeta     policyMeta
	TenantPolicies map[string]TenantSharingPolicy
	TenantMeta     map[string]policyMeta
}

type sharingPolicyPayload struct {
	Mode string `json:"mode"`
}

type tenantSharingPolicyPayload struct {
	ConsumeShared    bool     `json:"consume_shared"`
	ContributionMode string   `json:"contribution_mode"`
	Labels           []string `json:"labels,omitempty"`
}

type policyPersistence struct {
	db     *sql.DB
	driver string
}

func resolveSQLiteDBPath(dbPath string) (string, error) {
	if dbPath != "" {
		return dbPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("evolution: resolve home dir: %w", err)
	}
	return filepath.Join(home, ".hermes", "evolution.db"), nil
}

func openPolicyPersistence(cfg Config) (*policyPersistence, error) {
	var (
		driver string
		dsn    string
	)

	if cfg.StorageMode == "mysql" {
		if cfg.MySQLDSN == "" {
			return nil, fmt.Errorf("evolution: mysql_dsn required for mysql policy storage")
		}
		driver = "mysql"
		dsn = cfg.MySQLDSN
	} else {
		resolvedPath, err := resolveSQLiteDBPath(cfg.DBPath)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(resolvedPath), 0755); err != nil {
			return nil, fmt.Errorf("evolution: create policy db dir: %w", err)
		}
		driver = "sqlite"
		dsn = resolvedPath
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("evolution: open policy store: %w", err)
	}
	if driver == "sqlite" {
		_, _ = db.Exec("PRAGMA busy_timeout = 5000")
	}

	p := &policyPersistence{db: db, driver: driver}
	if err := p.ensureSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return p, nil
}

func (p *policyPersistence) ensureSchema() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS evolution_policy_current (
			scope_type TEXT NOT NULL,
			scope_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			payload TEXT NOT NULL,
			reason TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (scope_type, scope_id)
		)`,
		`CREATE TABLE IF NOT EXISTS evolution_policy_history (
			scope_type TEXT NOT NULL,
			scope_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			payload TEXT NOT NULL,
			reason TEXT NOT NULL,
			changed_at TEXT NOT NULL,
			PRIMARY KEY (scope_type, scope_id, version)
		)`,
	}
	for _, stmt := range statements {
		if _, err := p.db.Exec(stmt); err != nil {
			return fmt.Errorf("evolution: ensure policy schema: %w", err)
		}
	}
	return nil
}

func (p *policyPersistence) LoadState() (persistedPolicyState, error) {
	state := persistedPolicyState{
		TenantPolicies: make(map[string]TenantSharingPolicy),
		TenantMeta:     make(map[string]policyMeta),
	}

	rows, err := p.db.Query(`SELECT scope_type, scope_id, version, payload FROM evolution_policy_current`)
	if err != nil {
		return state, fmt.Errorf("evolution: load policy state: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			scopeType string
			scopeID   string
			version   int64
			payload   string
		)
		if err := rows.Scan(&scopeType, &scopeID, &version, &payload); err != nil {
			return state, fmt.Errorf("evolution: scan policy state: %w", err)
		}

		switch scopeType {
		case policyScopeGlobal:
			var current sharingPolicyPayload
			if err := json.Unmarshal([]byte(payload), &current); err != nil {
				return state, fmt.Errorf("evolution: decode global policy state: %w", err)
			}
			state.HasGlobal = true
			state.GlobalMode = normalizeSharingMode(current.Mode)
			state.GlobalMeta = policyMeta{Version: version}
		case policyScopeTenant:
			var current tenantSharingPolicyPayload
			if err := json.Unmarshal([]byte(payload), &current); err != nil {
				return state, fmt.Errorf("evolution: decode tenant policy state: %w", err)
			}
			state.TenantPolicies[scopeID] = TenantSharingPolicy{
				TenantID:         scopeID,
				ConsumeShared:    current.ConsumeShared,
				ContributionMode: normalizeSharingMode(current.ContributionMode),
				Labels:           append([]string(nil), current.Labels...),
			}
			state.TenantMeta[scopeID] = policyMeta{Version: version}
		}
	}
	if err := rows.Err(); err != nil {
		return state, fmt.Errorf("evolution: iterate policy state: %w", err)
	}
	return state, nil
}

func (p *policyPersistence) RevokeSharedGenes(ctx context.Context, criteria SharedRevokeCriteria) (int, error) {
	deleted := 0
	for {
		geneIDs, err := p.listSharedGeneIDs(ctx, criteria, revokeBatchSize)
		if err != nil {
			return deleted, err
		}
		if len(geneIDs) == 0 {
			return deleted, nil
		}
		if err := p.deleteGeneBatch(ctx, geneIDs); err != nil {
			return deleted, err
		}
		deleted += len(geneIDs)
	}
}

func (p *policyPersistence) listSharedGeneIDs(ctx context.Context, criteria SharedRevokeCriteria, limit int) ([]string, error) {
	where, args := p.buildSharedRevokeWhere(criteria)
	args = append(args, limit)

	rows, err := p.db.QueryContext(ctx, fmt.Sprintf(`SELECT gene_id FROM genes %s ORDER BY created_at DESC, gene_id ASC LIMIT ?`, where), args...)
	if err != nil {
		return nil, fmt.Errorf("evolution: query shared revoke candidates: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0, limit)
	for rows.Next() {
		var geneID string
		if err := rows.Scan(&geneID); err != nil {
			return nil, fmt.Errorf("evolution: scan shared revoke candidate: %w", err)
		}
		ids = append(ids, geneID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("evolution: iterate shared revoke candidates: %w", err)
	}
	return ids, nil
}

func (p *policyPersistence) buildSharedRevokeWhere(criteria SharedRevokeCriteria) (string, []any) {
	clauses := []string{"task_class LIKE ?"}
	args := []any{sharedTaskClassPrefix + "%"}

	if criteria.TaskClass != "" {
		clauses = append(clauses, "task_class = ?")
		args = append(args, sharedTaskClass(criteria.TaskClass))
	}
	if criteria.SourceTenant != "" {
		clauses = append(clauses, "contributor_id = ?")
		args = append(args, criteria.SourceTenant)
	}
	if criteria.Source != "" {
		clauses = append(clauses, "source = ?")
		args = append(args, criteria.Source)
	}
	if criteria.From != nil {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, p.timeArg(*criteria.From))
	}
	if criteria.To != nil {
		clauses = append(clauses, "created_at < ?")
		args = append(args, p.timeArg(*criteria.To))
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func (p *policyPersistence) deleteGeneBatch(ctx context.Context, geneIDs []string) error {
	placeholders := make([]string, 0, len(geneIDs))
	args := make([]any, 0, len(geneIDs))
	for _, geneID := range geneIDs {
		placeholders = append(placeholders, "?")
		args = append(args, geneID)
	}
	if _, err := p.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM genes WHERE gene_id IN (%s)`, strings.Join(placeholders, ", ")), args...); err != nil {
		return fmt.Errorf("evolution: delete shared revoke batch: %w", err)
	}
	return nil
}

func (p *policyPersistence) timeArg(ts time.Time) any {
	if p.driver == "mysql" {
		return ts.UTC()
	}
	return ts.UTC().Format(time.RFC3339)
}

func (p *policyPersistence) StoreGlobal(mode string, reason string) (policyMeta, error) {
	payload, err := json.Marshal(sharingPolicyPayload{Mode: normalizeSharingMode(mode)})
	if err != nil {
		return policyMeta{}, fmt.Errorf("evolution: encode global policy: %w", err)
	}
	return p.storePolicy(policyScopeGlobal, globalScopeID, string(payload), reason)
}

func (p *policyPersistence) StoreTenant(policy TenantSharingPolicy, reason string) (policyMeta, error) {
	payload, err := json.Marshal(tenantSharingPolicyPayload{
		ConsumeShared:    policy.ConsumeShared,
		ContributionMode: normalizeSharingMode(policy.ContributionMode),
		Labels:           append([]string(nil), policy.Labels...),
	})
	if err != nil {
		return policyMeta{}, fmt.Errorf("evolution: encode tenant policy: %w", err)
	}
	return p.storePolicy(policyScopeTenant, policy.TenantID, string(payload), reason)
}

func (p *policyPersistence) ListHistory(scopeType string, scopeID string, limit int, offset int) ([]SharingPolicyHistoryEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := p.db.Query(
		`SELECT version, payload, reason, changed_at FROM evolution_policy_history WHERE scope_type = ? AND scope_id = ? ORDER BY version DESC LIMIT ? OFFSET ?`,
		scopeType,
		scopeID,
		limit,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("evolution: list policy history: %w", err)
	}
	defer rows.Close()

	entries := make([]SharingPolicyHistoryEntry, 0, limit)
	for rows.Next() {
		var (
			version   int64
			payload   string
			reason    string
			changedAt string
		)
		if err := rows.Scan(&version, &payload, &reason, &changedAt); err != nil {
			return nil, fmt.Errorf("evolution: scan policy history: %w", err)
		}
		entry, err := decodeHistoryEntry(scopeType, scopeID, version, payload, reason, changedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("evolution: iterate policy history: %w", err)
	}
	return entries, nil
}

func decodeHistoryEntry(scopeType string, scopeID string, version int64, payload string, reason string, changedAt string) (SharingPolicyHistoryEntry, error) {
	parsedTime, err := time.Parse(time.RFC3339Nano, changedAt)
	if err != nil {
		return SharingPolicyHistoryEntry{}, fmt.Errorf("evolution: parse policy history timestamp: %w", err)
	}
	entry := SharingPolicyHistoryEntry{
		ScopeType: scopeType,
		ScopeID:   scopeID,
		Version:   version,
		Reason:    reason,
		ChangedAt: parsedTime,
	}
	switch scopeType {
	case policyScopeGlobal:
		var current sharingPolicyPayload
		if err := json.Unmarshal([]byte(payload), &current); err != nil {
			return SharingPolicyHistoryEntry{}, fmt.Errorf("evolution: decode global history payload: %w", err)
		}
		entry.Mode = normalizeSharingMode(current.Mode)
	case policyScopeTenant:
		var current tenantSharingPolicyPayload
		if err := json.Unmarshal([]byte(payload), &current); err != nil {
			return SharingPolicyHistoryEntry{}, fmt.Errorf("evolution: decode tenant history payload: %w", err)
		}
		consumeShared := current.ConsumeShared
		entry.ConsumeShared = &consumeShared
		entry.ContributionMode = normalizeSharingMode(current.ContributionMode)
		entry.Labels = append([]string(nil), current.Labels...)
	default:
		return SharingPolicyHistoryEntry{}, fmt.Errorf("evolution: unsupported policy history scope %q", scopeType)
	}
	return entry, nil
}

func (p *policyPersistence) LoadGlobalVersion(version int64) (string, error) {
	payload, err := p.loadHistoryPayload(policyScopeGlobal, globalScopeID, version)
	if err != nil {
		return "", err
	}
	var current sharingPolicyPayload
	if err := json.Unmarshal([]byte(payload), &current); err != nil {
		return "", fmt.Errorf("evolution: decode global version payload: %w", err)
	}
	return normalizeSharingMode(current.Mode), nil
}

func (p *policyPersistence) LoadTenantVersion(tenantID string, version int64) (TenantSharingPolicy, error) {
	payload, err := p.loadHistoryPayload(policyScopeTenant, tenantID, version)
	if err != nil {
		return TenantSharingPolicy{}, err
	}
	var current tenantSharingPolicyPayload
	if err := json.Unmarshal([]byte(payload), &current); err != nil {
		return TenantSharingPolicy{}, fmt.Errorf("evolution: decode tenant version payload: %w", err)
	}
	return TenantSharingPolicy{
		TenantID:         tenantID,
		ConsumeShared:    current.ConsumeShared,
		ContributionMode: normalizeSharingMode(current.ContributionMode),
		Labels:           append([]string(nil), current.Labels...),
	}, nil
}

func (p *policyPersistence) loadHistoryPayload(scopeType string, scopeID string, version int64) (string, error) {
	var payload string
	err := p.db.QueryRow(
		`SELECT payload FROM evolution_policy_history WHERE scope_type = ? AND scope_id = ? AND version = ?`,
		scopeType,
		scopeID,
		version,
	).Scan(&payload)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("%w: %d for %s/%s", ErrPolicyVersionNotFound, version, scopeType, scopeID)
		}
		return "", fmt.Errorf("evolution: load history payload: %w", err)
	}
	return payload, nil
}

func (p *policyPersistence) storePolicy(scopeType string, scopeID string, payload string, reason string) (policyMeta, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return policyMeta{}, fmt.Errorf("evolution: begin policy transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var currentVersion int64
	if scanErr := tx.QueryRow(
		`SELECT COALESCE(MAX(version), 0) FROM evolution_policy_history WHERE scope_type = ? AND scope_id = ?`,
		scopeType,
		scopeID,
	).Scan(&currentVersion); scanErr != nil {
		err = fmt.Errorf("evolution: read policy version: %w", scanErr)
		return policyMeta{}, err
	}

	nextVersion := currentVersion + 1
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)

	if _, execErr := tx.Exec(
		`INSERT INTO evolution_policy_history (scope_type, scope_id, version, payload, reason, changed_at) VALUES (?, ?, ?, ?, ?, ?)`,
		scopeType,
		scopeID,
		nextVersion,
		payload,
		reason,
		timestamp,
	); execErr != nil {
		err = fmt.Errorf("evolution: append policy history: %w", execErr)
		return policyMeta{}, err
	}

	if _, execErr := tx.Exec(
		`DELETE FROM evolution_policy_current WHERE scope_type = ? AND scope_id = ?`,
		scopeType,
		scopeID,
	); execErr != nil {
		err = fmt.Errorf("evolution: clear current policy: %w", execErr)
		return policyMeta{}, err
	}

	if _, execErr := tx.Exec(
		`INSERT INTO evolution_policy_current (scope_type, scope_id, version, payload, reason, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		scopeType,
		scopeID,
		nextVersion,
		payload,
		reason,
		timestamp,
	); execErr != nil {
		err = fmt.Errorf("evolution: write current policy: %w", execErr)
		return policyMeta{}, err
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return policyMeta{}, fmt.Errorf("evolution: commit policy transaction: %w", commitErr)
	}
	return policyMeta{Version: nextVersion}, nil
}

func (p *policyPersistence) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}
