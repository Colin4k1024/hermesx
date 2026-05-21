package eino

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/Colin4k1024/hermesx/internal/eino/ctxkeys"
	"github.com/Colin4k1024/hermesx/internal/eino/modeladapter"
	"github.com/Colin4k1024/hermesx/internal/eino/tooladapter"
	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/safety"
	"github.com/Colin4k1024/hermesx/internal/secrets"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

// FallbackModel describes an alternative model for ADK model failover.
type FallbackModel struct {
	Model    string
	Provider string
	BaseURL  string
	APIKey   string
	APIMode  string
}

// ConversationResult holds the result of an Eino agent conversation turn.
type ConversationResult struct {
	FinalResponse    string         `json:"final_response"`
	LastReasoning    string         `json:"last_reasoning,omitempty"`
	Messages         []llm.Message  `json:"messages"`
	APICalls         int            `json:"api_calls"`
	Completed        bool           `json:"completed"`
	Interrupted      bool           `json:"interrupted"`
	Model            string         `json:"model"`
	Provider         string         `json:"provider,omitempty"`
	BaseURL          string         `json:"base_url,omitempty"`
	InputTokens      int            `json:"input_tokens"`
	OutputTokens     int            `json:"output_tokens"`
	CacheReadTokens  int            `json:"cache_read_tokens"`
	CacheWriteTokens int            `json:"cache_write_tokens"`
	ReasoningTokens  int            `json:"reasoning_tokens"`
	TotalTokens      int            `json:"total_tokens"`
	AgenticBlocks    []AgenticBlock `json:"agentic_blocks,omitempty"`
}

// StreamCallbacks holds callback functions for streaming events.
type StreamCallbacks struct {
	OnStreamDelta  func(text string)
	OnReasoning    func(text string)
	OnAgenticBlock func(block AgenticBlock)
	OnToolStart    func(toolName string)
	OnToolComplete func(toolName string)
	OnStep         func(iteration int, prevTools []string)
	OnStatus       func(msg string)
	OnError        func(err error)
}

// EinoAgent wraps Eino ADK ChatModelAgent with HermesX production features.
type EinoAgent struct {
	agent        *adk.ChatModelAgent
	runner       *adk.Runner
	streamRunner *adk.Runner
	config       *agentConfig
	callbacks    *StreamCallbacks
	capture      *Capture
	runMu        sync.Mutex
}

type agentConfig struct {
	transport         llm.Transport
	modelName         string
	provider          string
	baseURL           string
	apiKey            string
	apiMode           string
	fallbackModels    []FallbackModel
	toolEntries       []*tools.ToolEntry
	maxIterations     int
	systemPrompt      string
	tenantID          string
	userID            string
	sessionID         string
	platform          string
	memoryProvider    tools.MemoryProvider
	checkpointStore   adk.CheckPointStore
	safetyInterceptor safety.SafetyInterceptor
	leakScanner       *secrets.LeakScanner
}

var agenticProviderModelFactory = NewAgenticProviderModel

// NewEinoAgent constructs a production Eino ADK-based agent.
func NewEinoAgent(ctx context.Context, opts ...Option) (*EinoAgent, error) {
	cfg := &agentConfig{
		maxIterations: 20,
		platform:      "api",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.transport == nil {
		return nil, fmt.Errorf("eino agent: transport is required")
	}

	capture := NewCapture()
	wrappedModel := buildChatModel(ctx, cfg, capture)

	wrappedTools := make([]einotool.BaseTool, 0, len(cfg.toolEntries))
	for _, entry := range cfg.toolEntries {
		wrappedTools = append(wrappedTools, tooladapter.Wrap(entry))
	}

	handler := &runtimeHandler{
		callbacks: func() *StreamCallbacks { return nil },
	}

	agentCfg := &adk.ChatModelAgentConfig{
		Name:        "hermesx-chat",
		Description: "HermesX SaaS conversation agent",
		Instruction: cfg.systemPrompt,
		Model:       wrappedModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: wrappedTools},
		},
		MaxIterations:       cfg.maxIterations,
		Handlers:            []adk.ChatModelAgentMiddleware{handler},
		ModelRetryConfig:    newRetryConfig(),
		ModelFailoverConfig: newFailoverConfig(cfg, capture),
	}

	adkAgent, err := adk.NewChatModelAgent(ctx, agentCfg)
	if err != nil {
		return nil, fmt.Errorf("eino agent: create ADK chat model agent: %w", err)
	}

	e := &EinoAgent{
		agent:   adkAgent,
		config:  cfg,
		capture: capture,
	}
	handler.callbacks = func() *StreamCallbacks { return e.callbacks }
	e.runner = adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           adkAgent,
		EnableStreaming: false,
		CheckPointStore: cfg.checkpointStore,
	})
	e.streamRunner = adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           adkAgent,
		EnableStreaming: true,
		CheckPointStore: cfg.checkpointStore,
	})

	return e, nil
}

