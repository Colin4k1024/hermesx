package eino

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"github.com/Colin4k1024/hermesx/internal/eino/ctxkeys"
	"github.com/Colin4k1024/hermesx/internal/eino/modeladapter"
	"github.com/Colin4k1024/hermesx/internal/eino/tooladapter"
	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/safety"
	"github.com/Colin4k1024/hermesx/internal/secrets"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

// ConversationResult holds the result of an Eino agent conversation turn.
type ConversationResult struct {
	FinalResponse string        `json:"final_response"`
	LastReasoning string        `json:"last_reasoning,omitempty"`
	Messages      []llm.Message `json:"messages"`
	APICalls      int           `json:"api_calls"`
	Completed     bool          `json:"completed"`
	Model         string        `json:"model"`
}

// StreamCallbacks holds callback functions for streaming events.
type StreamCallbacks struct {
	OnStreamDelta  func(text string)
	OnReasoning    func(text string)
	OnToolStart    func(toolName string)
	OnToolComplete func(toolName string)
	OnStep         func(iteration int, prevTools []string)
	OnStatus       func(msg string)
	OnError        func(err error)
}

// EinoAgent wraps Eino's ReAct agent with HermesX production features.
type EinoAgent struct {
	agent     *react.Agent
	config    *agentConfig
	callbacks *StreamCallbacks
}

type agentConfig struct {
	transport         llm.Transport
	modelName         string
	toolEntries       []*tools.ToolEntry
	maxIterations     int
	systemPrompt      string
	tenantID          string
	userID            string
	sessionID         string
	safetyInterceptor safety.SafetyInterceptor
	leakScanner       *secrets.LeakScanner
}

// NewEinoAgent constructs a production Eino-based agent.
func NewEinoAgent(ctx context.Context, opts ...Option) (*EinoAgent, error) {
	cfg := &agentConfig{
		maxIterations: 20,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.transport == nil {
		return nil, fmt.Errorf("eino agent: transport is required")
	}

	wrappedModel := modeladapter.Wrap(cfg.transport, cfg.modelName)

	wrappedTools := make([]tool.BaseTool, 0, len(cfg.toolEntries))
	for _, entry := range cfg.toolEntries {
		wrappedTools = append(wrappedTools, tooladapter.Wrap(entry))
	}

	agentCfg := &react.AgentConfig{
		ToolCallingModel: wrappedModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: wrappedTools,
		},
		MaxStep: cfg.maxIterations,
	}

	if cfg.systemPrompt != "" {
		agentCfg.MessageModifier = func(_ context.Context, input []*schema.Message) []*schema.Message {
			out := make([]*schema.Message, 0, len(input)+1)
			out = append(out, schema.SystemMessage(cfg.systemPrompt))
			out = append(out, input...)
			return out
		}
	}

	agent, err := react.NewAgent(ctx, agentCfg)
	if err != nil {
		return nil, fmt.Errorf("eino agent: create react agent: %w", err)
	}

	return &EinoAgent{
		agent:  agent,
		config: cfg,
	}, nil
}

// SetCallbacks sets streaming callbacks (can be called after construction).
func (e *EinoAgent) SetCallbacks(cb *StreamCallbacks) {
	e.callbacks = cb
}

// RunConversation runs a full conversation turn with tool calling.
func (e *EinoAgent) RunConversation(ctx context.Context, userMessage string, history []llm.Message) (*ConversationResult, error) {
	input := make([]*schema.Message, 0, len(history)+1)
	for _, msg := range history {
		input = append(input, convertLLMToSchema(&msg))
	}
	input = append(input, schema.UserMessage(userMessage))

	tctx := &tools.ToolContext{
		SessionID: e.config.sessionID,
		TenantID:  e.config.tenantID,
		UserID:    e.config.userID,
	}
	ctx = ctxkeys.WithToolContext(ctx, tctx)

	if e.callbacks != nil && e.callbacks.OnStatus != nil {
		e.callbacks.OnStatus("starting agent")
	}

	msg, err := e.agent.Generate(ctx, input)
	if err != nil {
		slog.Error("eino agent generate failed", "error", err)
		return nil, fmt.Errorf("eino agent: generate: %w", err)
	}

	result := &ConversationResult{
		FinalResponse: msg.Content,
		LastReasoning: msg.ReasoningContent,
		Completed:     true,
		Model:         e.config.modelName,
	}

	return result, nil
}

