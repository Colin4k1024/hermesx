package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Colin4k1024/hermesx/internal/config"
	"github.com/Colin4k1024/hermesx/internal/evolution"
	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/skills"
	"github.com/Colin4k1024/hermesx/internal/state"
	"github.com/Colin4k1024/hermesx/internal/tools"
	"github.com/Colin4k1024/hermesx/internal/toolsets"
	"github.com/google/uuid"
)

// FallbackModel describes an alternative model to try on API failures.
type FallbackModel struct {
	Model    string `yaml:"model"`
	Provider string `yaml:"provider"`
	BaseURL  string `yaml:"base_url"`
	APIKey   string `yaml:"api_key"`
}

// AIAgent is the core agent that manages conversations with LLM and tools.
type AIAgent struct {
	// Configuration
	model                 string
	baseURL               string
	apiKey                string
	provider              string
	apiMode               string // "openai" or "anthropic"
	maxIterations         int
	platform              string
	sessionID             string
	quietMode             bool
	enabledToolsets       []string
	disabledToolsets      []string
	ephemeralSystemPrompt string
	skillLoader           skills.SkillLoader
	tenantID              string
	userID                string
	memoryProvider        tools.MemoryProvider
	soulContent           string
	skipContextFiles      bool
	skipMemory            bool
	persistSession        bool
	reasoningLevel        string // "xhigh", "high", "medium", "low", "minimal", ""
	compressionCfg        CompressionConfig

	// Session resume
	resumeSessionID string

	// Fallback model chain
	fallbackModels []FallbackModel

	// Smart routing
	smartRouter      *SmartRouter
	multimodalRouter *MultimodalRouter

	// Self-improvement loop (optional, async, non-blocking).
	selfImprover *SelfImprover

	// Oris evolution path (optional, parallel to selfImprover).
	evolutionImprover *evolution.Improver

	// Runtime state
	client          *llm.Client
	auxiliaryClient *AuxiliaryClient
	sessionDB       *state.SessionDB
	budget          *IterationBudget
	callbacks       *StreamCallbacks
	toolDefs        []llm.ToolDef
	validTools      map[string]bool
	systemPrompt    string

	// Interrupt support (lock-free)
	interruptRequested atomic.Bool

	// Steer support (needs mutex for read-and-clear)
	steerMu      sync.Mutex
	steerMessage string

	// Runtime counters
	apiCallCount int
	lastActivity time.Time
	heartbeatCh  chan struct{}

	// Compression cooldown
	lastCompressionFailure time.Time

	// summaryCompleter overrides the LLM client used for context
	// compression summaries.  Nil means use the main client.
	summaryCompleter chatCompleter

	// Token tracking
	totalInputTokens  int
	totalOutputTokens int
	totalCacheRead    int
	totalCacheWrite   int
	totalReasoning    int

	// Cost tracking
	cumulativeCostUSD float64
}

