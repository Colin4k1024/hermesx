package eino

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

type inMemoryTurnLoopCheckpointStore struct {
	mu      sync.Mutex
	entries map[string][]byte
}

func newInMemoryTurnLoopCheckpointStore() *inMemoryTurnLoopCheckpointStore {
	return &inMemoryTurnLoopCheckpointStore{entries: make(map[string][]byte)}
}

func (s *inMemoryTurnLoopCheckpointStore) Get(_ context.Context, checkPointID string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.entries[checkPointID]
	if !ok {
		return nil, false, nil
	}
	out := append([]byte(nil), data...)
	return out, true, nil
}

func (s *inMemoryTurnLoopCheckpointStore) Set(_ context.Context, checkPointID string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[checkPointID] = append([]byte(nil), data...)
	return nil
}

func (s *inMemoryTurnLoopCheckpointStore) Delete(_ context.Context, checkPointID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, checkPointID)
	return nil
}

func (s *inMemoryTurnLoopCheckpointStore) Snapshot(checkPointID string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.entries[checkPointID]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), data...), true
}

var _ adk.CheckPointStore = (*inMemoryTurnLoopCheckpointStore)(nil)

type cancelOnFirstCallTransport struct {
	mu          sync.Mutex
	callCount   int
	firstCallCh chan struct{}
	firstCallMu sync.Once
}

func newCancelOnFirstCallTransport() *cancelOnFirstCallTransport {
	return &cancelOnFirstCallTransport{firstCallCh: make(chan struct{})}
}

func (t *cancelOnFirstCallTransport) Name() string { return "cancel-on-first-call" }

func (t *cancelOnFirstCallTransport) Chat(ctx context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	t.mu.Lock()
	t.callCount++
	call := t.callCount
	firstCallCh := t.firstCallCh
	t.mu.Unlock()

	if call == 1 {
		t.firstCallMu.Do(func() { close(firstCallCh) })
		<-ctx.Done()
		return nil, ctx.Err()
	}

	return &llm.ChatResponse{Content: "resumed answer", FinishReason: "stop"}, nil
}

func (t *cancelOnFirstCallTransport) ChatStream(_ context.Context, _ llm.ChatRequest) (<-chan llm.StreamDelta, <-chan error) {
	panic("ChatStream not implemented in test transport")
}

func (t *cancelOnFirstCallTransport) CallCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.callCount
}

func encodeTurnLoopCheckpointEnvelope(t *testing.T, envelope turnLoopCheckpointEnvelope) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(envelope); err != nil {
		t.Fatalf("encode checkpoint envelope: %v", err)
	}
	return buf.Bytes()
}

func TestPrepareTurnLoopCheckpoint_ClearsStaleOrPreemptedState(t *testing.T) {
	testCases := []struct {
		name        string
		envelope    turnLoopCheckpointEnvelope
		currentItem turnLoopItem
		wantKeep    bool
	}{
		{
			name: "keep matching interrupted item",
			envelope: turnLoopCheckpointEnvelope{
				RunnerCheckpoint: []byte("runner"),
				HasRunnerState:   true,
				CanceledItems:    []turnLoopItem{{UserMessage: "resume me"}},
			},
			currentItem: turnLoopItem{UserMessage: "resume me"},
			wantKeep:    true,
		},
		{
			name: "drop preempted interrupted item",
			envelope: turnLoopCheckpointEnvelope{
				RunnerCheckpoint: []byte("runner"),
				HasRunnerState:   true,
				CanceledItems:    []turnLoopItem{{UserMessage: "old request"}},
			},
			currentItem: turnLoopItem{UserMessage: "new request"},
			wantKeep:    false,
		},
		{
			name: "drop stale buffered item",
			envelope: turnLoopCheckpointEnvelope{
				UnhandledItems: []turnLoopItem{{UserMessage: "stale request"}},
			},
			currentItem: turnLoopItem{UserMessage: "fresh request"},
			wantKeep:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := newInMemoryTurnLoopCheckpointStore()
			checkpointID := "tenant-a/session-a"
			if err := store.Set(context.Background(), checkpointID, encodeTurnLoopCheckpointEnvelope(t, tc.envelope)); err != nil {
				t.Fatalf("seed checkpoint: %v", err)
			}

			cfg := &agentConfig{
				tenantID:        "tenant-a",
				sessionID:       "session-a",
				checkpointStore: store,
			}
			if err := prepareTurnLoopCheckpoint(context.Background(), cfg, tc.currentItem); err != nil {
				t.Fatalf("prepareTurnLoopCheckpoint: %v", err)
			}

			_, ok := store.Snapshot(checkpointID)
			if ok != tc.wantKeep {
				t.Fatalf("expected checkpoint keep=%t, got keep=%t", tc.wantKeep, ok)
			}
		})
	}
}

func TestRunConversationTurnLoopSafe_RequestCancelPersistsResumableCheckpoint(t *testing.T) {
	store := newInMemoryTurnLoopCheckpointStore()
	transport := newCancelOnFirstCallTransport()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	type runResult struct {
		result *ConversationResult
		err    error
	}
	firstRun := make(chan runResult, 1)
	go func() {
		result, err := RunConversationTurnLoopSafe(
			ctx,
			"resume me",
			nil,
			nil,
			WithTransport(transport),
			WithModel("test-model"),
			WithTenantID("tenant-a"),
			WithSessionID("session-a"),
			WithCheckpointStore(store),
		)
		firstRun <- runResult{result: result, err: err}
	}()

	select {
	case <-transport.firstCallCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first model call to start")
	}

	cancel()

	var interrupted runResult
	select {
	case interrupted = <-firstRun:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for interrupted run to exit")
	}
	if interrupted.err == nil {
		t.Fatal("expected interrupted run to return an error")
	}
	if interrupted.result == nil || !interrupted.result.Interrupted {
		t.Fatalf("expected interrupted result, got %#v", interrupted.result)
	}

	payload, ok := store.Snapshot("tenant-a/session-a")
	if !ok {
		t.Fatal("expected interrupted run to persist a checkpoint")
	}

	envelope, err := decodeTurnLoopCheckpoint(payload)
	if err != nil {
		t.Fatalf("decode checkpoint: %v", err)
	}
	if !envelope.HasRunnerState {
		t.Fatalf("expected resumable runner state, got %#v", envelope)
	}
	if len(envelope.CanceledItems) != 1 || envelope.CanceledItems[0].UserMessage != "resume me" {
		t.Fatalf("expected canceled item to match interrupted request, got %#v", envelope.CanceledItems)
	}

	resumed, err := RunConversationTurnLoopSafe(
		context.Background(),
		"resume me",
		nil,
		nil,
		WithTransport(transport),
		WithModel("test-model"),
		WithTenantID("tenant-a"),
		WithSessionID("session-a"),
		WithCheckpointStore(store),
	)
	if err != nil {
		var cancelErr *adk.CancelError
		if errors.As(err, &cancelErr) {
			t.Fatalf("expected resume run to complete, got cancel error: %v", err)
		}
		t.Fatalf("resume run failed: %v", err)
	}
	if resumed.FinalResponse != "resumed answer" {
		t.Fatalf("expected resumed final response, got %q", resumed.FinalResponse)
	}
	if resumed.Interrupted {
		t.Fatalf("expected resumed run to complete, got %#v", resumed)
	}
	if _, ok := store.Snapshot("tenant-a/session-a"); ok {
		t.Fatal("expected resume run to clear checkpoint after completion")
	}
	if got := transport.CallCount(); got != 2 {
		t.Fatalf("expected 2 model calls across interrupt/resume, got %d", got)
	}
}
