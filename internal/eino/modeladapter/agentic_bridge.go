package modeladapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// AgenticCapture is implemented by callers that want the sanitized native
// AgenticMessage blocks emitted by provider-native models.
type AgenticCapture interface {
	AddAgenticBlocks([]AgenticBlock)
}

// AgenticBlock mirrors internal/eino.AgenticBlock without creating an import cycle.
type AgenticBlock struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

// AgenticBridge adapts provider-native AgenticModel implementations into the
// schema.Message control plane used by Eino ADK ChatModelAgent.
type AgenticBridge struct {
	model   model.AgenticModel
	capture AgenticCapture
}

var _ model.BaseChatModel = (*AgenticBridge)(nil)

func WrapAgentic(agentic model.AgenticModel, capture AgenticCapture) *AgenticBridge {
	return &AgenticBridge{model: agentic, capture: capture}
}

func (m *AgenticBridge) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	agenticInput := make([]*schema.AgenticMessage, 0, len(input))
	for _, msg := range input {
		agenticInput = append(agenticInput, messageToAgentic(msg))
	}
	resp, err := m.model.Generate(ctx, agenticInput, opts...)
	if err != nil {
		return nil, err
	}
	m.captureAgentic(resp)
	return agenticToMessage(resp), nil
}

func (m *AgenticBridge) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	agenticInput := make([]*schema.AgenticMessage, 0, len(input))
	for _, msg := range input {
		agenticInput = append(agenticInput, messageToAgentic(msg))
	}
	reader, err := m.model.Stream(ctx, agenticInput, opts...)
	if err != nil {
		return nil, err
	}
	out, writer := schema.Pipe[*schema.Message](8)
	go func() {
		defer writer.Close()
		defer reader.Close()
		for {
			msg, recvErr := reader.Recv()
			if recvErr != nil {
				if errors.Is(recvErr, io.EOF) {
					return
				}
				writer.Send(nil, recvErr)
				return
			}
			m.captureAgentic(msg)
			if writer.Send(agenticToMessage(msg), nil) {
				return
			}
		}
	}()
	return out, nil
}

func (m *AgenticBridge) captureAgentic(msg *schema.AgenticMessage) {
	if m.capture == nil || msg == nil {
		return
	}
	blocks := make([]AgenticBlock, 0, len(msg.ContentBlocks))
	for _, block := range msg.ContentBlocks {
		if block == nil || !captureNativeOnly(block.Type) {
			continue
		}
		blocks = append(blocks, sanitizeContentBlock(block))
	}
	if len(blocks) == 0 {
		return
	}
	m.capture.AddAgenticBlocks(blocks)
}

func captureNativeOnly(blockType schema.ContentBlockType) bool {
	switch blockType {
	case schema.ContentBlockTypeReasoning,
		schema.ContentBlockTypeAssistantGenText,
		schema.ContentBlockTypeUserInputText,
		schema.ContentBlockTypeFunctionToolCall,
		schema.ContentBlockTypeFunctionToolResult:
		return false
	default:
		return true
	}
}

func sanitizeContentBlock(block *schema.ContentBlock) AgenticBlock {
	var raw map[string]any
	data, err := json.Marshal(block)
	if err == nil {
		_ = json.Unmarshal(data, &raw)
	}
	if raw == nil {
		raw = map[string]any{"type": string(block.Type)}
	}
	sanitizeMap(raw)
	delete(raw, "type")
	return AgenticBlock{Type: string(block.Type), Data: raw}
}

func sanitizeMap(m map[string]any) {
	for k, v := range m {
		lower := strings.ToLower(k)
		if strings.Contains(lower, "signature") ||
			strings.Contains(lower, "api_key") ||
			strings.Contains(lower, "apikey") ||
			strings.Contains(lower, "authorization") ||
			strings.Contains(lower, "auth") ||
			strings.Contains(lower, "secret") ||
			strings.Contains(lower, "token") {
			delete(m, k)
			continue
		}
		switch typed := v.(type) {
		case map[string]any:
			sanitizeMap(typed)
		case []any:
			for _, item := range typed {
				if nested, ok := item.(map[string]any); ok {
					sanitizeMap(nested)
				}
			}
		}
	}
}

func messageToAgentic(msg *schema.Message) *schema.AgenticMessage {
	if msg == nil {
		return nil
	}
	switch msg.Role {
	case schema.System:
		return schema.SystemAgenticMessage(msg.Content)
	case schema.User:
		return schema.UserAgenticMessage(msg.Content)
	default:
		am := &schema.AgenticMessage{Role: schema.AgenticRoleTypeAssistant}
		if msg.ReasoningContent != "" {
			am.ContentBlocks = append(am.ContentBlocks, schema.NewContentBlock(&schema.Reasoning{Text: msg.ReasoningContent}))
		}
		if msg.Content != "" {
			am.ContentBlocks = append(am.ContentBlocks, schema.NewContentBlock(&schema.AssistantGenText{Text: msg.Content}))
		}
		for _, tc := range msg.ToolCalls {
			am.ContentBlocks = append(am.ContentBlocks, schema.NewContentBlock(&schema.FunctionToolCall{
				CallID:    tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}))
		}
		return am
	}
}

func agenticToMessage(msg *schema.AgenticMessage) *schema.Message {
	if msg == nil {
		return nil
	}
	out := &schema.Message{Role: schema.RoleType(msg.Role)}
	if out.Role == "" {
		out.Role = schema.Assistant
	}
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		switch block.Type {
		case schema.ContentBlockTypeReasoning:
			if block.Reasoning != nil {
				out.ReasoningContent += block.Reasoning.Text
			}
		case schema.ContentBlockTypeAssistantGenText, schema.ContentBlockTypeUserInputText:
			if block.AssistantGenText != nil {
				out.Content += block.AssistantGenText.Text
			}
			if block.UserInputText != nil {
				out.Content += block.UserInputText.Text
			}
		case schema.ContentBlockTypeFunctionToolCall:
			if block.FunctionToolCall != nil {
				out.ToolCalls = append(out.ToolCalls, schema.ToolCall{
					ID:   block.FunctionToolCall.CallID,
					Type: "function",
					Function: schema.FunctionCall{
						Name:      block.FunctionToolCall.Name,
						Arguments: block.FunctionToolCall.Arguments,
					},
				})
			}
		}
	}
	if msg.ResponseMeta != nil {
		out.ResponseMeta = &schema.ResponseMeta{Usage: msg.ResponseMeta.TokenUsage}
	}
	return out
}