// New creates a new AIAgent with the given options.
func New(opts ...AgentOption) (*AIAgent, error) {
	cfg := config.Load()

	// Resolve reasoning level from config.
	var reasoningLvl string
	if cfg.Reasoning.Enabled {
		reasoningLvl = cfg.Reasoning.Effort
		if reasoningLvl == "" {
			reasoningLvl = "medium"
		}
	}

	a := &AIAgent{
		model:            cfg.Model,
		baseURL:          cfg.BaseURL,
		apiKey:           cfg.APIKey,
		provider:         cfg.Provider,
		apiMode:          cfg.APIMode,
		maxIterations:    cfg.MaxIterations,
		platform:         "cli",
		persistSession:   true,
		reasoningLevel:   reasoningLvl,
		compressionCfg:   DefaultCompressionConfig(),
		multimodalRouter: DefaultMultimodalRouter(),
		lastActivity:     time.Now(),
	}

	// Options override config defaults
	for _, opt := range opts {
		opt(a)
	}

	// Create iteration budget if not shared
	if a.budget == nil {
		a.budget = NewIterationBudget(a.maxIterations)
	}

	// Generate session ID if not set
	if a.sessionID == "" {
		a.sessionID = uuid.New().String()
	}

	// Create LLM client
	var err error
	if a.baseURL != "" && a.apiKey != "" {
		if a.apiMode != "" {
			a.client, err = llm.NewClientWithMode(a.model, a.baseURL, a.apiKey, a.provider, llm.APIMode(a.apiMode))
		} else {
			a.client, err = llm.NewClientWithParams(a.model, a.baseURL, a.apiKey, a.provider)
		}
	} else {
		a.client, err = llm.NewClient(cfg)
	}
	if err != nil {
		return nil, fmt.Errorf("create LLM client: %w", err)
	}

	a.model = a.client.Model()
	a.provider = a.client.Provider()
	a.baseURL = a.client.BaseURL()

	// Initialize auxiliary clients
	a.auxiliaryClient = NewAuxiliaryClient(cfg)
	if a.auxiliaryClient != nil && a.auxiliaryClient.SummaryClient() != nil {
		a.summaryCompleter = a.auxiliaryClient.SummaryClient()
	}

	// Open session DB
	if a.persistSession {
		a.sessionDB, err = state.NewSessionDB("")
		if err != nil {
			slog.Warn("Could not open session DB", "error", err)
		}
	}

	// Handle session resume
	if a.resumeSessionID != "" {
		if err := a.loadResumedSession(); err != nil {
			slog.Warn("Failed to resume session", "session_id", a.resumeSessionID, "error", err)
		}
	}

	// Build tool definitions
	a.buildToolDefs(cfg)

	// Build system prompt
	a.systemPrompt = a.buildSystemPrompt()

	return a, nil
}

// Chat is the simple chat interface. Returns final response text.
func (a *AIAgent) Chat(message string) (string, error) {
	result, err := a.RunConversation(message, nil)
	if err != nil {
		return "", err
	}
	return result.FinalResponse, nil
}