// SetCallbacks sets streaming callbacks (can be called after construction).
func (e *EinoAgent) SetCallbacks(cb *StreamCallbacks) {
	e.runMu.Lock()
	defer e.runMu.Unlock()
	e.callbacks = cb
}

// RunConversation runs a full conversation turn with tool calling.
func (e *EinoAgent) RunConversation(ctx context.Context, userMessage string, history []llm.Message) (*ConversationResult, error) {
	e.runMu.Lock()
	defer e.runMu.Unlock()
	return e.run(ctx, e.runner, userMessage, history)
}

// Stream runs the conversation in streaming mode, firing callbacks as content arrives.
func (e *EinoAgent) Stream(ctx context.Context, userMessage string, history []llm.Message) (*ConversationResult, error) {
	e.runMu.Lock()
	defer e.runMu.Unlock()
	return e.run(ctx, e.streamRunner, userMessage, history)
}

// StreamSafe applies safety hooks around the streaming agent call.
func (e *EinoAgent) StreamSafe(ctx context.Context, userMessage string, history []llm.Message) (*ConversationResult, error) {
	e.runMu.Lock()
	defer e.runMu.Unlock()
	return e.streamSafeLocked(ctx, userMessage, history)
}

func (e *EinoAgent) streamSafeLocked(ctx context.Context, userMessage string, history []llm.Message) (*ConversationResult, error) {
	if err := e.checkInput(ctx, userMessage); err != nil {
		return nil, err
	}
	redactionHook := NewRedactionHook(e.config.leakScanner)
	originalCallbacks := e.callbacks
	var safeWriter *safeStreamWriter
	if originalCallbacks != nil && originalCallbacks.OnStreamDelta != nil {
		cbCopy := *originalCallbacks
		safeWriter = newSafeStreamWriter(redactionHook, originalCallbacks)
		cbCopy.OnStreamDelta = func(text string) {
			safeWriter.Write(text)
		}
		e.callbacks = &cbCopy
		defer func() { e.callbacks = originalCallbacks }()
	}
	result, err := e.run(ctx, e.streamRunner, userMessage, history)
	if err != nil {
		return nil, err
	}
	if safeWriter != nil {
		_ = safeWriter.Flush()
	}
	return e.checkOutput(ctx, result)
}

// RunConversationSafe applies safety hooks around the agent generate call.
func (e *EinoAgent) RunConversationSafe(ctx context.Context, userMessage string, history []llm.Message) (*ConversationResult, error) {
	e.runMu.Lock()
	defer e.runMu.Unlock()
	if e.callbacks != nil {
		return e.streamSafeLocked(ctx, userMessage, history)
	}
	if err := e.checkInput(ctx, userMessage); err != nil {
		return nil, err
	}
	result, err := e.run(ctx, e.runner, userMessage, history)
	if err != nil {
		return nil, err
	}
	return e.checkOutput(ctx, result)
}