// Stream runs the conversation in streaming mode, firing callbacks as content arrives.
func (e *EinoAgent) Stream(ctx context.Context, userMessage string, history []llm.Message) (*ConversationResult, error) {
	input := make([]*schema.Message, 0, len(history)+1)
	for _, msg := range history {
		input = append(input, convertLLMToSchema(&msg))
	}
	input = append(input, schema.UserMessage(userMessage))

	tctx := &tools.ToolContext{
		SessionID: e.config.sessionID,
		TenantID:  e.config.tenantID,
		UserID:    e.config.userID,
	}
	ctx = ctxkeys.WithToolContext(ctx, tctx)

	reader, err := e.agent.Stream(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("eino agent: stream: %w", err)
	}
	defer reader.Close()

	var contentBuf strings.Builder
	var reasoningBuf strings.Builder

	for {
		msg, err := reader.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			slog.Error("eino agent stream recv error", "error", err)
			return nil, fmt.Errorf("eino agent: stream recv: %w", err)
		}
		if msg == nil {
			continue
		}
		if msg.Content != "" {
			contentBuf.WriteString(msg.Content)
			if e.callbacks != nil && e.callbacks.OnStreamDelta != nil {
				e.callbacks.OnStreamDelta(msg.Content)
			}
		}
		if msg.ReasoningContent != "" {
			reasoningBuf.WriteString(msg.ReasoningContent)
			if e.callbacks != nil && e.callbacks.OnReasoning != nil {
				e.callbacks.OnReasoning(msg.ReasoningContent)
			}
		}
	}

	return &ConversationResult{
		FinalResponse: contentBuf.String(),
		LastReasoning: reasoningBuf.String(),
		Completed:     true,
		Model:         e.config.modelName,
	}, nil
}

// StreamSafe applies safety hooks around the streaming agent call.
// Chunk-level redaction: callbacks receive redacted deltas (secrets buffered until safe boundary).
func (e *EinoAgent) StreamSafe(ctx context.Context, userMessage string, history []llm.Message) (*ConversationResult, error) {
	safetyHook := NewSafetyHook(e.config.safetyInterceptor)
	redactionHook := NewRedactionHook(e.config.leakScanner)

	if err := safetyHook.CheckInput(ctx, e.config.tenantID, userMessage); err != nil {
		return nil, fmt.Errorf("safety: %w", err)
	}

	input := make([]*schema.Message, 0, len(history)+1)
	for _, msg := range history {
		input = append(input, convertLLMToSchema(&msg))
	}
	input = append(input, schema.UserMessage(userMessage))

	tctx := &tools.ToolContext{
		SessionID: e.config.sessionID,
		TenantID:  e.config.tenantID,
		UserID:    e.config.userID,
	}
	ctx = ctxkeys.WithToolContext(ctx, tctx)

	reader, err := e.agent.Stream(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("eino agent: stream: %w", err)
	}
	defer reader.Close()

	sw := newSafeStreamWriter(redactionHook, e.callbacks)
	var reasoningBuf strings.Builder

	for {
		msg, err := reader.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			slog.Error("eino agent stream recv error", "error", err)
			return nil, fmt.Errorf("eino agent: stream recv: %w", err)
		}
		if msg == nil {
			continue
		}
		if msg.Content != "" {
			sw.Write(msg.Content)
		}
		if msg.ReasoningContent != "" {
			reasoningBuf.WriteString(msg.ReasoningContent)
			if e.callbacks != nil && e.callbacks.OnReasoning != nil {
				e.callbacks.OnReasoning(msg.ReasoningContent)
			}
		}
	}

	finalContent := sw.Flush()

	checkedOutput, err := safetyHook.CheckOutput(ctx, e.config.tenantID, finalContent)
	if err != nil {
		return nil, fmt.Errorf("safety: %w", err)
	}
	finalRedacted := redactionHook.RedactToolOutput(checkedOutput)

	return &ConversationResult{
		FinalResponse: finalRedacted,
		LastReasoning: reasoningBuf.String(),
		Completed:     true,
		Model:         e.config.modelName,
	}, nil
}

// RunConversationSafe applies safety hooks (input guard, output check, secret redaction)
// around the agent generate call.
func (e *EinoAgent) RunConversationSafe(ctx context.Context, userMessage string, history []llm.Message) (*ConversationResult, error) {
	safetyHook := NewSafetyHook(e.config.safetyInterceptor)
	redactionHook := NewRedactionHook(e.config.leakScanner)

	if err := safetyHook.CheckInput(ctx, e.config.tenantID, userMessage); err != nil {
		return nil, fmt.Errorf("safety: %w", err)
	}

	var result *ConversationResult
	var err error

	if e.callbacks != nil {
		result, err = e.RunConversationWithCallbacks(ctx, userMessage, history)
	} else {
		result, err = e.RunConversation(ctx, userMessage, history)
	}
	if err != nil {
		return nil, err
	}

	checkedOutput, err := safetyHook.CheckOutput(ctx, e.config.tenantID, result.FinalResponse)
	if err != nil {
		return nil, fmt.Errorf("safety: %w", err)
	}
	result.FinalResponse = redactionHook.RedactToolOutput(checkedOutput)

	return result, nil
}

func convertLLMToSchema(msg *llm.Message) *schema.Message {
	m := &schema.Message{
		Role:             schema.RoleType(msg.Role),
		Content:          msg.Content,
		ToolCallID:       msg.ToolCallID,
		ToolName:         msg.ToolName,
		ReasoningContent: msg.ReasoningContent,
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
