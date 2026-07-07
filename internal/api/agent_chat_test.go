package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/eino"
	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/store"
)

func TestServeAgentHTTP_IncludeAgenticBlocksOptIn(t *testing.T) {
	blocks := []eino.AgenticBlock{{
		Type: "server_tool_call",
		Data: map[string]any{"server_tool_call": map[string]any{"name": "web_search"}},
	}}
	h := NewChatHandler(stubStore{}, nil, nil)
	h.llmModel = "test-model"
	h.runAgent = func(_ context.Context, _ string, _ []llm.Message, _ *eino.StreamCallbacks) (*eino.ConversationResult, error) {
		return &eino.ConversationResult{FinalResponse: "done", AgenticBlocks: blocks}, nil
	}

	t.Run("omits blocks by default", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"hi"}]}`)

		h.ServeAgentHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if _, ok := body["agentic_blocks"]; ok {
			t.Fatal("expected agentic_blocks to be omitted when include_agentic_blocks is false")
		}
	})

	t.Run("returns blocks when opted in", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"hi"}],"include_agentic_blocks":true}`)

		h.ServeAgentHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body struct {
			AgenticBlocks []eino.AgenticBlock `json:"agentic_blocks"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(body.AgenticBlocks) != 1 {
			t.Fatalf("agentic_blocks len = %d, want 1", len(body.AgenticBlocks))
		}
		if body.AgenticBlocks[0].Type != "server_tool_call" {
			t.Fatalf("agentic_blocks[0].type = %q, want server_tool_call", body.AgenticBlocks[0].Type)
		}
	})
}

func TestServeAgentHTTP_StreamAgenticBlockEvent(t *testing.T) {
	block := eino.AgenticBlock{
		Type: "server_tool_call",
		Data: map[string]any{"server_tool_call": map[string]any{"name": "web_search"}},
	}
	h := NewChatHandler(stubStore{}, nil, nil)
	h.llmModel = "test-model"
	h.runAgent = func(_ context.Context, _ string, _ []llm.Message, callbacks *eino.StreamCallbacks) (*eino.ConversationResult, error) {
		if callbacks != nil {
			callbacks.OnStreamDelta("partial")
			callbacks.OnAgenticBlock(block)
		}
		return &eino.ConversationResult{FinalResponse: "done", AgenticBlocks: []eino.AgenticBlock{block}}, nil
	}

	t.Run("emits agentic block event when opted in", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"hi"}],"stream":true,"include_agentic_blocks":true}`)

		h.ServeAgentHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "event: agentic_block") {
			t.Fatalf("expected SSE body to contain agentic_block event, got %q", body)
		}
		if !strings.Contains(body, `"type":"server_tool_call"`) {
			t.Fatalf("expected SSE body to contain serialized block payload, got %q", body)
		}
	})

	t.Run("hides agentic block event by default", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"hi"}],"stream":true}`)

		h.ServeAgentHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, "event: agentic_block") {
			t.Fatalf("expected SSE body to hide agentic_block event by default, got %q", body)
		}
	})
}

func TestServeAgentHTTP_ResumeDoesNotDuplicateInterruptedUserTurn(t *testing.T) {
	fake := newAgentChatFakeStore()
	h := NewChatHandler(fake, nil, nil)
	h.llmModel = "test-model"

	call := 0
	h.runAgent = func(ctx context.Context, userMessage string, history []llm.Message, _ *eino.StreamCallbacks) (*eino.ConversationResult, error) {
		call++
		if userMessage != "resume me" {
			t.Fatalf("userMessage = %q, want resume me", userMessage)
		}
		if len(history) != 0 {
			t.Fatalf("history len = %d, want 0 before interrupted turn is persisted", len(history))
		}
		if call == 1 {
			err := fake.AgentCheckpoints().Set(ctx, &store.AgentCheckpoint{
				TenantID:     "tenant-1",
				SessionID:    "resume-session",
				CheckpointID: "tenant-1/resume-session",
				Payload:      []byte("resumable-state"),
			})
			if err != nil {
				t.Fatalf("seed checkpoint: %v", err)
			}
			return nil, errors.New("request canceled")
		}
		if err := fake.AgentCheckpoints().Delete(ctx, "tenant-1", "resume-session", "tenant-1/resume-session"); err != nil {
			t.Fatalf("delete checkpoint: %v", err)
		}
		return &eino.ConversationResult{FinalResponse: "resumed answer", InputTokens: 3, OutputTokens: 4, TotalTokens: 7}, nil
	}

	first := httptest.NewRecorder()
	firstReq := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"resume me"}]}`)
	firstReq.Header.Set("X-Hermes-Session-Id", "resume-session")
	h.ServeAgentHTTP(first, firstReq)
	if first.Code != http.StatusBadGateway {
		t.Fatalf("first status = %d, want 502", first.Code)
	}
	if got := fake.messages.CountRole("user"); got != 0 {
		t.Fatalf("interrupted request persisted %d user messages, want 0", got)
	}
	if _, err := fake.AgentCheckpoints().Get(context.Background(), "tenant-1", "resume-session", "tenant-1/resume-session"); err != nil {
		t.Fatalf("expected checkpoint after interrupted request: %v", err)
	}

	second := httptest.NewRecorder()
	secondReq := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"resume me"}]}`)
	secondReq.Header.Set("X-Hermes-Session-Id", "resume-session")
	h.ServeAgentHTTP(second, secondReq)
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200 body=%q", second.Code, second.Body.String())
	}
	if got := second.Header().Get("X-Hermes-Session-Id"); got != "resume-session" {
		t.Fatalf("session header = %q, want resume-session", got)
	}
	if _, err := fake.AgentCheckpoints().Get(context.Background(), "tenant-1", "resume-session", "tenant-1/resume-session"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected checkpoint to be consumed, got err=%v", err)
	}
	msgs := fake.messages.Snapshot()
	if len(msgs) != 2 {
		t.Fatalf("message count = %d, want completed user+assistant turn: %#v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Content != "resume me" || msgs[1].Role != "assistant" || msgs[1].Content != "resumed answer" {
		t.Fatalf("unexpected persisted messages: %#v", msgs)
	}
	if got := fake.messages.CountRole("user"); got != 1 {
		t.Fatalf("user message count = %d, want 1", got)
	}
}

func TestServeAgentHTTP_ErrorPathsDoNotPersistDirtyState(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		setup      func(*agentChatFakeStore, *chatHandler)
		wantStatus int
	}{
		{
			name:       "invalid JSON",
			body:       `{"messages":[`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing user message",
			body:       `{"messages":[{"role":"assistant","content":"hi"}]}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "foreign session",
			body: `{"messages":[{"role":"user","content":"hi"}]}`,
			setup: func(fake *agentChatFakeStore, _ *chatHandler) {
				fake.sessions.sessions["fixed-session"] = &store.Session{ID: "fixed-session", TenantID: "tenant-1", UserID: "other-user"}
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "session create failure",
			body: `{"messages":[{"role":"user","content":"hi"}]}`,
			setup: func(fake *agentChatFakeStore, _ *chatHandler) {
				fake.sessions.createErr = errors.New("db down")
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "runAgent failure",
			body: `{"messages":[{"role":"user","content":"hi"}]}`,
			setup: func(_ *agentChatFakeStore, h *chatHandler) {
				h.runAgent = func(context.Context, string, []llm.Message, *eino.StreamCallbacks) (*eino.ConversationResult, error) {
					return nil, errors.New("model down")
				}
			},
			wantStatus: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newAgentChatFakeStore()
			h := NewChatHandler(fake, nil, nil)
			h.llmModel = "test-model"
			if h.runAgent == nil {
				h.runAgent = func(context.Context, string, []llm.Message, *eino.StreamCallbacks) (*eino.ConversationResult, error) {
					return &eino.ConversationResult{FinalResponse: "ok"}, nil
				}
			}
			if tt.setup != nil {
				tt.setup(fake, h)
			}

			rec := httptest.NewRecorder()
			req := newAgentChatRequest(t, tt.body)
			req.Header.Set("X-Hermes-Session-Id", "fixed-session")
			h.ServeAgentHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d body=%q", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := len(fake.messages.Snapshot()); got != 0 {
				t.Fatalf("persisted %d messages on failure, want 0", got)
			}
			if fake.sessions.TokenUpdates() != 0 {
				t.Fatalf("updated tokens on failure, want no token updates")
			}
		})
	}
}

