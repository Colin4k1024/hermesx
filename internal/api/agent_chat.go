package api

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Colin4k1024/hermesx/internal/agent"
	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/eino"
	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/metering"
	"github.com/Colin4k1024/hermesx/internal/skills"
	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/Colin4k1024/hermesx/internal/tools"
	"github.com/Colin4k1024/hermesx/internal/toolsets"
)

// ServeAgentHTTP handles POST /v1/agent/chat using the full AIAgent (tool loop, soul, skills, memory).
func (h *chatHandler) ServeAgentHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	tenantID := ac.TenantID
	userID := ac.Identity

	sessionID := r.Header.Get("X-Hermes-Session-Id")
	if sessionID == "" {
		b := make([]byte, 16)
		_, _ = crypto_rand.Read(b)
		sessionID = fmt.Sprintf("sess_%x", b)
	}

	var req chatReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON request body", http.StatusBadRequest)
		return
	}

	// Extract the latest user message from the request.
	userMessage := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			userMessage = req.Messages[i].Content
			break
		}
	}
	if userMessage == "" {
		http.Error(w, "no user message found", http.StatusBadRequest)
		return
	}

	// Ensure session exists in PG.
	isNewSession := false
	sess, err := h.store.Sessions().Get(ctx, tenantID, sessionID)
	if err != nil || sess == nil {
		title := agent.GenerateSessionTitle([]llm.Message{{Role: "user", Content: userMessage}})
		sess = &store.Session{
			ID:        sessionID,
			TenantID:  tenantID,
			Platform:  "api",
			UserID:    userID,
			Model:     h.llmModel,
			Title:     title,
			StartedAt: time.Now(),
		}
		if createErr := h.store.Sessions().Create(ctx, tenantID, sess); createErr != nil {
			slog.Error("session creation failed", "error", createErr, "tenant_id", tenantID, "session_id", sess.ID)
			http.Error(w, "session creation failed", http.StatusInternalServerError)
			return
		}
		isNewSession = true
	} else if sess.UserID != "" && sess.UserID != userID && !ac.HasScope("admin") {
		http.Error(w, "forbidden: session belongs to another user", http.StatusForbidden)
		return
	}
	unlockSession := h.lockAgentSession(tenantID, sessionID)
	defer unlockSession()

	if !isNewSession && sess.Title == "" {
		title := agent.GenerateSessionTitle([]llm.Message{{Role: "user", Content: userMessage}})
		if title != "Untitled session" {
			_ = h.store.Sessions().SetTitle(ctx, tenantID, sessionID, title)
		}
	}
	w.Header().Set("X-Hermes-Session-Id", sessionID)

	// Load existing conversation history from PG. The current user turn is kept
	// out of persistence until the agent completes, so failed or interrupted
	// requests do not leave half-written turns or duplicate resume history.
	historyMsgs, err := h.store.Messages().List(ctx, tenantID, sessionID, 200, 0)
	if err != nil {
		historyMsgs = nil
	}

	// Build []llm.Message history for the agent; RunConversation takes
	// userMessage separately and appends it internally.
	history := make([]llm.Message, 0, len(historyMsgs))
	for _, m := range historyMsgs {
		if m.Role == "system" {
			continue
		}
		history = append(history, storeMessageToLLM(m))
	}

	// Load per-tenant soul from MinIO.
	soulContent := h.getSoulPrompt(ctx, tenantID)

	// Trigger per-user skill provisioning on the first request from this user.
	// The check is done in-memory first (sync.Map) to avoid an OSS HEAD call on every request.
	if h.provisioner != nil && userID != "" {
		cacheKey := tenantID + "/" + userID
		if _, alreadyDone := h.provisionedUsers.Load(cacheKey); !alreadyDone {
			bgCtx, bgCancel := context.WithTimeout(context.Background(), 30*time.Second)
			go func() {
				defer bgCancel()
				if err := h.provisioner.ProvisionUserSkills(bgCtx, tenantID, userID); err != nil {
					slog.Warn("user_skill_provision_failed", "tenant", tenantID, "user", userID, "error", err)
					return
				}
				h.provisionedUsers.Store(cacheKey, struct{}{})
			}()
		}
	}

	runAgent := h.runAgent
	if runAgent == nil {
		// Build skill loader for this tenant+user (composite: user-scoped first, tenant fallback).
		var skillLoader skills.SkillLoader
		if h.skillsClient != nil {
			tenantLoader := skills.NewMinIOSkillLoader(h.skillsClient, tenantID)
			if userID != "" {
				userLoader := skills.NewMinIOUserSkillLoader(h.skillsClient, tenantID, userID)
				skillLoader = skills.NewCompositeSkillLoader(userLoader, tenantLoader)
			} else {
				skillLoader = tenantLoader
			}
		}

		// Build store-backed memory provider (works with MySQL, PostgreSQL, SQLite).
		var memProvider tools.MemoryProvider
		if h.store != nil {
			memProvider = agent.NewStoreMemoryProviderAsToolsProvider(h.store, tenantID, userID)
		}

		systemPrompt := h.buildAgenticSystemPrompt(ctx, soulContent, skillLoader, memProvider)

		var llmClient *llm.Client
		if h.apiMode != "" {
			llmClient, err = llm.NewClientWithMode(h.llmModel, h.llmURL, h.llmAPIKey, "", llm.APIMode(h.apiMode))
		} else {
			llmClient, err = llm.NewClientWithParams(h.llmModel, h.llmURL, h.llmAPIKey, "")
		}
		if err != nil || llmClient == nil {
			slog.Error("llm_client_creation_failed", "tenant", tenantID, "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		agentOpts := []eino.Option{
			eino.WithTransport(llmClient.GetTransport()),
			eino.WithModel(llmClient.Model()),
			eino.WithProvider(llmClient.Provider()),
			eino.WithBaseURL(llmClient.BaseURL()),
			eino.WithAPIKey(h.llmAPIKey),
			eino.WithAPIMode(string(llmClient.APIMode())),
			eino.WithTenantID(tenantID),
			eino.WithUserID(userID),
			eino.WithSessionID(sessionID),
			eino.WithPlatform("api"),
			eino.WithSystemPrompt(systemPrompt),
			eino.WithTools(h.agenticToolEntries(ctx, tenantID)),
			eino.WithMemoryProvider(memProvider),
			eino.WithCheckpointStore(storeCheckpointAdapter(h.store)),
			eino.WithReceiptRecorder(tools.NewReceiptRecorder(h.store.ExecutionReceipts())),
			eino.WithObjectStore(h.skillsClient),
		}
		if h.egressTransport != nil {
			agentOpts = append(agentOpts, eino.WithHTTPTransport(h.egressTransport))
		}
		if h.safetyInterceptor != nil {
			agentOpts = append(agentOpts, eino.WithSafetyInterceptor(h.safetyInterceptor))
		}
		if h.leakScanner != nil {
			agentOpts = append(agentOpts, eino.WithLeakScanner(h.leakScanner))
		}
		runAgent = func(ctx context.Context, userMessage string, history []llm.Message, callbacks *eino.StreamCallbacks) (*eino.ConversationResult, error) {
			return eino.RunConversationTurnLoopSafe(ctx, userMessage, history, callbacks, agentOpts...)
		}
	}

	// Real SSE streaming: set up headers and callbacks BEFORE running agent.
	if req.Stream {
		rc := http.NewResponseController(w)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		if err := rc.Flush(); err != nil {
			slog.Error("streaming flush failed", "error", err)
			return
		}

		// Track SSE connection for per-user stream limiting (ADR-001).
		if h.sseTracker != nil {
			sseKey := tenantID + ":" + userID
			h.sseTracker.IncrSSEConnections(sseKey)
			defer h.sseTracker.DecrSSEConnections(sseKey)
		}

		created := time.Now().Unix()
		chunkID := sessionID

		var writeMu sync.Mutex
		writeSSE := func(data []byte) {
			writeMu.Lock()
			fmt.Fprintf(w, "data: %s\n\n", data)
			rc.Flush()
			writeMu.Unlock()
		}

		// Role announcement.
		roleChunk, _ := json.Marshal(sseChunkResp{
			ID: chunkID, Object: "chat.completion.chunk", Created: created, Model: h.llmModel,
			Choices: []sseChunkDelta{{Index: 0, Delta: sseChunkContent{Role: "assistant"}}},
		})
		writeSSE(roleChunk)

		// Wire real-time streaming callbacks.
		var planMu sync.Mutex
		planSteps := make([]eino.PlanStep, 0)
		planStepIndex := make(map[string]int)
		planStarted := false
		writeSSEEvent := func(event string, payload any) {
			evt, _ := json.Marshal(payload)
			writeMu.Lock()
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, evt)
			rc.Flush()
			writeMu.Unlock()
		}

		callbacks := &eino.StreamCallbacks{
			OnStreamDelta: func(text string) {
				chunk, _ := json.Marshal(sseChunkResp{
					ID: chunkID, Object: "chat.completion.chunk", Created: created, Model: h.llmModel,
					Choices: []sseChunkDelta{{Index: 0, Delta: sseChunkContent{Content: text}}},
				})
				writeSSE(chunk)
			},
			OnAgenticBlock: func(block eino.AgenticBlock) {
				if !req.IncludeAgenticBlocks {
					return
				}
				writeSSEEvent("agentic_block", block)
			},
			OnToolStart: func(toolName string) {
				writeSSEEvent("tool_call", map[string]string{"tool": toolName, "status": "started"})

				planMu.Lock()
				stepID := toolName
				if _, exists := planStepIndex[stepID]; !exists {
					step := eino.PlanStep{ID: stepID, Title: toolName}
					planSteps = append(planSteps, step)
					planStepIndex[stepID] = len(planSteps) - 1
				}
				if !planStarted {
					planStarted = true
					stepsCopy := make([]eino.PlanStep, len(planSteps))
					copy(stepsCopy, planSteps)
					planMu.Unlock()
					writeSSEEvent("plan_start", eino.PlanStartEvent{Steps: stepsCopy})
				} else {
					planMu.Unlock()
				}
				writeSSEEvent("plan_step_update", eino.PlanStepUpdateEvent{StepID: stepID, Status: "running"})
			},
			OnToolComplete: func(toolName string) {
				writeSSEEvent("tool_result", map[string]string{"tool": toolName, "status": "completed"})
				writeSSEEvent("plan_step_update", eino.PlanStepUpdateEvent{StepID: toolName, Status: "completed"})
			},
		}

		// Heartbeat in background.
		heartDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					writeMu.Lock()
					fmt.Fprintf(w, ": heartbeat\n\n")
					rc.Flush()
					writeMu.Unlock()
				case <-heartDone:
					return
				case <-r.Context().Done():
					return
				}
			}
		}()

		// Run agent (deltas fire callbacks above in real-time).
		result, err := runAgent(ctx, userMessage, history, callbacks)
		close(heartDone)

		if err != nil {
			slog.Error("agent_stream_failed", "tenant", tenantID, "session", sessionID, "error", err)
			errEvt, _ := json.Marshal(map[string]string{"error": "agent execution failed"})
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", errEvt)
			rc.Flush()
			fmt.Fprintf(w, "data: [DONE]\n\n")
			rc.Flush()
			return
		}

		// Persist and update tokens.
		h.sendMsg(ctx, tenantID, sessionID, "user", userMessage)
		h.sendMsgWithMeta(ctx, tenantID, sessionID, "assistant", result.FinalResponse, result.LastReasoning, eino.BlocksJSON(result.AgenticBlocks))
		h.extractAndPersistMemories(tenantID, userID, userMessage)
		h.store.Sessions().UpdateTokens(ctx, tenantID, sessionID, store.TokenDelta{
			Input: result.InputTokens, Output: result.OutputTokens,
		})
		h.recordAgentUsage(ctx, tenantID, sessionID, userID, result)

		// Finish + DONE.
		stop := "stop"
		finalChunk, _ := json.Marshal(sseChunkResp{
			ID: chunkID, Object: "chat.completion.chunk", Created: created, Model: h.llmModel,
			Choices: []sseChunkDelta{{Index: 0, Delta: sseChunkContent{}, FinishReason: &stop}},
		})
		writeSSE(finalChunk)
		fmt.Fprintf(w, "data: [DONE]\n\n")
		rc.Flush()
		return
	}

	// Non-streaming path.
	result, err := runAgent(ctx, userMessage, history, nil)
	if err != nil {
		slog.Error("agent_run_failed", "tenant", tenantID, "session", sessionID, "error", err)
		http.Error(w, "agent execution failed", http.StatusBadGateway)
		return
	}

	reply := result.FinalResponse

	// Persist the completed user/assistant turn to PG.
	h.sendMsg(ctx, tenantID, sessionID, "user", userMessage)
	msgID := h.sendMsgWithMeta(ctx, tenantID, sessionID, "assistant", reply, result.LastReasoning, eino.BlocksJSON(result.AgenticBlocks))

	h.extractAndPersistMemories(tenantID, userID, userMessage)

	// Update session token counters.
	h.store.Sessions().UpdateTokens(ctx, tenantID, sessionID, store.TokenDelta{
		Input:  result.InputTokens,
		Output: result.OutputTokens,
	})
	h.recordAgentUsage(ctx, tenantID, sessionID, userID, result)

	slog.Info("agent_chat_completion",
		"tenant", tenantID,
		"session", sessionID,
		"api_calls", result.APICalls,
		"input_tokens", result.InputTokens,
		"output_tokens", result.OutputTokens,
		"msg_id", msgID,
	)

	w.Header().Set("Content-Type", "application/json")
	resp := chatResp{
		ID:      sessionID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   h.llmModel,
		Choices: []chatChoice{{
			Index:        0,
			Message:      chatMessage{Role: "assistant", Content: reply},
			FinishReason: "stop",
		}},
		Usage: chatUsage{
			PromptTokens:     result.InputTokens,
			CompletionTokens: result.OutputTokens,
			TotalTokens:      result.TotalTokens,
		},
	}
	if req.IncludeAgenticBlocks {
		resp.AgenticBlocks = result.AgenticBlocks
	}
	json.NewEncoder(w).Encode(resp)
}