func (e *EinoAgent) run(ctx context.Context, runner *adk.Runner, userMessage string, history []llm.Message) (*ConversationResult, error) {
	e.capture.Reset()
	input := make([]*schema.Message, 0, len(history)+1)
	for _, msg := range history {
		input = append(input, convertLLMToSchema(&msg))
	}
	input = append(input, schema.UserMessage(userMessage))

	tctx := &tools.ToolContext{
		SessionID:      e.config.sessionID,
		TenantID:       e.config.tenantID,
		UserID:         e.config.userID,
		Platform:       e.config.platform,
		MemoryProvider: e.config.memoryProvider,
	}
	ctx = ctxkeys.WithToolContext(ctx, tctx)
	ctx = llm.WithTenantID(ctx, e.config.tenantID)

	if e.callbacks != nil && e.callbacks.OnStatus != nil {
		e.callbacks.OnStatus("starting agent")
	}
	if e.callbacks != nil && e.callbacks.OnStep != nil {
		e.callbacks.OnStep(0, nil)
	}

	cancelOpt, cancelFn := adk.WithCancel()
	opts := []adk.AgentRunOption{cancelOpt}
	if e.config.checkpointStore != nil && e.config.tenantID != "" && e.config.sessionID != "" {
		opts = append(opts, adk.WithCheckPointID(e.checkpointID()))
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			handle, ok := cancelFn(
				adk.WithAgentCancelMode(adk.CancelAfterChatModel|adk.CancelAfterToolCalls),
				adk.WithAgentCancelTimeout(2*time.Second),
				adk.WithRecursive(),
			)
			if ok && handle != nil {
				_ = handle.Wait()
			}
		case <-done:
			return
		}
	}()

	iter := runner.Run(ctx, input, opts...)
	result := &ConversationResult{
		Model:    e.config.modelName,
		Provider: e.config.provider,
		BaseURL:  e.config.baseURL,
	}
	var finalMessages []*schema.Message
	var finalContent strings.Builder
	var finalReasoning strings.Builder
	var apiCalls int

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			var cancelErr *adk.CancelError
			if errors.As(event.Err, &cancelErr) {
				result.Interrupted = true
			}
			if e.callbacks != nil && e.callbacks.OnError != nil {
				e.callbacks.OnError(event.Err)
			}
			return result, event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		msg, err := e.consumeMessageVariant(event.Output.MessageOutput)
		if err != nil {
			return nil, err
		}
		if msg == nil {
			continue
		}
		finalMessages = append(finalMessages, msg)
		e.capture.AddBlocks(AgenticBlocksFromMessage(msg))
		e.emitBlocks(AgenticBlocksFromMessage(msg))
		if msg.Role == schema.Assistant {
			apiCalls++
			if len(msg.ToolCalls) == 0 {
				finalContent.WriteString(msg.Content)
				finalReasoning.WriteString(msg.ReasoningContent)
			}
			addUsage(result, msg.ResponseMeta)
		}
	}

	result.FinalResponse = finalContent.String()
	result.LastReasoning = finalReasoning.String()
	result.Messages = SchemaMessagesToLLM(finalMessages)
	result.APICalls = apiCalls
	result.Completed = !result.Interrupted
	result.TotalTokens = result.InputTokens + result.OutputTokens
	result.AgenticBlocks = e.capture.Blocks()
	return result, nil
}