func TestServeAgentHTTP_StreamErrorEventDoesNotPersistAssistantTurn(t *testing.T) {
	fake := newAgentChatFakeStore()
	h := NewChatHandler(fake, nil, nil)
	h.llmModel = "test-model"
	h.runAgent = func(_ context.Context, _ string, _ []llm.Message, callbacks *eino.StreamCallbacks) (*eino.ConversationResult, error) {
		if callbacks != nil {
			callbacks.OnStreamDelta("partial")
		}
		return nil, errors.New("stream failed")
	}

	rec := httptest.NewRecorder()
	req := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"hi"}],"stream":true}`)
	req.Header.Set("X-Hermes-Session-Id", "stream-session")
	h.ServeAgentHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Fatalf("expected error event, got %q", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("expected DONE marker, got %q", body)
	}
	if strings.Contains(body, `"finish_reason":"stop"`) {
		t.Fatalf("stream error emitted stop finish_reason: %q", body)
	}
	if got := len(fake.messages.Snapshot()); got != 0 {
		t.Fatalf("persisted %d messages on stream failure, want 0", got)
	}
	if fake.sessions.TokenUpdates() != 0 {
		t.Fatalf("updated tokens on stream failure, want no token updates")
	}
}

func TestServeAgentHTTP_StreamPersistsExtractedMemories(t *testing.T) {
	fake := newAgentChatFakeStore()
	h := NewChatHandler(fake, nil, nil)
	h.llmModel = "test-model"
	h.runAgent = func(_ context.Context, _ string, _ []llm.Message, callbacks *eino.StreamCallbacks) (*eino.ConversationResult, error) {
		if callbacks != nil {
			callbacks.OnStreamDelta("ok")
		}
		return &eino.ConversationResult{FinalResponse: "ok"}, nil
	}

	rec := httptest.NewRecorder()
	req := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"请记住：我最喜欢的水果是芒果"}],"stream":true}`)
	req.Header.Set("X-Hermes-Session-Id", "memory-stream-session")
	h.ServeAgentHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%q", rec.Code, rec.Body.String())
	}
	memories, err := fake.memories.List(context.Background(), "tenant-1", "user-1")
	if err != nil {
		t.Fatalf("list memories: %v", err)
	}
	if len(memories) == 0 {
		t.Fatal("expected streaming request to persist extracted memories")
	}
	found := false
	for _, memory := range memories {
		if strings.Contains(memory.Content, "芒果") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected persisted memory to contain 芒果, got %#v", memories)
	}
}

