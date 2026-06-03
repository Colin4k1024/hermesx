package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Colin4k1024/hermesx/internal/objstore"
	"github.com/Colin4k1024/hermesx/internal/store"
)

const (
	defaultArchiveRetention time.Duration = 90 * 24 * time.Hour
	defaultArchiveInterval  time.Duration = 6 * time.Hour
	defaultArchiveBatchSize int           = 1000
	archiveBucketPrefix                   = "audit-archive/"
)

// AuditArchivalJob moves audit logs older than the retention window to object storage.
type AuditArchivalJob struct {
	auditLogs store.AuditLogStore
	objStore  objstore.ObjectStore
	retention time.Duration
	interval  time.Duration
	batchSize int
}

type ArchivalOption func(*AuditArchivalJob)

func WithArchiveRetention(d time.Duration) ArchivalOption {
	return func(j *AuditArchivalJob) { j.retention = d }
}

func WithArchiveInterval(d time.Duration) ArchivalOption {
	return func(j *AuditArchivalJob) { j.interval = d }
}

func WithArchiveBatchSize(n int) ArchivalOption {
	return func(j *AuditArchivalJob) { j.batchSize = n }
}

func NewAuditArchivalJob(auditLogs store.AuditLogStore, obj objstore.ObjectStore, opts ...ArchivalOption) *AuditArchivalJob {
	j := &AuditArchivalJob{
		auditLogs: auditLogs,
		objStore:  obj,
		retention: defaultArchiveRetention,
		interval:  defaultArchiveInterval,
		batchSize: defaultArchiveBatchSize,
	}
	for _, opt := range opts {
		opt(j)
	}
	return j
}

// Run starts the archival loop. It runs until the context is cancelled.
func (j *AuditArchivalJob) Run(ctx context.Context) {
	slog.Info("audit archival job started",
		"retention", j.retention.String(),
		"interval", j.interval.String(),
		"batch_size", j.batchSize,
	)

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	// Run once immediately on startup.
	j.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("audit archival job stopped")
			return
		case <-ticker.C:
			j.runOnce(ctx)
		}
	}
}

// RunOnce performs a single archival pass (exported for testing/manual trigger).
func (j *AuditArchivalJob) RunOnce(ctx context.Context) (totalArchived int64, err error) {
	return j.archivePass(ctx)
}

func (j *AuditArchivalJob) runOnce(ctx context.Context) {
	total, err := j.archivePass(ctx)
	if err != nil {
		slog.Error("audit archival pass failed", "error", err)
		return
	}
	if total > 0 {
		slog.Info("audit archival pass completed", "archived", total)
	}
}

func (j *AuditArchivalJob) archivePass(ctx context.Context) (int64, error) {
	cutoff := time.Now().UTC().Add(-j.retention)
	var totalArchived int64

	for {
		if ctx.Err() != nil {
			return totalArchived, ctx.Err()
		}

		batch, err := j.auditLogs.ArchiveOlderThan(ctx, cutoff, j.batchSize)
		if err != nil {
			return totalArchived, fmt.Errorf("fetch archive batch: %w", err)
		}
		if len(batch) == 0 {
			break
		}

		if err := j.writeBatchToObjectStore(ctx, batch); err != nil {
			return totalArchived, fmt.Errorf("write archive batch: %w", err)
		}

		totalArchived += int64(len(batch))

		if len(batch) < j.batchSize {
			break
		}
	}
	return totalArchived, nil
}

func (j *AuditArchivalJob) writeBatchToObjectStore(ctx context.Context, logs []*store.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}

	first := logs[0].CreatedAt
	last := logs[len(logs)-1].CreatedAt

	key := fmt.Sprintf("%s%s/%s_to_%s_%d.jsonl",
		archiveBucketPrefix,
		first.Format("2006/01/02"),
		first.Format("150405"),
		last.Format("150405"),
		len(logs),
	)

	data, err := marshalJSONLines(logs)
	if err != nil {
		return fmt.Errorf("marshal archive batch: %w", err)
	}

	if err := j.objStore.PutObject(ctx, key, data); err != nil {
		return fmt.Errorf("put archive object %s: %w", key, err)
	}

	slog.Debug("archived audit batch to object store",
		"key", key,
		"count", len(logs),
		"from", first.Format(time.RFC3339),
		"to", last.Format(time.RFC3339),
	)
	return nil
}

func marshalJSONLines(logs []*store.AuditLog) ([]byte, error) {
	var buf []byte
	for _, l := range logs {
		line, err := json.Marshal(l)
		if err != nil {
			return nil, err
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	return buf, nil
}
