package evolution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	orisstore "github.com/Colin4k1024/Oris/sdks/go/store"
)

// allowedOrderBy is a whitelist of safe ORDER BY clauses (B4: SQL injection defense).
var allowedOrderBy = map[string]string{
	"confidence_desc": "confidence DESC",
	"use_count_desc":  "use_count DESC",
	"created_at_desc": "created_at DESC",
}

// safeOrderBy returns an allowlisted ORDER BY clause, defaulting to confidence DESC.
func safeOrderBy(key string) string {
	if v, ok := allowedOrderBy[key]; ok {
		return v
	}
	return "confidence DESC"
}

// tenantTaskClass namespaces taskClass under tenantID to prevent cross-tenant gene sharing (B2).
func tenantTaskClass(tenantID, taskClass string) string {
	if tenantID == "" {
		return taskClass
	}
	return tenantID + ":" + taskClass
}

// GeneStore wraps the Oris store.Store for hermesx evolution.
type GeneStore struct {
	inner               orisstore.Store
	policyPersistence   *policyPersistence
	mu                  sync.RWMutex
	cfg                 Config
	sharingVersion      int64
	tenantPolicyVersion map[string]int64
}

// Open creates a GeneStore backed by SQLite (default) or MySQL.
func Open(cfg Config) (*GeneStore, error) {
	if cfg.StorageMode == "mysql" {
		if cfg.MySQLDSN == "" {
			return nil, fmt.Errorf("evolution: mysql_dsn required for mysql storage_mode")
		}
		s, err := orisstore.OpenMySQLFromDSN(cfg.MySQLDSN)
		if err != nil {
			return nil, fmt.Errorf("evolution: open mysql: %w", err)
		}
		policyPersistence, err := openPolicyPersistence(cfg)
		if err != nil {
			_ = s.Close()
			return nil, err
		}
		loaded, err := policyPersistence.LoadState()
		if err != nil {
			_ = policyPersistence.Close()
			_ = s.Close()
			return nil, err
		}
		if loaded.HasGlobal {
			cfg.SharingMode = loaded.GlobalMode
		}
		if len(loaded.TenantPolicies) > 0 {
			cfg.TenantPolicies = loaded.TenantPolicies
		}
		return &GeneStore{
			inner:               s,
			policyPersistence:   policyPersistence,
			cfg:                 cfg,
			sharingVersion:      loaded.GlobalMeta.Version,
			tenantPolicyVersion: toTenantPolicyVersionMap(loaded.TenantMeta),
		}, nil
	}

	dbPath, err := resolveSQLiteDBPath(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("evolution: create db dir: %w", err)
	}

	s, err := orisstore.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("evolution: open sqlite: %w", err)
	}
	policyPersistence, err := openPolicyPersistence(cfg)
	if err != nil {
		_ = s.Close()
		return nil, err
	}
	loaded, err := policyPersistence.LoadState()
	if err != nil {
		_ = policyPersistence.Close()
		_ = s.Close()
		return nil, err
	}
	if loaded.HasGlobal {
		cfg.SharingMode = loaded.GlobalMode
	}
	if len(loaded.TenantPolicies) > 0 {
		cfg.TenantPolicies = loaded.TenantPolicies
	}
	return &GeneStore{
		inner:               s,
		policyPersistence:   policyPersistence,
		cfg:                 cfg,
		sharingVersion:      loaded.GlobalMeta.Version,
		tenantPolicyVersion: toTenantPolicyVersionMap(loaded.TenantMeta),
	}, nil
}

func toTenantPolicyVersionMap(meta map[string]policyMeta) map[string]int64 {
	if len(meta) == 0 {
		return make(map[string]int64)
	}
	out := make(map[string]int64, len(meta))
	for tenantID, item := range meta {
		out[tenantID] = item.Version
	}
	return out
}

// QueryTop returns up to limit genes for taskClass with confidence ≥ minConfidence.
// tenantID namespaces the query to prevent cross-tenant gene leakage (B2).
func (g *GeneStore) QueryTop(ctx context.Context, tenantID string, taskClass TaskClass, minConfidence float64, limit int) ([]orisstore.Gene, error) {
	if limit <= 0 {
		return nil, nil
	}
	sharingMode := g.SharingMode()

	local, err := g.inner.Query(ctx, orisstore.StoreQuery{
		TaskClass:     tenantTaskClass(tenantID, taskClass),
		MinConfidence: minConfidence,
		OrderBy:       safeOrderBy("confidence_desc"),
		Limit:         limit,
	})
	if err != nil {
		return nil, err
	}
	if tenantID == "" || sharingMode == SharingDisabled || !g.EffectiveTenantSharingPolicy(tenantID).ConsumeShared || len(local) >= limit {
		return local, nil
	}

	shared, err := g.inner.Query(ctx, orisstore.StoreQuery{
		TaskClass:     sharedTaskClass(taskClass),
		MinConfidence: minConfidence,
		OrderBy:       safeOrderBy("confidence_desc"),
		Limit:         limit - len(local),
	})
	if err != nil {
		return nil, err
	}
	return append(local, shared...), nil
}