func TestServeAgentHTTP_SerializesConcurrentRequestsForSameSession(t *testing.T) {
	fake := newAgentChatFakeStore()
	fake.sessions.sessions["shared-session"] = &store.Session{ID: "shared-session", TenantID: "tenant-1", UserID: "user-1"}
	h := NewChatHandler(fake, nil, nil)
	h.llmModel = "test-model"

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	var mu sync.Mutex
	call := 0
	maxActive := 0
	active := 0
	secondHistoryLen := -1

	h.runAgent = func(_ context.Context, userMessage string, history []llm.Message, _ *eino.StreamCallbacks) (*eino.ConversationResult, error) {
		mu.Lock()
		call++
		currentCall := call
		active++
		if active > maxActive {
			maxActive = active
		}
		if currentCall == 2 {
			secondHistoryLen = len(history)
		}
		mu.Unlock()

		if currentCall == 1 {
			close(firstEntered)
			<-releaseFirst
		}

		mu.Lock()
		active--
		mu.Unlock()
		return &eino.ConversationResult{FinalResponse: "reply to " + userMessage, InputTokens: 1, OutputTokens: 1, TotalTokens: 2}, nil
	}

	firstDone := make(chan int, 1)
	secondDone := make(chan int, 1)
	go func() {
		rec := httptest.NewRecorder()
		req := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"first"}]}`)
		req.Header.Set("X-Hermes-Session-Id", "shared-session")
		h.ServeAgentHTTP(rec, req)
		firstDone <- rec.Code
	}()

	select {
	case <-firstEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first request to enter runner")
	}

	go func() {
		rec := httptest.NewRecorder()
		req := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"second"}]}`)
		req.Header.Set("X-Hermes-Session-Id", "shared-session")
		h.ServeAgentHTTP(rec, req)
		secondDone <- rec.Code
	}()

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	callsWhileFirstBlocked := call
	mu.Unlock()
	if callsWhileFirstBlocked != 1 {
		t.Fatalf("second request entered runner before first completed; call count = %d", callsWhileFirstBlocked)
	}
	close(releaseFirst)

	if code := <-firstDone; code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", code)
	}
	if code := <-secondDone; code != http.StatusOK {
		t.Fatalf("second status = %d, want 200", code)
	}
	if maxActive != 1 {
		t.Fatalf("max active runners = %d, want 1", maxActive)
	}
	if secondHistoryLen != 2 {
		t.Fatalf("second history len = %d, want first completed user+assistant turn", secondHistoryLen)
	}
	msgs := fake.messages.Snapshot()
	if len(msgs) != 4 {
		t.Fatalf("message count = %d, want 4: %#v", len(msgs), msgs)
	}
}

func newAgentChatRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/chat", strings.NewReader(body))
	ctx := auth.WithContext(req.Context(), &auth.AuthContext{TenantID: "tenant-1", Identity: "user-1"})
	return req.WithContext(ctx)
}

type agentChatFakeStore struct {
	stubStore
	sessions    *agentChatSessionStore
	messages    *agentChatMessageStore
	memories    *agentChatMemoryStore
	checkpoints *agentChatCheckpointStore
}

func newAgentChatFakeStore() *agentChatFakeStore {
	return &agentChatFakeStore{
		sessions:    &agentChatSessionStore{sessions: map[string]*store.Session{}},
		messages:    &agentChatMessageStore{},
		memories:    &agentChatMemoryStore{entries: map[string]store.MemoryEntry{}},
		checkpoints: &agentChatCheckpointStore{entries: map[string]*store.AgentCheckpoint{}},
	}
}

func (s *agentChatFakeStore) Sessions() store.SessionStore { return s.sessions }
func (s *agentChatFakeStore) Messages() store.MessageStore { return s.messages }
func (s *agentChatFakeStore) Memories() store.MemoryStore  { return s.memories }
func (s *agentChatFakeStore) AgentCheckpoints() store.AgentCheckpointStore {
	return s.checkpoints
}

type agentChatSessionStore struct {
	mu           sync.Mutex
	sessions     map[string]*store.Session
	createErr    error
	tokenUpdates int
}

func (s *agentChatSessionStore) Create(_ context.Context, _ string, sess *store.Session) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := *sess
	s.sessions[sess.ID] = &clone
	return nil
}

func (s *agentChatSessionStore) Get(_ context.Context, _ string, sessionID string) (*store.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, store.ErrNotFound
	}
	clone := *sess
	return &clone, nil
}

func (s *agentChatSessionStore) End(_ context.Context, _, _, _ string) error { return nil }
func (s *agentChatSessionStore) List(_ context.Context, _ string, _ store.ListOptions) ([]*store.Session, int, error) {
	return nil, 0, nil
}
func (s *agentChatSessionStore) Delete(_ context.Context, _, _ string) error { return nil }
func (s *agentChatSessionStore) UpdateTokens(_ context.Context, _, _ string, _ store.TokenDelta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokenUpdates++
	return nil
}
func (s *agentChatSessionStore) SetTitle(_ context.Context, _, sessionID, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[sessionID]; ok {
		sess.Title = title
	}
	return nil
}
func (s *agentChatSessionStore) TokenUpdates() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokenUpdates
}

type agentChatMessageStore struct {
	mu       sync.Mutex
	nextID   int64
	messages []*store.Message
}

func (s *agentChatMessageStore) Append(_ context.Context, tenantID, sessionID string, msg *store.Message) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	clone := *msg
	clone.ID = s.nextID
	clone.TenantID = tenantID
	clone.SessionID = sessionID
	s.messages = append(s.messages, &clone)
	return clone.ID, nil
}

func (s *agentChatMessageStore) List(_ context.Context, tenantID, sessionID string, limit, offset int) ([]*store.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*store.Message, 0, len(s.messages))
	for _, msg := range s.messages {
		if msg.TenantID != tenantID || msg.SessionID != sessionID {
			continue
		}
		clone := *msg
		out = append(out, &clone)
	}
	if offset > len(out) {
		return nil, nil
	}
	out = out[offset:]
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *agentChatMessageStore) Search(_ context.Context, _, _ string, _ int) ([]*store.SearchResult, error) {
	return nil, nil
}
func (s *agentChatMessageStore) CountBySession(_ context.Context, tenantID, sessionID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, msg := range s.messages {
		if msg.TenantID == tenantID && msg.SessionID == sessionID {
			count++
		}
	}
	return count, nil
}
func (s *agentChatMessageStore) Snapshot() []*store.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*store.Message, 0, len(s.messages))
	for _, msg := range s.messages {
		clone := *msg
		out = append(out, &clone)
	}
	return out
}
func (s *agentChatMessageStore) CountRole(role string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, msg := range s.messages {
		if msg.Role == role {
			count++
		}
	}
	return count
}

type agentChatMemoryStore struct {
	mu      sync.Mutex
	entries map[string]store.MemoryEntry
}

func (s *agentChatMemoryStore) Get(_ context.Context, tenantID, userID, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[tenantID+"/"+userID+"/"+key]
	if !ok {
		return "", store.ErrNotFound
	}
	return entry.Content, nil
}

func (s *agentChatMemoryStore) List(_ context.Context, tenantID, userID string) ([]store.MemoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]store.MemoryEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.TenantID == tenantID && entry.UserID == userID {
			out = append(out, entry)
		}
	}
	return out, nil
}