func (h *chatHandler) extractAndPersistMemories(tenantID, userID, userMessage string) {
	if h.store == nil {
		return
	}
	memStore := h.store.Memories()
	if memStore == nil {
		return
	}
	extractor := &memoryExtractor{memStore: memStore}
	if memories := extractor.extract(userMessage); len(memories) > 0 {
		extractor.persist(tenantID, userID, memories)
	}
}

func (h *chatHandler) agenticToolEntries(ctx context.Context, tenantID string) []*tools.ToolEntry {
	names := toolsets.ResolveToolset(getEnvOr("HERMES_AGENT_TOOLSET", toolsets.GovernedToolsetName))
	names = toolsets.FilterTenantAuthorizedTools(names, h.tenantAllowedTools(ctx, tenantID))
	entries := make([]*tools.ToolEntry, 0, len(names))
	for _, name := range names {
		entry := tools.Registry().Lookup(name)
		if entry == nil {
			continue
		}
		if entry.CheckFn != nil && !entry.CheckFn() {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

func (h *chatHandler) tenantAllowedTools(ctx context.Context, tenantID string) []string {
	if h.store == nil || strings.TrimSpace(tenantID) == "" {
		return nil
	}
	tenant, err := h.store.Tenants().Get(ctx, tenantID)
	if err != nil || tenant == nil || tenant.SandboxPolicy == nil {
		return nil
	}
	return tenant.SandboxPolicy.AllowedTools
}

func (h *chatHandler) recordAgentUsage(ctx context.Context, tenantID, sessionID, userID string, result *eino.ConversationResult) {
	if h.usageStore == nil || result == nil {
		return
	}
	model := result.Model
	if model == "" {
		model = h.llmModel
	}
	provider := result.Provider
	if provider == "" {
		provider = "custom"
	}
	record := metering.UsageRecord{
		TenantID:         tenantID,
		SessionID:        sessionID,
		UserID:           userID,
		Model:            model,
		Provider:         provider,
		InputTokens:      result.InputTokens,
		OutputTokens:     result.OutputTokens,
		CacheReadTokens:  result.CacheReadTokens,
		CacheWriteTokens: result.CacheWriteTokens,
		CostUSD:          metering.CalculateCost(model, result.InputTokens, result.OutputTokens),
		CreatedAt:        time.Now(),
	}
	if err := h.usageStore.BatchInsert(ctx, []metering.UsageRecord{record}); err != nil {
		slog.Warn("usage_record_insert_failed", "tenant", tenantID, "session", sessionID, "error", err)
	}
}

func storeMessageToLLM(m *store.Message) llm.Message {
	if m == nil {
		return llm.Message{}
	}
	out := llm.Message{
		Role:         m.Role,
		Content:      m.Content,
		ToolCallID:   m.ToolCallID,
		ToolName:     m.ToolName,
		Reasoning:    m.Reasoning,
		FinishReason: m.FinishReason,
	}
	if m.ToolCalls != "" {
		var calls []llm.ToolCall
		if err := json.Unmarshal([]byte(m.ToolCalls), &calls); err == nil {
			out.ToolCalls = calls
		}
	}
	return out
}

func (h *chatHandler) lockAgentSession(tenantID, sessionID string) func() {
	key := tenantID + "/" + sessionID
	actual, _ := h.sessionLocks.LoadOrStore(key, &sync.Mutex{})
	mu := actual.(*sync.Mutex)
	mu.Lock()
	return func() {
		mu.Unlock()
		h.sessionLocks.Delete(key)
	}
}

// SSE chunk types matching OpenAI chat.completion.chunk format.
type sseChunkResp struct {
	ID      string          `json:"id"`
	Object  string          `json:"object"`
	Created int64           `json:"created"`
	Model   string          `json:"model"`
	Choices []sseChunkDelta `json:"choices"`
}

type sseChunkDelta struct {
	Index        int             `json:"index"`
	Delta        sseChunkContent `json:"delta"`
	FinishReason *string         `json:"finish_reason"`
}

type sseChunkContent struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}