// Save upserts a gene under the tenant namespace (B2).
func (g *GeneStore) Save(ctx context.Context, tenantID string, gene orisstore.Gene) error {
	originalTaskClass := gene.TaskClass
	mode := g.EffectiveTenantSharingPolicy(tenantID).EffectiveContributionMode
	gene.TaskClass = tenantTaskClass(tenantID, gene.TaskClass)
	if err := g.inner.Save(ctx, gene); err != nil {
		return err
	}

	if tenantID == "" || mode == SharingDisabled {
		return nil
	}

	sharedGene := gene
	sharedGene.GeneID = "shared-" + gene.GeneID
	sharedGene.TaskClass = sharedTaskClass(originalTaskClass)
	sharedGene.Source = "shared." + mode
	if mode == SharingTrusted {
		sharedGene.ContributorID = tenantID
	} else {
		sharedGene.ContributorID = ""
	}
	return g.inner.Save(ctx, sharedGene)
}

// SharingMode returns the current runtime sharing mode.
func (g *GeneStore) SharingMode() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return normalizeSharingMode(g.cfg.SharingMode)
}

// SetSharingMode updates the runtime sharing mode used for future writes and reads.
func (g *GeneStore) SetSharingMode(mode string, reason string) (SharingPolicySnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	normalized := normalizeSharingMode(mode)
	if mode != SharingDisabled && normalized != mode {
		return g.sharingPolicySnapshotLocked(), fmt.Errorf("evolution: invalid sharing mode %q", mode)
	}
	meta, err := g.policyPersistence.StoreGlobal(normalized, reason)
	if err != nil {
		return g.sharingPolicySnapshotLocked(), err
	}
	g.cfg.SharingMode = normalized
	g.sharingVersion = meta.Version
	return g.sharingPolicySnapshotLocked(), nil
}

// SetTenantSharingPolicy updates and persists a tenant-level sharing policy.
func (g *GeneStore) SetTenantSharingPolicy(policy TenantSharingPolicy, reason string) (EffectiveTenantSharingPolicy, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	policy.ContributionMode = normalizeSharingMode(policy.ContributionMode)
	meta, err := g.policyPersistence.StoreTenant(policy, reason)
	if err != nil {
		return g.effectiveTenantSharingPolicyLocked(policy.TenantID), err
	}
	if g.cfg.TenantPolicies == nil {
		g.cfg.TenantPolicies = make(map[string]TenantSharingPolicy)
	}
	if g.tenantPolicyVersion == nil {
		g.tenantPolicyVersion = make(map[string]int64)
	}
	g.cfg.TenantPolicies[policy.TenantID] = policy
	g.tenantPolicyVersion[policy.TenantID] = meta.Version
	return g.effectiveTenantSharingPolicyLocked(policy.TenantID), nil
}

// EffectiveTenantSharingPolicy resolves a tenant-level policy against the global sharing mode.
func (g *GeneStore) EffectiveTenantSharingPolicy(tenantID string) EffectiveTenantSharingPolicy {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.effectiveTenantSharingPolicyLocked(tenantID)
}

func (g *GeneStore) effectiveTenantSharingPolicyLocked(tenantID string) EffectiveTenantSharingPolicy {
	globalMode := normalizeSharingMode(g.cfg.SharingMode)
	policy, ok := g.cfg.TenantPolicies[tenantID]
	if !ok {
		return EffectiveTenantSharingPolicy{
			TenantID:                  tenantID,
			GlobalMode:                globalMode,
			ConsumeShared:             globalMode != SharingDisabled,
			ContributionMode:          globalMode,
			EffectiveContributionMode: globalMode,
		}
	}
	contributionMode := normalizeSharingMode(policy.ContributionMode)
	effectiveContributionMode := contributionMode
	if globalMode == SharingDisabled || contributionMode == SharingDisabled {
		effectiveContributionMode = SharingDisabled
	}
	if globalMode == SharingAnonymous && contributionMode == SharingTrusted {
		effectiveContributionMode = SharingAnonymous
	}
	return EffectiveTenantSharingPolicy{
		TenantID:                  tenantID,
		GlobalMode:                globalMode,
		ConsumeShared:             policy.ConsumeShared && globalMode != SharingDisabled,
		ContributionMode:          contributionMode,
		EffectiveContributionMode: effectiveContributionMode,
		Labels:                    append([]string(nil), policy.Labels...),
		Version:                   g.tenantPolicyVersion[tenantID],
	}
}

// SharingPolicySnapshot returns an auditable view of the current sharing policy.
func (g *GeneStore) SharingPolicySnapshot() SharingPolicySnapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.sharingPolicySnapshotLocked()
}

