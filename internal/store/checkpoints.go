package store

import (
	"context"
	"strings"
)

// EinoCheckPointStore adapts tenant-scoped AgentCheckpointStore to Eino ADK's
// string-key checkpoint interface. IDs use tenantID/sessionID[/checkpointID].
type EinoCheckPointStore struct {
	inner AgentCheckpointStore
}

func NewEinoCheckPointStore(inner AgentCheckpointStore) *EinoCheckPointStore {
	if inner == nil {
		return nil
	}
	return &EinoCheckPointStore{inner: inner}
}

func (s *EinoCheckPointStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	tenantID, sessionID, id := splitCheckpointID(checkPointID)
	cp, err := s.inner.Get(ctx, tenantID, sessionID, id)
	if err != nil {
		if err == ErrNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	if cp == nil {
		return nil, false, nil
	}
	return cp.Payload, true, nil
}

func (s *EinoCheckPointStore) Set(ctx context.Context, checkPointID string, payload []byte) error {
	tenantID, sessionID, id := splitCheckpointID(checkPointID)
	return s.inner.Set(ctx, &AgentCheckpoint{
		TenantID:     tenantID,
		SessionID:    sessionID,
		CheckpointID: id,
		Payload:      payload,
	})
}

func (s *EinoCheckPointStore) Delete(ctx context.Context, checkPointID string) error {
	tenantID, sessionID, id := splitCheckpointID(checkPointID)
	return s.inner.Delete(ctx, tenantID, sessionID, id)
}

func splitCheckpointID(checkPointID string) (tenantID, sessionID, id string) {
	parts := strings.Split(checkPointID, "/")
	if len(parts) >= 2 {
		tenantID = parts[0]
		sessionID = parts[1]
		if len(parts) > 2 {
			id = strings.Join(parts[2:], "/")
		}
	}
	if tenantID == "" {
		tenantID = "default"
	}
	if sessionID == "" {
		sessionID = "default"
	}
	if id == "" {
		id = checkPointID
	}
	return tenantID, sessionID, id
}