// RunConversation runs a full conversation turn with tool calling.
func (a *AIAgent) RunConversation(userMessage string, history []llm.Message) (*ConversationResult, error) {
	timeout := 120 * time.Second
	if v := os.Getenv("HERMES_CONVERSATION_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create session in DB
	if a.sessionDB != nil {
		a.sessionDB.CreateSession(a.sessionID, a.platform, a.model, "")
	}

	// Build messages
	messages := make([]llm.Message, 0, len(history)+2)
	messages = append(messages, history...)
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: userMessage,
	})

	// Persist user message
	if a.sessionDB != nil {
		a.sessionDB.AppendMessage(a.sessionID, "user", userMessage, "", "", nil, "")
	}

	// Auto-generate title on first turn
	a.autoGenerateTitle(messages)

	result := &ConversationResult{
		Model:    a.model,
		Provider: a.provider,
		BaseURL:  a.baseURL,
	}

	a.apiCallCount = 0
	a.interruptRequested.Store(false)

	// Oris evolution: inject high-confidence gene strategies into context.
	if a.evolutionImprover != nil {
		taskClass := evolution.DetectTaskClass(messages, nil)
		if strategies := a.evolutionImprover.PreTurnEnrich(ctx, a.tenantID, taskClass); len(strategies) > 0 {
			messages = prependEvolutionContext(messages, strategies)
		}
	}

	// Main agent loop
	emptyRetryCount := 0
	for a.apiCallCount < a.maxIterations {
		if !a.budget.Consume() {
			a.fireStatus("Iteration budget exhausted")
			break
		}

		if a.isInterrupted() {
			result.Interrupted = true
			break
		}

		a.apiCallCount++
		a.lastActivity = time.Now()

		// Proactive context compression when approaching model limits.
		if a.ShouldCompress(messages) {
			slog.Info("Context pressure: compressing", "session", a.sessionID)
			if compressed, compErr := a.CompressContext(ctx, messages); compErr == nil {
				messages = compressed
			} else {
				slog.Warn("Compression failed, continuing", "error", compErr)
			}
		}

		// Fire step callback
		a.fireStep(a.apiCallCount, nil)

		// Build API request
		apiMessages := a.buildAPIMessages(messages)

		req := llm.ChatRequest{
			Messages:       apiMessages,
			Tools:          a.toolDefs,
			Stream:         a.hasStreamConsumers(),
			ReasoningLevel: a.reasoningLevel,
		}

		// Route to vision client if images present and model lacks vision.
		activeClient := a.client
		if vc := a.visionClientForRequest(apiMessages); vc != nil {
			activeClient = vc
		}

		// Call LLM (with fallback chain on failure)
		a.fireHeartbeat()
		var resp *llm.ChatResponse
		var err error

		if req.Stream {
			resp, err = a.streamingAPICall(ctx, req)
		} else {
			resp, err = activeClient.CreateChatCompletion(ctx, req)
		}

		if err != nil {
			// Try fallback models
			resp, err = a.tryFallbackModels(ctx, req, err)
		}

		if err != nil {
			classified := ClassifyError(err, 0, a.provider, a.model)
			if classified != nil && classified.ShouldCompress && !a.isInCompressionCooldown() {
				slog.Warn("Context overflow detected, compressing and retrying", "session", a.sessionID)
				if compressed, compErr := a.CompressContext(ctx, messages); compErr == nil {
					messages = compressed
					a.apiCallCount--
					continue
				}
			}
			slog.Error("API call failed", "error", err, "attempt", a.apiCallCount)
			result.FinalResponse = fmt.Sprintf("API error: %v", err)
			result.Completed = false
			break
		}

		// Track tokens
		a.totalInputTokens += resp.Usage.PromptTokens
		a.totalOutputTokens += resp.Usage.CompletionTokens
		if a.sessionDB != nil {
			a.sessionDB.UpdateTokenCounts(a.sessionID, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, 0, 0, 0)
		}

		// Log context pressure for observability.
		LogContextPressure(a.totalInputTokens, llm.GetModelMeta(a.model).ContextLength, a.sessionID)

		// Extract reasoning from think blocks if not already present
		if resp.Reasoning == "" && resp.Content != "" {
			cleaned, reasoning := ExtractThinkContent(resp.Content)
			if reasoning != "" {
				resp.Reasoning = reasoning
				resp.Content = cleaned
			}
		}

		// Sanitize surrogates to prevent JSON encoding failures
		resp.Content = SanitizeSurrogates(resp.Content)

		// Repair misspelled tool names
		if len(resp.ToolCalls) > 0 && a.validTools != nil {
			repaired, count := RepairToolCalls(resp.ToolCalls, a.validTools)
			if count > 0 {
				slog.Info("Auto-repaired tool names", "count", count)
				resp.ToolCalls = repaired
			}
		}

		// Deduplicate tool calls
		if len(resp.ToolCalls) > 1 {
			resp.ToolCalls = DeduplicateToolCalls(resp.ToolCalls)
		}

		// Empty response recovery: retry with nudge
		if resp.Content == "" && len(resp.ToolCalls) == 0 {
			emptyRetryCount++
			if emptyRetryCount <= 3 {
				slog.Warn("Empty response from LLM, injecting nudge", "retry", emptyRetryCount)
				messages = append(messages, llm.Message{
					Role:    "user",
					Content: "Please continue with your response or use a tool to make progress.",
				})
				continue
			}
			slog.Error("Empty responses after multiple retries", "count", emptyRetryCount)
			result.FinalResponse = "Agent produced empty responses after multiple retries."
			result.Completed = false
			break
		}
		emptyRetryCount = 0

		// Validate tool calls for truncation
		if len(resp.ToolCalls) > 0 {
			valid, tcErrors := ValidateToolCalls(resp.ToolCalls)
			if len(tcErrors) > 0 {
				slog.Warn("Truncated tool calls detected", "count", len(tcErrors))
				resp.ToolCalls = valid
				// Inject error results for truncated calls
				for _, tce := range tcErrors {
					messages = append(messages, llm.Message{
						Role:       "tool",
						Content:    fmt.Sprintf(`{"error": "%s"}`, tce.Reason),
						ToolCallID: tce.ToolCall.ID,
						ToolName:   tce.ToolCall.Function.Name,
					})
				}
				if len(valid) == 0 {
					continue
				}
			}
		}

		// Build assistant message
		assistantMsg := llm.Message{
			Role:         "assistant",
			Content:      resp.Content,
			FinishReason: resp.FinishReason,
			Reasoning:    resp.Reasoning,
		}

		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = resp.ToolCalls
		}

		messages = append(messages, assistantMsg)

		// Persist assistant message
		if a.sessionDB != nil {
			var tcData []map[string]any
			for _, tc := range resp.ToolCalls {
				tcData = append(tcData, map[string]any{
					"id":   tc.ID,
					"type": tc.Type,
					"function": map[string]any{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				})
			}
			a.sessionDB.AppendMessage(a.sessionID, "assistant", resp.Content, "", "", tcData, resp.Reasoning)
		}

		// Handle tool calls
		if len(resp.ToolCalls) > 0 {
			toolResults := a.executeToolCalls(ctx, resp.ToolCalls)

			for _, tr := range toolResults {
				messages = append(messages, tr)
				if a.sessionDB != nil {
					a.sessionDB.AppendMessage(a.sessionID, "tool", tr.Content, tr.ToolCallID, tr.ToolName, nil, "")
				}
			}

			a.fireHeartbeat()

			if a.isInterrupted() {
				result.Interrupted = true
				break
			}

			// Check for steer message injection
			if steer := a.consumeSteer(); steer != "" {
				slog.Info("Injecting steer message", "length", len(steer))
				messages = append(messages, llm.Message{
					Role:    "user",
					Content: steer,
				})
			}

			// Continue loop for next LLM call
			continue
		}

		// No tool calls — final response
		result.FinalResponse = resp.Content
		result.Completed = true
		result.LastReasoning = resp.Reasoning
		break
	}

	// Finalize result
	result.Messages = messages
	result.APICalls = a.apiCallCount
	result.InputTokens = a.totalInputTokens
	result.OutputTokens = a.totalOutputTokens
	result.TotalTokens = a.totalInputTokens + a.totalOutputTokens
	result.CacheReadTokens = a.totalCacheRead
	result.CacheWriteTokens = a.totalCacheWrite
	result.ReasoningTokens = a.totalReasoning

	// Cost tracking
	cost := EstimateCost(a.model, a.totalInputTokens, a.totalOutputTokens)
	a.cumulativeCostUSD += cost
	result.EstimatedCostUSD = a.cumulativeCostUSD
	if _, found := GetPricing(a.model); found {
		result.CostSource = "known_pricing"
		result.CostStatus = "estimated"
	} else {
		result.CostSource = "unknown_model"
		result.CostStatus = "unavailable"
	}

	// End session
	if a.sessionDB != nil && result.Completed {
		a.sessionDB.EndSession(a.sessionID, "completed")
	}

	// Self-improvement: record turn and trigger async review when due.
	if a.selfImprover != nil && result.Completed {
		if shouldReview := a.selfImprover.RecordTurn(); shouldReview {
			msgsCopy := make([]llm.Message, len(messages))
			copy(msgsCopy, messages)
			go func(msgs []llm.Message, tid, uid string) {
				ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel2()
				if _, err := a.selfImprover.Review(ctx2, msgs, tid, uid); err != nil {
					slog.Warn("Self-improvement review failed", "error", err)
				}
			}(msgsCopy, a.tenantID, a.userID)
		}
	}

	// Oris evolution: record outcome and persist gene insights asynchronously.
	if a.evolutionImprover != nil && result.Completed {
		msgsCopy := make([]llm.Message, len(messages)) // defensive copy: goroutine must not share backing array
		copy(msgsCopy, messages)
		go func(msgs []llm.Message, tenantID string, completed bool) {
			ctx2, cancel2 := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel2()
			a.evolutionImprover.PostTurnRecord(ctx2, tenantID, msgs, completed)
		}(msgsCopy, a.tenantID, result.Completed)
	}

	return result, nil
}