func (g *GeneStore) ListSharingPolicyHistory(limit int, offset int) ([]SharingPolicyHistoryEntry, error) {
	return g.policyPersistence.ListHistory(policyScopeGlobal, globalScopeID, limit, offset)
}

func (g *GeneStore) RollbackSharingPolicy(version int64, reason string) (SharingPolicySnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	mode, err := g.policyPersistence.LoadGlobalVersion(version)
	if err != nil {
		return g.sharingPolicySnapshotLocked(), err
	}
	if reason == "" {
		reason = fmt.Sprintf("rollback to version %d", version)
	}
	meta, err := g.policyPersistence.StoreGlobal(mode, reason)
	if err != nil {
		return g.sharingPolicySnapshotLocked(), err
	}
	g.cfg.SharingMode = mode
	g.sharingVersion = meta.Version
	return g.sharingPolicySnapshotLocked(), nil
}

func (g *GeneStore) ListTenantSharingPolicyHistory(tenantID string, limit int, offset int) ([]SharingPolicyHistoryEntry, error) {
	return g.policyPersistence.ListHistory(policyScopeTenant, tenantID, limit, offset)
}

func (g *GeneStore) RollbackTenantSharingPolicy(tenantID string, version int64, reason string) (EffectiveTenantSharingPolicy, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	policy, err := g.policyPersistence.LoadTenantVersion(tenantID, version)
	if err != nil {
		return g.effectiveTenantSharingPolicyLocked(tenantID), err
	}
	if reason == "" {
		reason = fmt.Sprintf("rollback to version %d", version)
	}
	meta, err := g.policyPersistence.StoreTenant(policy, reason)
	if err != nil {
		return g.effectiveTenantSharingPolicyLocked(tenantID), err
	}
	if g.cfg.TenantPolicies == nil {
		g.cfg.TenantPolicies = make(map[string]TenantSharingPolicy)
	}
	if g.tenantPolicyVersion == nil {
		g.tenantPolicyVersion = make(map[string]int64)
	}
	g.cfg.TenantPolicies[tenantID] = policy
	g.tenantPolicyVersion[tenantID] = meta.Version
	return g.effectiveTenantSharingPolicyLocked(tenantID), nil
}

func (g *GeneStore) sharingPolicySnapshotLocked() SharingPolicySnapshot {
	return SharingPolicySnapshot{
		Mode:         normalizeSharingMode(g.cfg.SharingMode),
		SharedPrefix: sharedTaskClassPrefix,
		Levels:       []string{SharingDisabled, SharingAnonymous, SharingTrusted},
		Version:      g.sharingVersion,
	}
}

// SharedRevokeCriteria scopes shared gene rollback.
type SharedRevokeCriteria struct {
	TaskClass    string
	SourceTenant string
	Source       string
	From         *time.Time
	To           *time.Time
	ConfirmAll   bool
}

// RevokeShared deletes shared genes matching the supplied criteria.
//
// Deletion is executed in bounded batches against the evolution backing store.
// This keeps memory usage stable for large revoke windows, but the operation is
// not atomic across the full match set: if interrupted, a suffix of matching
// genes may remain and can be revoked by rerunning the same criteria.
func (g *GeneStore) RevokeShared(ctx context.Context, criteria SharedRevokeCriteria) (int, error) {
	if !criteria.ConfirmAll && criteria.TaskClass == "" && criteria.SourceTenant == "" && criteria.Source == "" && criteria.From == nil && criteria.To == nil {
		return 0, fmt.Errorf("evolution: shared revoke requires at least one criterion or confirm_all=true")
	}
	if g.policyPersistence != nil {
		return g.policyPersistence.RevokeSharedGenes(ctx, criteria)
	}
	return 0, fmt.Errorf("evolution: policy persistence not configured")
}

// RecordOutcome increments use/success counts and refreshes the confidence score.
func (g *GeneStore) RecordOutcome(ctx context.Context, tenantID string, geneID string, success bool) error {
	if err := g.inner.UpdateStats(ctx, geneID, true, success); err != nil {
		return err
	}
	gene, err := g.inner.Get(ctx, geneID)
	if err != nil {
		return err
	}
	if gene == nil {
		return fmt.Errorf("evolution: gene %s not found", geneID)
	}
	if gene.UseCount > 0 {
		gene.Confidence = float64(gene.SuccessCount) / float64(gene.UseCount)
		return g.inner.Save(ctx, *gene) // inner.Save: gene.TaskClass already tenant-prefixed
	}
	return nil
}

// Close releases the underlying store connection.
func (g *GeneStore) Close() error {
	var errs []error
	if g.inner != nil {
		errs = append(errs, g.inner.Close())
	}
	if g.policyPersistence != nil {
		errs = append(errs, g.policyPersistence.Close())
	}
	return errors.Join(errs...)
}