func (e *EinoAgent) consumeMessageVariant(mv *adk.MessageVariant) (*schema.Message, error) {
	if mv == nil {
		return nil, nil
	}
	if !mv.IsStreaming {
		msg, err := mv.GetMessage()
		if err != nil {
			return nil, err
		}
		if msg != nil {
			e.emitMessageCallbacks(msg, mv.Role, mv.ToolName)
		}
		return msg, nil
	}
	var chunks []*schema.Message
	for {
		chunk, err := mv.MessageStream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if chunk == nil {
			continue
		}
		chunks = append(chunks, chunk)
		e.emitMessageCallbacks(chunk, mv.Role, mv.ToolName)
	}
	if len(chunks) == 0 {
		return nil, nil
	}
	msg, err := schema.ConcatMessages(chunks)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (e *EinoAgent) emitMessageCallbacks(msg *schema.Message, role schema.RoleType, toolName string) {
	if e.callbacks == nil || msg == nil {
		return
	}
	if role == schema.Tool {
		name := toolName
		if name == "" {
			name = msg.ToolName
		}
		if name != "" && e.callbacks.OnToolComplete != nil {
			e.callbacks.OnToolComplete(name)
		}
		return
	}
	if msg.Content != "" && e.callbacks.OnStreamDelta != nil {
		e.callbacks.OnStreamDelta(msg.Content)
	}
	if msg.ReasoningContent != "" && e.callbacks.OnReasoning != nil {
		e.callbacks.OnReasoning(msg.ReasoningContent)
	}
}

func (e *EinoAgent) emitBlocks(blocks []AgenticBlock) {
	if e.callbacks == nil || e.callbacks.OnAgenticBlock == nil {
		return
	}
	for _, block := range blocks {
		e.callbacks.OnAgenticBlock(block)
	}
}

func (e *EinoAgent) checkpointID() string {
	return e.config.tenantID + "/" + e.config.sessionID
}

func (e *EinoAgent) checkInput(ctx context.Context, userMessage string) error {
	safetyHook := NewSafetyHook(e.config.safetyInterceptor)
	if err := safetyHook.CheckInput(ctx, e.config.tenantID, userMessage); err != nil {
		return fmt.Errorf("safety: %w", err)
	}
	return nil
}

func (e *EinoAgent) checkOutput(ctx context.Context, result *ConversationResult) (*ConversationResult, error) {
	safetyHook := NewSafetyHook(e.config.safetyInterceptor)
	redactionHook := NewRedactionHook(e.config.leakScanner)
	checkedOutput, err := safetyHook.CheckOutput(ctx, e.config.tenantID, result.FinalResponse)
	if err != nil {
		return nil, fmt.Errorf("safety: %w", err)
	}
	result.FinalResponse = redactionHook.RedactToolOutput(checkedOutput)
	return result, nil
}

type runtimeHandler struct {
	*adk.BaseChatModelAgentMiddleware
	callbacks func() *StreamCallbacks
}

func (h *runtimeHandler) WrapInvokableToolCall(ctx context.Context, endpoint adk.InvokableToolCallEndpoint, tCtx *adk.ToolContext) (adk.InvokableToolCallEndpoint, error) {
	return func(ctx context.Context, argumentsInJSON string, opts ...einotool.Option) (string, error) {
		if cb := h.callbacks(); cb != nil && cb.OnToolStart != nil {
			cb.OnToolStart(tCtx.Name)
		}
		out, err := endpoint(ctx, argumentsInJSON, opts...)
		if err != nil {
			return out, err
		}
		return out, nil
	}, nil
}

func (h *runtimeHandler) AfterAgent(ctx context.Context, state *adk.ChatModelAgentState) (context.Context, error) {
	if cb := h.callbacks(); cb != nil && cb.OnStatus != nil {
		cb.OnStatus("agent completed")
	}
	return ctx, nil
}

func newRetryConfig() *adk.ModelRetryConfig {
	return &adk.ModelRetryConfig{
		MaxRetries: 3,
		ShouldRetry: func(ctx context.Context, retryCtx *adk.RetryContext) *adk.RetryDecision {
			if retryCtx == nil {
				return nil
			}
			if retryCtx.Err != nil {
				return &adk.RetryDecision{Retry: true, Backoff: retryBackoff(retryCtx.RetryAttempt), RejectReason: retryCtx.Err.Error()}
			}
			msg := retryCtx.OutputMessage
			if msg == nil {
				modified := append([]*schema.Message{}, retryCtx.InputMessages...)
				modified = append(modified, schema.UserMessage("Please continue with your response or use a tool to make progress."))
				return &adk.RetryDecision{Retry: true, ModifiedInputMessages: modified, Backoff: retryBackoff(retryCtx.RetryAttempt), RejectReason: "empty output"}
			}
			if msg.Content == "" && len(msg.ToolCalls) == 0 {
				modified := append([]*schema.Message{}, retryCtx.InputMessages...)
				modified = append(modified, schema.UserMessage("Please continue with your response or use a tool to make progress."))
				return &adk.RetryDecision{Retry: true, ModifiedInputMessages: modified, Backoff: retryBackoff(retryCtx.RetryAttempt), RejectReason: "empty response"}
			}
			if msg.ResponseMeta != nil && msg.ResponseMeta.FinishReason == "length" {
				return &adk.RetryDecision{
					Retry:             true,
					AdditionalOptions: []model.Option{model.WithMaxTokens(4096)},
					Backoff:           retryBackoff(retryCtx.RetryAttempt),
					RejectReason:      "finish_reason=length",
				}
			}
			return nil
		},
	}
}

func retryBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 5 {
		attempt = 5
	}
	return time.Duration(attempt*attempt) * 100 * time.Millisecond
}

