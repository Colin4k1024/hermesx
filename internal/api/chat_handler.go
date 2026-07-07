package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/eino"
	"github.com/Colin4k1024/hermesx/internal/evolution"
	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/metering"
	"github.com/Colin4k1024/hermesx/internal/middleware"
	"github.com/Colin4k1024/hermesx/internal/objstore"
	"github.com/Colin4k1024/hermesx/internal/safety"
	"github.com/Colin4k1024/hermesx/internal/secrets"
	"github.com/Colin4k1024/hermesx/internal/skills"
	"github.com/Colin4k1024/hermesx/internal/store"
	lru "github.com/hashicorp/golang-lru/v2/expirable"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type agentConversationRunner func(ctx context.Context, userMessage string, history []llm.Message, callbacks *eino.StreamCallbacks) (*eino.ConversationResult, error)

// activeSession tracks an in-flight agent conversation for abort support.
type activeSession struct {
	Cancel context.CancelFunc
	Done   chan struct{}
}

// chatHandler holds shared dependencies for agent chat, session, and memory endpoints.
type chatHandler struct {
	store             store.Store
	llmURL            string
	llmAPIKey         string
	llmModel          string
	apiMode           string
	httpClient        *http.Client
	egressTransport   *http.Transport
	safetyInterceptor safety.SafetyInterceptor
	leakScanner       *secrets.LeakScanner
	usageStore        metering.UsageStore
	skillsClient      objstore.ObjectStore

	// sseTracker manages per-user SSE connection counts for the stream limit.
	sseTracker *middleware.SSEConnectionTracker

	// provisioner copies tenant skills into per-user OSS namespaces on first request.
	provisioner *skills.Provisioner
	// provisionedUsers tracks which (tenantID, userID) pairs have already been provisioned
	// in this process lifetime, avoiding redundant OSS HEAD calls on every request.
	// Value type: struct{} (present = already triggered).
	provisionedUsers sync.Map

	soulCache *lru.LRU[string, string]

	// evolutionImprover is the global Oris gene-backed evolution path (optional).
	evolutionImprover *evolution.Improver
	runAgent          agentConversationRunner
	sessionLocks      sync.Map

	// activeSessions tracks in-flight agent sessions for abort support.
	// Key: "tenantID:sessionID", Value: *activeSession
	activeSessions sync.Map
}

// SetEvolutionImprover attaches a shared Oris Improver to the handler.
// Must be called before the handler starts serving requests.
func (h *chatHandler) SetEvolutionImprover(imp *evolution.Improver) {
	h.evolutionImprover = imp
}

const soulCacheTTL = 30 * time.Minute
const soulCacheMaxEntries = 500

const defaultSoul = `You are a helpful, knowledgeable, and friendly AI assistant.
Be concise, accurate, and helpful in all your responses.`

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

// chatReq / chatResp match the OpenAI /v1/chat/completions format.
type chatReq struct {
	Model                string        `json:"model"`
	Messages             []chatMessage `json:"messages"`
	Stream               bool          `json:"stream"`
	IncludeAgenticBlocks bool          `json:"include_agentic_blocks,omitempty"`
}

type chatResp struct {
	ID            string              `json:"id"`
	Object        string              `json:"object"`
	Created       int64               `json:"created"`
	Model         string              `json:"model"`
	Choices       []chatChoice        `json:"choices"`
	Usage         chatUsage           `json:"usage"`
	AgenticBlocks []eino.AgenticBlock `json:"agentic_blocks,omitempty"`
}

type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (h *chatHandler) getSoulPrompt(ctx context.Context, tenantID string) string {
	if content, ok := h.soulCache.Get(tenantID); ok {
		return content
	}

	if h.skillsClient == nil {
		return defaultSoul
	}

	raw, err := h.skillsClient.GetObject(ctx, tenantID+"/SOUL.md")
	if err != nil {
		slog.Debug("soul_load_fallback", "tenant", tenantID, "error", err)
		return defaultSoul
	}
	content := string(raw)

	h.soulCache.Add(tenantID, content)
	return content
}

func (h *chatHandler) sendMsg(ctx context.Context, tenantID, sessionID string, role, content string) int64 {
	id, err := h.store.Messages().Append(ctx, tenantID, sessionID, &store.Message{
		TenantID:  tenantID,
		SessionID: sessionID,
		Role:      role,
		Content:   content,
	})
	if err != nil {
		slog.Error("sendMsg_FAILED", "tenant", tenantID, "session", sessionID, "role", role, "error", err)
		return 0
	}
	slog.Info("sendMsg_OK", "tenant", tenantID, "session", sessionID, "role", role, "msg_id", id)
	return id
}

// NewChatHandler creates the chat handler wired into the SaaS API server.
// provisioner may be nil when object storage is not configured.
func NewChatHandler(s store.Store, skillsClient objstore.ObjectStore, provisioner *skills.Provisioner) *chatHandler {
	return &chatHandler{
		store:        s,
		llmURL:       getEnvOr("LLM_API_URL", "http://localhost:8000"),
		llmAPIKey:    getEnvOr("LLM_API_KEY", ""),
		llmModel:     getEnvOr("LLM_MODEL", "Qwen3-Coder-Next-4bit"),
		apiMode:      getEnvOr("HERMES_API_MODE", ""),
		httpClient:   &http.Client{Timeout: 120 * time.Second},
		skillsClient: skillsClient,
		provisioner:  provisioner,
		soulCache:    lru.NewLRU[string, string](soulCacheMaxEntries, nil, soulCacheTTL),
	}
}

func (h *chatHandler) SetEgressTransport(transport *http.Transport) {
	h.egressTransport = transport
}

func (h *chatHandler) SetSafetyInterceptor(interceptor safety.SafetyInterceptor) {
	h.safetyInterceptor = interceptor
}

func (h *chatHandler) SetLeakScanner(scanner *secrets.LeakScanner) {
	h.leakScanner = scanner
}

func (h *chatHandler) SetUsageStore(store metering.UsageStore) {
	h.usageStore = store
}

// SetSSETracker attaches a shared SSE connection tracker for per-user stream limiting.
func (h *chatHandler) SetSSETracker(tracker *middleware.SSEConnectionTracker) {
	h.sseTracker = tracker
}

// registerActiveSession registers an active session for abort support.
func (h *chatHandler) registerActiveSession(tenantID, sessionID string, cancel context.CancelFunc) *activeSession {
	key := tenantID + ":" + sessionID
	session := &activeSession{
		Cancel: cancel,
		Done:   make(chan struct{}),
	}
	h.activeSessions.Store(key, session)
	return session
}

// unregisterActiveSession removes a completed session.
func (h *chatHandler) unregisterActiveSession(tenantID, sessionID string) {
	key := tenantID + ":" + sessionID
	h.activeSessions.Delete(key)
}

// abortSession cancels an active session.
func (h *chatHandler) abortSession(tenantID, sessionID string) bool {
	key := tenantID + ":" + sessionID
	if val, ok := h.activeSessions.Load(key); ok {
		session := val.(*activeSession)
		session.Cancel()
		return true
	}
	return false
}

// AbortAgentHTTP handles POST /v1/chat/abort to cancel an in-flight agent session.
func (h *chatHandler) AbortAgentHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == "" {
		http.Error(w, "invalid request: session_id required", http.StatusBadRequest)
		return
	}

	if h.abortSession(ac.TenantID, req.SessionID) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success":    true,
			"session_id": req.SessionID,
			"message":    "abort signal sent",
		})
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"success":    false,
			"session_id": req.SessionID,
			"message":    "session not found or already completed",
		})
	}
}