// prependEvolutionContext injects gene strategy texts as a system context message
// before the conversation messages.
func prependEvolutionContext(messages []llm.Message, strategies []string) []llm.Message {
	if len(strategies) == 0 {
		return messages
	}
	var sb strings.Builder
	sb.WriteString("## Behavioral Strategies (from experience)\n")
	for i, s := range strategies {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
	}
	hint := llm.Message{Role: "system", Content: sb.String()}
	result := make([]llm.Message, 0, len(messages)+1)
	result = append(result, hint)
	result = append(result, messages...)
	return result
}

// Interrupt requests the agent to stop after the current tool call (lock-free).
func (a *AIAgent) Interrupt() {
	a.interruptRequested.Store(true)
}

func (a *AIAgent) isInterrupted() bool {
	return a.interruptRequested.Load()
}

// Steer injects a user message into the conversation at the next safe point.
func (a *AIAgent) Steer(prompt string) {
	a.steerMu.Lock()
	defer a.steerMu.Unlock()
	a.steerMessage = prompt
}

func (a *AIAgent) consumeSteer() string {
	a.steerMu.Lock()
	defer a.steerMu.Unlock()
	msg := a.steerMessage
	a.steerMessage = ""
	return msg
}

// Heartbeat returns a channel that receives a signal on each LLM call or tool execution.
func (a *AIAgent) Heartbeat() <-chan struct{} {
	if a.heartbeatCh == nil {
		a.heartbeatCh = make(chan struct{}, 1)
	}
	return a.heartbeatCh
}

