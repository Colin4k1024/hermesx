package store

import (
	"context"
	"testing"
	"time"
)

type fakeAgentCheckpointStore struct {
	entries map[string]*AgentCheckpoint
}

func (f *fakeAgentCheckpointStore) Get(_ context.Context, tenantID, sessionID, checkpointID string) (*AgentCheckpoint, error) {
	cp, ok := f.entries[tenantID+"/"+sessionID+"/"+checkpointID]
	if !ok {
		return nil, ErrNotFound
	}
	clone := *cp
	return &clone, nil
}

func (f *fakeAgentCheckpointStore) Set(_ context.Context, checkpoint *AgentCheckpoint) error {
	if f.entries == nil {
		f.entries = map[string]*AgentCheckpoint{}
	}
	clone := *checkpoint
	f.entries[checkpoint.TenantID+"/"+checkpoint.SessionID+"/"+checkpoint.CheckpointID] = &clone
	return nil
}

func (f *fakeAgentCheckpointStore) Delete(_ context.Context, tenantID, sessionID, checkpointID string) error {
	delete(f.entries, tenantID+"/"+sessionID+"/"+checkpointID)
	return nil
}

func TestEinoCheckPointStore_RoundTrip(t *testing.T) {
	inner := &fakeAgentCheckpointStore{}
	store := NewEinoCheckPointStore(inner)
	ctx := context.Background()
	payload := []byte("checkpoint-payload")

	if err := store.Set(ctx, "tenant-a/session-b/turn-1", payload); err != nil {
		t.Fatalf("Set: %v", err)
	}

	cp, err := inner.Get(ctx, "tenant-a", "session-b", "turn-1")
	if err != nil {
		t.Fatalf("inner.Get: %v", err)
	}
	if string(cp.Payload) != string(payload) {
		t.Fatalf("stored payload = %q, want %q", cp.Payload, payload)
	}
	if cp.UpdatedAt.After(time.Now().Add(5 * time.Second)) {
		t.Fatalf("unexpected UpdatedAt in future: %v", cp.UpdatedAt)
	}

	got, ok, err := store.Get(ctx, "tenant-a/session-b/turn-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get reported checkpoint missing")
	}
	if string(got) != string(payload) {
		t.Fatalf("Get payload = %q, want %q", got, payload)
	}

	if err := store.Delete(ctx, "tenant-a/session-b/turn-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, err := store.Get(ctx, "tenant-a/session-b/turn-1"); err != nil {
		t.Fatalf("Get after delete: %v", err)
	} else if ok {
		t.Fatal("expected checkpoint to be deleted")
	}
}

func TestSplitCheckpointID(t *testing.T) {
	tests := []struct {
		name        string
		checkpoint  string
		wantTenant  string
		wantSession string
		wantID      string
	}{
		{name: "full path", checkpoint: "tenant-a/session-b/turn-1", wantTenant: "tenant-a", wantSession: "session-b", wantID: "turn-1"},
		{name: "two segments keeps full id", checkpoint: "tenant-a/session-b", wantTenant: "tenant-a", wantSession: "session-b", wantID: "tenant-a/session-b"},
		{name: "fallback defaults", checkpoint: "checkpoint-only", wantTenant: "default", wantSession: "default", wantID: "checkpoint-only"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantID, sessionID, checkpointID := splitCheckpointID(tt.checkpoint)
			if tenantID != tt.wantTenant || sessionID != tt.wantSession || checkpointID != tt.wantID {
				t.Fatalf("splitCheckpointID(%q) = (%q, %q, %q), want (%q, %q, %q)", tt.checkpoint, tenantID, sessionID, checkpointID, tt.wantTenant, tt.wantSession, tt.wantID)
			}
		})
	}
}
