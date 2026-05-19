package eino

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	agentpkg "github.com/cloudwego/eino/flow/agent"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	template "github.com/cloudwego/eino/utils/callbacks"

	"github.com/Colin4k1024/hermesx/internal/eino/ctxkeys"
	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

// buildCallbackOption constructs an Eino AgentOption that bridges Eino's callback
// system to HermesX StreamCallbacks for tool lifecycle events.
func buildCallbackOption(cb *StreamCallbacks) agentpkg.AgentOption {
	if cb == nil {
		return agentpkg.AgentOption{}
	}

	modelHandler := &template.ModelCallbackHandler{}
	toolHandler := &template.ToolCallbackHandler{}

	if cb.OnStep != nil {
		modelHandler.OnStart = func(ctx context.Context, info *callbacks.RunInfo, input *model.CallbackInput) context.Context {
			cb.OnStep(0, nil)
			return ctx
		}
	}

	if cb.OnToolStart != nil {
		toolHandler.OnStart = func(ctx context.Context, info *callbacks.RunInfo, input *tool.CallbackInput) context.Context {
			name := ""
			if info != nil {
				name = info.Name
			}
			cb.OnToolStart(name)
			return ctx
		}
	}

	if cb.OnToolComplete != nil {
		toolHandler.OnEnd = func(ctx context.Context, info *callbacks.RunInfo, output *tool.CallbackOutput) context.Context {
			name := ""
			if info != nil {
				name = info.Name
			}
			cb.OnToolComplete(name)
			return ctx
		}
	}

	if cb.OnError != nil {
		toolHandler.OnError = func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			cb.OnError(err)
			return ctx
		}
		modelHandler.OnError = func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			cb.OnError(err)
			return ctx
		}
	}

	handler := react.BuildAgentCallback(modelHandler, toolHandler)
	return agentpkg.WithComposeOptions(compose.WithCallbacks(handler))
}

// RunConversationWithCallbacks runs the agent with Eino-native callbacks enabled.
func (e *EinoAgent) RunConversationWithCallbacks(ctx context.Context, userMessage string, history []llm.Message) (*ConversationResult, error) {
	if e.callbacks == nil {
		return e.RunConversation(ctx, userMessage, history)
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

	opt := buildCallbackOption(e.callbacks)
	msg, err := e.agent.Generate(ctx, input, opt)
	if err != nil {
		slog.Error("eino agent generate with callbacks failed", "error", err)
		return nil, fmt.Errorf("eino agent: generate: %w", err)
	}

	return &ConversationResult{
		FinalResponse: msg.Content,
		LastReasoning: msg.ReasoningContent,
		Completed:     true,
		Model:         e.config.modelName,
	}, nil
}