func (s *agentChatMemoryStore) Upsert(_ context.Context, tenantID, userID, key, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[tenantID+"/"+userID+"/"+key] = store.MemoryEntry{
		TenantID: tenantID,
		UserID:   userID,
		Key:      key,
		Content:  content,
	}
	return nil
}

func (s *agentChatMemoryStore) Delete(_ context.Context, tenantID, userID, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, tenantID+"/"+userID+"/"+key)
	return nil
}

func (s *agentChatMemoryStore) DeleteAllByUser(_ context.Context, tenantID, userID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var deleted int64
	for key, entry := range s.entries {
		if entry.TenantID == tenantID && entry.UserID == userID {
			delete(s.entries, key)
			deleted++
		}
	}
	return deleted, nil
}

func (s *agentChatMemoryStore) DeleteAllByTenant(_ context.Context, tenantID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var deleted int64
	for key, entry := range s.entries {
		if entry.TenantID == tenantID {
			delete(s.entries, key)
			deleted++
		}
	}
	return deleted, nil
}

type agentChatCheckpointStore struct {
	mu      sync.Mutex
	entries map[string]*store.AgentCheckpoint
}

func (s *agentChatCheckpointStore) Get(_ context.Context, tenantID, sessionID, checkpointID string) (*store.AgentCheckpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp, ok := s.entries[tenantID+"/"+sessionID+"/"+checkpointID]
	if !ok {
		return nil, store.ErrNotFound
	}
	clone := *cp
	clone.Payload = append([]byte(nil), cp.Payload...)
	return &clone, nil
}

func (s *agentChatCheckpointStore) Set(_ context.Context, cp *store.AgentCheckpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := *cp
	clone.Payload = append([]byte(nil), cp.Payload...)
	s.entries[cp.TenantID+"/"+cp.SessionID+"/"+cp.CheckpointID] = &clone
	return nil
}

func (s *agentChatCheckpointStore) Delete(_ context.Context, tenantID, sessionID, checkpointID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, tenantID+"/"+sessionID+"/"+checkpointID)
	return nil
}

func TestServeAgentHTTP_StreamPlanEvents(t *testing.T) {
	h := NewChatHandler(stubStore{}, nil, nil)
	h.llmModel = "test-model"
	h.runAgent = func(_ context.Context, _ string, _ []llm.Message, callbacks *eino.StreamCallbacks) (*eino.ConversationResult, error) {
		if callbacks != nil {
			// First tool call: emits plan_start + plan_step_update(running) + tool_result + plan_step_update(completed)
			callbacks.OnToolStart("web_search")
			callbacks.OnToolComplete("web_search")
			// Second tool call: only plan_step_update since plan already started
			callbacks.OnToolStart("code_exec")
			callbacks.OnToolComplete("code_exec")
			callbacks.OnStreamDelta("final answer")
		}
		return &eino.ConversationResult{FinalResponse: "final answer"}, nil
	}

	rec := httptest.NewRecorder()
	req := newAgentChatRequest(t, `{"messages":[{"role":"user","content":"search and run code"}],"stream":true}`)

	h.ServeAgentHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	// Verify plan_start event is emitted with both steps.
	if !strings.Contains(body, "event: plan_start") {
		t.Fatalf("expected plan_start event in SSE stream, got %q", body)
	}
	if !strings.Contains(body, `"steps"`) {
		t.Fatalf("expected steps in plan_start payload, got %q", body)
	}
	if !strings.Contains(body, `"web_search"`) {
		t.Fatalf("expected web_search in plan steps, got %q", body)
	}

	// Verify plan_step_update events are emitted.
	if !strings.Contains(body, "event: plan_step_update") {
		t.Fatalf("expected plan_step_update event in SSE stream, got %q", body)
	}
	if !strings.Contains(body, `"step_id":"web_search"`) {
		t.Fatalf("expected step_id web_search, got %q", body)
	}
	if !strings.Contains(body, `"step_id":"code_exec"`) {
		t.Fatalf("expected step_id code_exec, got %q", body)
	}
	if !strings.Contains(body, `"status":"running"`) {
		t.Fatalf("expected status running, got %q", body)
	}
	if !strings.Contains(body, `"status":"completed"`) {
		t.Fatalf("expected status completed, got %q", body)
	}

	// Verify existing tool_call and tool_result events are still present.
	if !strings.Contains(body, "event: tool_call") {
		t.Fatalf("expected tool_call event preserved, got %q", body)
	}
	if !strings.Contains(body, "event: tool_result") {
		t.Fatalf("expected tool_result event preserved, got %q", body)
	}
}
