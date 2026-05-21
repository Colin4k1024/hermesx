package pg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgAgentCheckpointStore struct {
	pool *pgxpool.Pool
}

func (s *pgAgentCheckpointStore) Get(ctx context.Context, tenantID, sessionID, checkpointID string) (*store.AgentCheckpoint, error) {
	var cp store.AgentCheckpoint
	err := s.pool.QueryRow(ctx, `
SELECT tenant_id, session_id, checkpoint_id, payload, updated_at
FROM agent_checkpoints
WHERE tenant_id=$1 AND session_id=$2 AND checkpoint_id=$3`,
		tenantID, sessionID, checkpointID).Scan(&cp.TenantID, &cp.SessionID, &cp.CheckpointID, &cp.Payload, &cp.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("pg get agent checkpoint: %w", err)
	}
	return &cp, nil
}

func (s *pgAgentCheckpointStore) Set(ctx context.Context, cp *store.AgentCheckpoint) error {
	if cp == nil {
		return fmt.Errorf("pg set agent checkpoint: checkpoint is nil")
	}
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = time.Now()
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO agent_checkpoints (tenant_id, session_id, checkpoint_id, payload, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (tenant_id, session_id, checkpoint_id)
DO UPDATE SET payload=EXCLUDED.payload, updated_at=EXCLUDED.updated_at`,
		cp.TenantID, cp.SessionID, cp.CheckpointID, cp.Payload, cp.UpdatedAt)
	if err != nil {
		return fmt.Errorf("pg set agent checkpoint: %w", err)
	}
	return nil
}

func (s *pgAgentCheckpointStore) Delete(ctx context.Context, tenantID, sessionID, checkpointID string) error {
	_, err := s.pool.Exec(ctx, `
DELETE FROM agent_checkpoints
WHERE tenant_id=$1 AND session_id=$2 AND checkpoint_id=$3`,
		tenantID, sessionID, checkpointID)
	if err != nil {
		return fmt.Errorf("pg delete agent checkpoint: %w", err)
	}
	return nil
}

var _ store.AgentCheckpointStore = (*pgAgentCheckpointStore)(nil)
