package evolution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
	inner orisstore.Store
	cfg   Config
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
		return &GeneStore{inner: s, cfg: cfg}, nil
	}

	dbPath := cfg.DBPath
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("evolution: resolve home dir: %w", err)
		}
		dbPath = filepath.Join(home, ".hermes", "evolution.db")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("evolution: create db dir: %w", err)
	}

	s, err := orisstore.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("evolution: open sqlite: %w", err)
	}
	return &GeneStore{inner: s, cfg: cfg}, nil
}

// QueryTop returns up to limit genes for taskClass with confidence ≥ minConfidence.
// tenantID namespaces the query to prevent cross-tenant gene leakage (B2).
func (g *GeneStore) QueryTop(ctx context.Context, tenantID string, taskClass TaskClass, minConfidence float64, limit int) ([]orisstore.Gene, error) {
	return g.inner.Query(ctx, orisstore.StoreQuery{
		TaskClass:     tenantTaskClass(tenantID, taskClass),
		MinConfidence: minConfidence,
		OrderBy:       safeOrderBy("confidence_desc"),
		Limit:         limit,
	})
}

// Save upserts a gene under the tenant namespace (B2).
func (g *GeneStore) Save(ctx context.Context, tenantID string, gene orisstore.Gene) error {
	gene.TaskClass = tenantTaskClass(tenantID, gene.TaskClass)
	return g.inner.Save(ctx, gene)
}

// RecordOutcome increments use/success counts and refreshes the confidence score.
func (g *GeneStore) RecordOutcome(ctx context.Context, tenantID string, geneID string, success bool) error {
	if err := g.inner.UpdateStats(ctx, geneID, true, success); err != nil {
		return err
	}
	gene, err := g.inner.Get(ctx, geneID)
	if err != nil || gene == nil {
		return err
	}
	if gene.UseCount > 0 {
		gene.Confidence = float64(gene.SuccessCount) / float64(gene.UseCount)
		return g.inner.Save(ctx, *gene) // inner.Save: gene.TaskClass already tenant-prefixed
	}
	return nil
}

// Close releases the underlying store connection.
func (g *GeneStore) Close() error {
	return g.inner.Close()
}