func (a *AIAgent) fireHeartbeat() {
	if a.heartbeatCh == nil {
		return
	}
	select {
	case a.heartbeatCh <- struct{}{}:
	default:
	}
}

// SessionID returns the current session ID.
func (a *AIAgent) SessionID() string {
	return a.sessionID
}

// Model returns the current model.
func (a *AIAgent) Model() string {
	return a.model
}

// Callbacks returns the current stream callbacks.
func (a *AIAgent) Callbacks() *StreamCallbacks {
	return a.callbacks
}

// Close cleans up resources.
func (a *AIAgent) Close() {
	if a.sessionDB != nil {
		a.sessionDB.Close()
	}
}

// executeToolCalls runs tool calls, parallelizing when safe.
// Uses smart path-based overlap detection for file-scoped tools.
func (a *AIAgent) executeToolCalls(ctx context.Context, toolCalls []llm.ToolCall) []llm.Message {
	if len(toolCalls) == 1 || !ShouldParallelizeToolBatch(toolCalls) {
		// Sequential execution
		var results []llm.Message
		for _, tc := range toolCalls {
			if a.isInterrupted() {
				break
			}
			results = append(results, a.executeSingleTool(ctx, tc))
		}
		return results
	}

	// Parallel execution with WaitGroup + timeout
	type indexedResult struct {
		index int
		msg   llm.Message
	}

	parallelCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var wg sync.WaitGroup
	resultCh := make(chan indexedResult, len(toolCalls))
	sem := make(chan struct{}, MaxParallelWorkers)

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, call llm.ToolCall) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Tool panicked", "tool", call.Function.Name, "panic", r)
					resultCh <- indexedResult{index: idx, msg: llm.Message{
						Role:       "tool",
						Content:    fmt.Sprintf(`{"error":"tool panicked: %v"}`, r),
						ToolCallID: call.ID,
						ToolName:   call.Function.Name,
					}}
				}
			}()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				msg := a.executeSingleTool(parallelCtx, call)
				resultCh <- indexedResult{index: idx, msg: msg}
			case <-parallelCtx.Done():
				resultCh <- indexedResult{index: idx, msg: llm.Message{
					Role:       "tool",
					Content:    `{"error":"tool execution timed out"}`,
					ToolCallID: call.ID,
					ToolName:   call.Function.Name,
				}}
			}
		}(i, tc)
	}

	go func() { wg.Wait(); close(resultCh) }()

	collected := make([]llm.Message, len(toolCalls))
	for ir := range resultCh {
		collected[ir.index] = ir.msg
	}

	return collected
}

