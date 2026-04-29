package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hermes-agent/hermes-agent-go/internal/auth"
)

// chatMessage represents a single message in a session.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// mockChatStore is a thread-safe in-memory store for sessions.
// Messages are scoped to (tenantID, sessionID).
type mockChatStore struct {
	mu       sync.RWMutex
	messages map[string][]chatMessage // key: "tenantID:sessionID"
}

func newMockChatStore() *mockChatStore {
	return &mockChatStore{messages: make(map[string][]chatMessage)}
}

func (s *mockChatStore) sessionKey(tenantID, sessionID string) string {
	return tenantID + ":" + sessionID
}

func (s *mockChatStore) GetMessages(tenantID, sessionID string) []chatMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.messages[s.sessionKey(tenantID, sessionID)]
}

func (s *mockChatStore) AppendMessage(tenantID, sessionID, role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.sessionKey(tenantID, sessionID)
	s.messages[key] = append(s.messages[key], chatMessage{Role: role, Content: content})
}

func (s *mockChatStore) ClearSession(tenantID, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.messages, s.sessionKey(tenantID, sessionID))
}

func (s *mockChatStore) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = make(map[string][]chatMessage)
}

// mockChatHandler handles chat requests. It stores messages per (tenant, session)
// and calls a real LLM API to generate responses, enabling true multi-tenant
// isolation testing with actual AI responses.
type mockChatHandler struct {
	store     *mockChatStore
	llmURL    string
	llmAPIKey string
	llmModel  string
	httpClient *http.Client
}

func newMockChatHandler() *mockChatHandler {
	return &mockChatHandler{
		store:     newMockChatStore(),
		llmURL:    getEnvOr("LLM_API_URL", "http://localhost:8000"),
		llmAPIKey: getEnvOr("LLM_API_KEY", "123456"),
		llmModel:  getEnvOr("LLM_MODEL", "Qwen3-Coder-Next-4bit"),
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

// chatReq / chatResp match the OpenAI /v1/chat/completions format.
type chatReq struct {
	Model    string         `json:"model"`
	Messages []chatMessage  `json:"messages"`
	Stream   bool           `json:"stream"`
}

type chatResp struct {
	ID      string           `json:"id"`
	Object string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []chatChoice    `json:"choices"`
	Usage   chatUsage        `json:"usage"`
}

type chatChoice struct {
	Index        int          `json:"index"`
	Message      chatMessage  `json:"message"`
	FinishReason string       `json:"finish_reason"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens     int `json:"total_tokens"`
}

func (h *mockChatHandler) callLLM(ctx context.Context, tenantID, sessionID string, messages []chatMessage) (string, error) {
	// Build system prompt with tenant context
	// Strip dashes from UUID so the first 8 chars are a compact, unique prefix.
	tenantCompact := strings.ReplaceAll(tenantID, "-", "")
	systemPrompt := fmt.Sprintf(
		"You are a helpful assistant. Your tenant ID is '%s' and session ID is '%s'. "+
			"Include BOTH in every reply in this exact format: [T:%s S:%s] — then your answer.",
		tenantID, sessionID, tenantCompact[:8], sessionID,
	)

	// Prepend system prompt
	llmMessages := append([]chatMessage{{Role: "system", Content: systemPrompt}}, messages...)

	payload := map[string]any{
		"model":    h.llmModel,
		"messages": llmMessages,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.llmURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.llmAPIKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call LLM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", fmt.Errorf("LLM returned HTTP %d: %s", resp.StatusCode, string(b))
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return "", fmt.Errorf("decode LLM response: %w", err)
	}
	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}
	return llmResp.Choices[0].Message.Content, nil
}

// ServeHTTP handles POST /v1/chat/completions.
func (h *mockChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract auth context (set by Auth middleware).
	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	tenantID := ac.TenantID

	// Session ID from header or generate one.
	sessionID := r.Header.Get("X-Hermes-Session-Id")
	if sessionID == "" {
		sessionID = fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}

	var req chatReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Append all messages to session store (for isolation verification).
	for _, msg := range req.Messages {
		if msg.Content == "" {
			continue
		}
		h.store.AppendMessage(tenantID, sessionID, msg.Role, msg.Content)
	}

	// Get conversation history for context.
	history := h.store.GetMessages(tenantID, sessionID)

	// Call real LLM.
	reply, err := h.callLLM(r.Context(), tenantID, sessionID, history)
	if err != nil {
		slog.Warn("LLM call failed, returning error response", "error", err, "tenant", tenantID)
		http.Error(w, fmt.Sprintf("LLM error: %v", err), http.StatusBadGateway)
		return
	}

	// Append assistant response to session store.
	h.store.AppendMessage(tenantID, sessionID, "assistant", reply)

	slog.Info("chat_completion",
		"tenant", tenantID,
		"session", sessionID,
		"messages", len(history),
		"auth_method", ac.AuthMethod,
		"model", h.llmModel,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chatResp{
		ID:      sessionID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []chatChoice{{
			Index:        0,
			Message:      chatMessage{Role: "assistant", Content: reply},
			FinishReason: "stop",
		}},
		Usage: chatUsage{
			PromptTokens:     42,
			CompletionTokens: len(reply) / 4,
			TotalTokens:     42 + len(reply)/4,
		},
	})
}

// sessionsResp returns session info for GET /v1/mock-sessions.
type sessionsResp struct {
	TenantID string        `json:"tenant_id"`
	Sessions []sessionInfo `json:"sessions"`
}

type sessionInfo struct {
	SessionID  string `json:"session_id"`
	MessageCount int  `json:"message_count"`
}

// handleMockSessionList handles GET /v1/mock-sessions.
func (h *mockChatHandler) handleSessionList(w http.ResponseWriter, r *http.Request) {
	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	h.store.mu.RLock()
	defer h.store.mu.RUnlock()

	prefix := ac.TenantID + ":"
	var sessions []sessionInfo
	for key, msgs := range h.store.messages {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		sessionID := strings.TrimPrefix(key, prefix)
		sessions = append(sessions, sessionInfo{
			SessionID:   sessionID,
			MessageCount: len(msgs),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessionsResp{
		TenantID: ac.TenantID,
		Sessions: sessions,
	})
}

// handleMockClearSession handles DELETE /v1/mock-sessions/:id.
func (h *mockChatHandler) handleClearSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID := strings.TrimPrefix(r.URL.Path, "/v1/mock-sessions/")
	h.store.ClearSession(ac.TenantID, sessionID)

	w.WriteHeader(http.StatusNoContent)
}

// MockChatStore returns the underlying store for inspection.
func (h *mockChatHandler) MockChatStore() *mockChatStore {
	return h.store
}

// NewMockChatHandler creates a mock chat handler wired into the SaaS API server.
func NewMockChatHandler() *mockChatHandler {
	return newMockChatHandler()
}
