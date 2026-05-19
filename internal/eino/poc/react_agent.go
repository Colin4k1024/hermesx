package poc

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"github.com/Colin4k1024/hermesx/internal/eino/ctxkeys"
	"github.com/Colin4k1024/hermesx/internal/eino/modeladapter"
	"github.com/Colin4k1024/hermesx/internal/eino/tooladapter"
	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

// ReactAgent wraps Eino's ReAct agent with HermesX adapters.
type ReactAgent struct {
	agent *react.Agent
}

// ReactAgentConfig holds configuration for creating a ReactAgent.
type ReactAgentConfig struct {
	Transport llm.Transport
	ModelName string
	ToolSet   []*tools.ToolEntry
	MaxStep   int
	SystemMsg string
}

// NewReactAgent constructs a ReAct agent using Eino's graph orchestration,
// bridging HermesX tools and LLM transport via adapters.
func NewReactAgent(ctx context.Context, cfg ReactAgentConfig) (*ReactAgent, error) {
	wrappedModel := modeladapter.Wrap(cfg.Transport, cfg.ModelName)

	wrappedTools := make([]tool.BaseTool, 0, len(cfg.ToolSet))
	for _, entry := range cfg.ToolSet {
		wrappedTools = append(wrappedTools, tooladapter.Wrap(entry))
	}

	maxStep := cfg.MaxStep
	if maxStep <= 0 {
		maxStep = 20
	}

	agentConfig := &react.AgentConfig{
		ToolCallingModel: wrappedModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: wrappedTools,
		},
		MaxStep: maxStep,
	}

	if cfg.SystemMsg != "" {
		agentConfig.MessageModifier = func(_ context.Context, input []*schema.Message) []*schema.Message {
			out := make([]*schema.Message, 0, len(input)+1)
			out = append(out, schema.SystemMessage(cfg.SystemMsg))
			out = append(out, input...)
			return out
		}
	}

	agent, err := react.NewAgent(ctx, agentConfig)
	if err != nil {
		return nil, fmt.Errorf("create react agent: %w", err)
	}

	return &ReactAgent{agent: agent}, nil
}

// Generate runs the ReAct loop to completion and returns the final assistant message.
func (a *ReactAgent) Generate(ctx context.Context, userMsg string, tctx *tools.ToolContext) (*schema.Message, error) {
	ctx = ctxkeys.WithToolContext(ctx, tctx)
	input := []*schema.Message{
		schema.UserMessage(userMsg),
	}
	return a.agent.Generate(ctx, input)
}

// Stream runs the ReAct loop in streaming mode.
func (a *ReactAgent) Stream(ctx context.Context, userMsg string, tctx *tools.ToolContext) (*schema.StreamReader[*schema.Message], error) {
	ctx = ctxkeys.WithToolContext(ctx, tctx)
	input := []*schema.Message{
		schema.UserMessage(userMsg),
	}
	return a.agent.Stream(ctx, input)
}

// GenerateWithHistory runs the ReAct loop with full message history.
func (a *ReactAgent) GenerateWithHistory(ctx context.Context, messages []*schema.Message, tctx *tools.ToolContext) (*schema.Message, error) {
	ctx = ctxkeys.WithToolContext(ctx, tctx)
	return a.agent.Generate(ctx, messages)
}