func (a *AIAgent) executeSingleTool(ctx context.Context, tc llm.ToolCall) llm.Message {
	toolName := tc.Function.Name
	a.fireToolStart(toolName)
	a.fireToolProgress(toolName, truncate(tc.Function.Arguments, 100))

	args, err := llm.ParseToolArgs(tc.Function.Arguments)
	if err != nil {
		args = map[string]any{}
		slog.Warn("Failed to parse tool args", "tool", toolName, "error", err)
	}

	toolCtx := &tools.ToolContext{
		SessionID:      a.sessionID,
		ToolCallID:     tc.ID,
		Platform:       a.platform,
		TenantID:       a.tenantID,
		UserID:         a.userID,
		MemoryProvider: a.memoryProvider,
	}

	toolResult := tools.Registry().Dispatch(ctx, toolName, args, toolCtx)

	// Redact secrets before the result enters conversation history
	toolResult = RedactSecrets(toolResult)

	// Save oversized results to disk
	if IsOversizedResult(toolResult) {
		slog.Info("Tool result oversized, saving to file", "tool", toolName, "chars", len(toolResult))
		toolResult = SaveOversizedResult(toolName, toolResult)
	}

	a.fireToolComplete(toolName)

	return llm.Message{
		Role:       "tool",
		Content:    toolResult,
		ToolCallID: tc.ID,
		ToolName:   toolName,
	}
}

func (a *AIAgent) buildAPIMessages(messages []llm.Message) []llm.Message {
	apiMessages := make([]llm.Message, 0, len(messages)+1)

	// System prompt
	apiMessages = append(apiMessages, llm.Message{
		Role:    "system",
		Content: a.systemPrompt,
	})

	// Conversation messages
	apiMessages = append(apiMessages, messages...)

	return apiMessages
}