func buildChatModel(ctx context.Context, cfg *agentConfig, capture *Capture) model.BaseModel[*schema.Message] {
	if cfg == nil {
		return nil
	}
	return buildChatModelWithConfig(ctx, cfg.transport, cfg.modelName, cfg.provider, cfg.baseURL, cfg.apiKey, cfg.apiMode, capture)
}

func buildChatModelWithConfig(ctx context.Context, transport llm.Transport, modelName, provider, baseURL, apiKey, apiMode string, capture *Capture) model.BaseModel[*schema.Message] {
	if modelName != "" && apiKey != "" {
		agenticModel, err := agenticProviderModelFactory(ctx, AgenticProviderConfig{
			Provider: provider,
			APIMode:  apiMode,
			BaseURL:  baseURL,
			APIKey:   apiKey,
			Model:    modelName,
		})
		if err == nil && agenticModel != nil {
			return modeladapter.WrapAgentic(agenticModel, capture)
		}
		if err != nil {
			slog.Debug("eino_agentic_model_fallback", "model", modelName, "provider", provider, "api_mode", apiMode, "error", err)
		}
	}
	return modeladapter.Wrap(transport, modelName)
}

func newFailoverConfig(cfg *agentConfig, capture *Capture) *adk.ModelFailoverConfig[*schema.Message] {
	if cfg == nil || len(cfg.fallbackModels) == 0 {
		return nil
	}
	return &adk.ModelFailoverConfig[*schema.Message]{
		MaxRetries: uint(len(cfg.fallbackModels)),
		ShouldFailover: func(ctx context.Context, _ *schema.Message, err error) bool {
			if err == nil || ctx.Err() != nil {
				return false
			}
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "401") || strings.Contains(msg, "403") || strings.Contains(msg, "api key") || strings.Contains(msg, "unauthorized") {
				return false
			}
			return true
		},
		GetFailoverModel: func(ctx context.Context, fctx *adk.FailoverContext[*schema.Message]) (model.BaseModel[*schema.Message], []*schema.Message, error) {
			idx := int(fctx.FailoverAttempt) - 1
			if idx < 0 || idx >= len(cfg.fallbackModels) {
				return nil, nil, nil
			}
			fb := cfg.fallbackModels[idx]
			apiKey := fb.APIKey
			if apiKey == "" {
				apiKey = cfg.apiKey
			}
			baseURL := fb.BaseURL
			if baseURL == "" {
				baseURL = cfg.baseURL
			}
			provider := fb.Provider
			if provider == "" {
				provider = cfg.provider
			}
			var client *llm.Client
			var err error
			if fb.APIMode != "" {
				client, err = llm.NewClientWithMode(fb.Model, baseURL, apiKey, provider, llm.APIMode(fb.APIMode))
			} else {
				client, err = llm.NewClientWithParams(fb.Model, baseURL, apiKey, provider)
			}
			if err != nil {
				return nil, nil, err
			}
			return buildChatModelWithConfig(ctx, client.GetTransport(), fb.Model, provider, baseURL, apiKey, fb.APIMode, capture), nil, nil
		},
	}
}

func addUsage(result *ConversationResult, meta *schema.ResponseMeta) {
	if result == nil || meta == nil || meta.Usage == nil {
		return
	}
	result.InputTokens += meta.Usage.PromptTokens
	result.OutputTokens += meta.Usage.CompletionTokens
	result.CacheReadTokens += meta.Usage.PromptTokenDetails.CachedTokens
	result.ReasoningTokens += meta.Usage.CompletionTokensDetails.ReasoningTokens
}

func convertLLMToSchema(msg *llm.Message) *schema.Message {
	m := &schema.Message{
		Role:             schema.RoleType(msg.Role),
		Content:          msg.Content,
		ToolCallID:       msg.ToolCallID,
		ToolName:         msg.ToolName,
		ReasoningContent: msg.ReasoningContent,
	}
	if m.ReasoningContent == "" {
		m.ReasoningContent = msg.Reasoning
	}
	if msg.FinishReason != "" {
		m.ResponseMeta = &schema.ResponseMeta{FinishReason: msg.FinishReason}
	}
	if len(msg.ToolCalls) > 0 {
		m.ToolCalls = make([]schema.ToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			m.ToolCalls[i] = schema.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: schema.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}
	return m
}

func init() {
	schema.RegisterName[AgenticBlock]("hermesx_eino_agentic_block")
}
