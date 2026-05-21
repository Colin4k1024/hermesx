package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

type myAgentCheckpointStore struct {
	db *sql.DB
}

func (s *myAgentCheckpointStore) Get(ctx context.Context, tenantID, sessionID, checkpointID string) (*store.AgentCheckpoint, error) {
	var cp store.AgentCheckpoint
	err := s.db.QueryRowContext(ctx, `
SELECT tenant_id, session_id, checkpoint_id, payload, updated_at
FROM agent_checkpoints
WHERE tenant_id=? AND session_id=? AND checkpoint_id=?`,
		tenantID, sessionID, checkpointID).Scan(&cp.TenantID, &cp.SessionID, &cp.CheckpointID, &cp.Payload, &cp.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("mysql get agent checkpoint: %w", err)
	}
	return &cp, nil
}

func (s *myAgentCheckpointStore) Set(ctx context.Context, cp *store.AgentCheckpoint) error {
	if cp == nil {
		return fmt.Errorf("mysql set agent checkpoint: checkpoint is nil")
	}
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO agent_checkpoints (tenant_id, session_id, checkpoint_id, payload, updated_at)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE payload=VALUES(payload), updated_at=VALUES(updated_at)`,
		cp.TenantID, cp.SessionID, cp.CheckpointID, cp.Payload, cp.UpdatedAt)
	if err != nil {
		return fmt.Errorf("mysql set agent checkpoint: %w", err)
	}
	return nil
}

func (s *myAgentCheckpointStore) Delete(ctx context.Context, tenantID, sessionID, checkpointID string) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM agent_checkpoints
WHERE tenant_id=? AND session_id=? AND checkpoint_id=?`,
		tenantID, sessionID, checkpointID)
	if err != nil {
		return fmt.Errorf("mysql delete agent checkpoint: %w", err)
	}
	return nil
}

var _ store.AgentCheckpointStore = (*myAgentCheckpointStore)(nil)