func (a *AIAgent) buildToolDefs(cfg *config.Config) {
	// Resolve which tools to enable
	toolNames := resolveTools(a.enabledToolsets, a.disabledToolsets)
	a.validTools = toolNames

	// Get OpenAI-format definitions
	defs := tools.Registry().GetDefinitions(toolNames, a.quietMode)

	a.toolDefs = make([]llm.ToolDef, 0, len(defs))
	for _, d := range defs {
		fnDef, ok := d["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := fnDef["name"].(string)
		desc, _ := fnDef["description"].(string)
		var params map[string]any
		if p, ok := fnDef["parameters"]; ok {
			if pm, ok := p.(map[string]any); ok {
				params = pm
			} else {
				b, _ := json.Marshal(p)
				json.Unmarshal(b, &params)
			}
		}
		a.toolDefs = append(a.toolDefs, llm.ToolDef{
			Name:        name,
			Description: desc,
			Parameters:  params,
		})
	}
}

func (a *AIAgent) streamingAPICall(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	deltaCh, errCh := a.client.CreateChatCompletionStream(ctx, req)
	return a.consumeStream(deltaCh, errCh)
}

// consumeStream drains a streaming delta channel and collects the response.
func (a *AIAgent) consumeStream(deltaCh <-chan llm.StreamDelta, errCh <-chan error) (*llm.ChatResponse, error) {
	resp := &llm.ChatResponse{}
	var contentBuilder []byte

	for delta := range deltaCh {
		if delta.Done {
			resp.ToolCalls = delta.ToolCalls
			break
		}

		if delta.Content != "" {
			contentBuilder = append(contentBuilder, delta.Content...)
			a.fireStreamDelta(delta.Content)
		}

		if delta.Reasoning != "" {
			a.fireReasoning(delta.Reasoning)
			resp.Reasoning += delta.Reasoning
		}
	}

	// Block until the stream wrapper closes errCh (always happens after deltaCh drains).
	if err := <-errCh; err != nil {
		return nil, err
	}

	resp.Content = string(contentBuilder)

	if len(resp.ToolCalls) > 0 {
		resp.FinishReason = "tool_calls"
	} else {
		resp.FinishReason = "stop"
	}

	return resp, nil
}

func resolveTools(enabled, disabled []string) map[string]bool {
	var toolList []string

	if len(enabled) > 0 {
		toolList = toolsets.ResolveMultipleToolsets(enabled)
	} else {
		// Default: use hermes-cli toolset (which equals CoreTools)
		toolList = toolsets.ResolveToolset("hermesx-cli")
	}

	result := make(map[string]bool, len(toolList))
	for _, t := range toolList {
		result[t] = true
	}

	// Remove disabled toolset tools
	if len(disabled) > 0 {
		disabledTools := toolsets.ResolveMultipleToolsets(disabled)
		for _, t := range disabledTools {
			delete(result, t)
		}
	}

	return result
}

// ResumeSession loads history from a previous session and resumes it.
func (a *AIAgent) ResumeSession(sessionID string) error {
	a.resumeSessionID = sessionID
	return a.loadResumedSession()
}

// loadResumedSession loads messages from the session DB for a resumed session.
func (a *AIAgent) loadResumedSession() error {
	if a.sessionDB == nil {
		return fmt.Errorf("session DB not available")
	}
	if a.resumeSessionID == "" {
		return nil
	}

	// Verify the session exists
	sess, err := a.sessionDB.GetSession(a.resumeSessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}
	if sess == nil {
		return fmt.Errorf("session %s not found", a.resumeSessionID)
	}

	// Use the resumed session's ID going forward
	a.sessionID = a.resumeSessionID

	slog.Info("Resumed session", "session_id", a.sessionID)
	return nil
}

// tryFallbackModels attempts each fallback model in order after the primary fails.
func (a *AIAgent) tryFallbackModels(ctx context.Context, req llm.ChatRequest, primaryErr error) (*llm.ChatResponse, error) {
	if len(a.fallbackModels) == 0 {
		return nil, primaryErr
	}

	for _, fb := range a.fallbackModels {
		slog.Warn("Primary model failed, trying fallback",
			"primary_error", primaryErr,
			"fallback_model", fb.Model)

		apiKey := fb.APIKey
		if apiKey == "" {
			apiKey = a.apiKey
		}
		baseURL := fb.BaseURL
		if baseURL == "" {
			baseURL = a.baseURL
		}

		fbClient, err := llm.NewClientWithParams(fb.Model, baseURL, apiKey, fb.Provider)
		if err != nil {
			slog.Warn("Failed to create fallback client", "model", fb.Model, "error", err)
			continue
		}

		var resp *llm.ChatResponse
		var fbErr error

		if req.Stream && a.hasStreamConsumers() {
			deltaCh, errCh := fbClient.CreateChatCompletionStream(ctx, req)
			resp, fbErr = a.consumeStream(deltaCh, errCh)
		} else {
			resp, fbErr = fbClient.CreateChatCompletion(ctx, req)
		}

		if fbErr != nil {
			slog.Warn("Fallback model also failed", "model", fb.Model, "error", fbErr)
			primaryErr = fbErr
			continue
		}

		slog.Info("Fallback model succeeded", "model", fb.Model)
		return resp, nil
	}

	return nil, primaryErr
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
