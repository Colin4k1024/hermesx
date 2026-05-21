package modeladapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

// WrappedModel bridges HermesX llm.Transport to Eino's ToolCallingChatModel.
type WrappedModel struct {
	transport llm.Transport
	modelName string
	tools     []*schema.ToolInfo
}

var _ model.ToolCallingChatModel = (*WrappedModel)(nil)

// Wrap creates a new WrappedModel from a Transport and model name.
func Wrap(transport llm.Transport, modelName string) *WrappedModel {
	return &WrappedModel{
		transport: transport,
		modelName: modelName,
	}
}

func (m *WrappedModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	options := model.GetCommonOptions(&model.Options{}, opts...)

	req := m.buildRequest(input, options)
	req.Stream = false

	resp, err := m.transport.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("model generate: %w", err)
	}

	return convertResponse(resp), nil
}

func (m *WrappedModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	options := model.GetCommonOptions(&model.Options{}, opts...)

	req := m.buildRequest(input, options)
	req.Stream = true

	deltaCh, errCh := m.transport.ChatStream(ctx, req)

	reader, writer := schema.Pipe[*schema.Message](8)
	go func() {
		defer writer.Close()
		for {
			select {
			case delta, ok := <-deltaCh:
				if !ok {
					return
				}
				if delta.Done {
					if len(delta.ToolCalls) > 0 || delta.Reasoning != "" || delta.Content != "" {
						if writer.Send(convertDelta(&delta), nil) {
							return
						}
					}
					return
				}
				msg := convertDelta(&delta)
				if writer.Send(msg, nil) {
					return
				}
			case err, ok := <-errCh:
				if !ok {
					return
				}
				if err != nil {
					writer.Send(nil, err)
					return
				}
			case <-ctx.Done():
				writer.Send(nil, ctx.Err())
				return
			}
		}
	}()

	return reader, nil
}

func (m *WrappedModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	clone := &WrappedModel{
		transport: m.transport,
		modelName: m.modelName,
		tools:     tools,
	}
	return clone, nil
}

func (m *WrappedModel) buildRequest(input []*schema.Message, opts *model.Options) llm.ChatRequest {
	req := llm.ChatRequest{
		Messages: convertMessagesToLLM(input),
	}

	if opts.MaxTokens != nil {
		req.MaxTokens = *opts.MaxTokens
	}
	if opts.Temperature != nil {
		req.Temperature = opts.Temperature
	}

	tools := m.tools
	if opts.Tools != nil {
		tools = opts.Tools
	}
	if len(tools) > 0 {
		req.Tools = convertToolInfos(tools)
	}

	return req
}

func convertMessagesToLLM(msgs []*schema.Message) []llm.Message {
	out := make([]llm.Message, 0, len(msgs))
	for _, msg := range msgs {
		m := llm.Message{
			Role:             string(msg.Role),
			Content:          msg.Content,
			ToolCallID:       msg.ToolCallID,
			ToolName:         msg.ToolName,
			ReasoningContent: msg.ReasoningContent,
		}
		if len(msg.ToolCalls) > 0 {
			m.ToolCalls = make([]llm.ToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				m.ToolCalls[i] = llm.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: llm.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
		out = append(out, m)
	}
	return out
}

func convertToolInfos(tools []*schema.ToolInfo) []llm.ToolDef {
	defs := make([]llm.ToolDef, 0, len(tools))
	for _, t := range tools {
		def := llm.ToolDef{
			Name:        t.Name,
			Description: t.Desc,
		}
		if t.ParamsOneOf != nil {
			js, err := t.ParamsOneOf.ToJSONSchema()
			if err == nil && js != nil {
				data, _ := json.Marshal(js)
				var params map[string]any
				_ = json.Unmarshal(data, &params)
				def.Parameters = params
			}
		}
		defs = append(defs, def)
	}
	return defs
}

func convertResponse(resp *llm.ChatResponse) *schema.Message {
	msg := &schema.Message{
		Role:             schema.Assistant,
		Content:          resp.Content,
		ReasoningContent: resp.Reasoning,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: resp.FinishReason,
			Usage: &schema.TokenUsage{
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
				TotalTokens:      resp.Usage.TotalTokens,
				PromptTokenDetails: schema.PromptTokenDetails{
					CachedTokens: resp.Usage.CacheReadTokens,
				},
				CompletionTokensDetails: schema.CompletionTokensDetails{
					ReasoningTokens: resp.Usage.ReasoningTokens,
				},
			},
		},
	}
	if len(resp.ToolCalls) > 0 {
		msg.ToolCalls = make([]schema.ToolCall, len(resp.ToolCalls))
		for i, tc := range resp.ToolCalls {
			msg.ToolCalls[i] = schema.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: schema.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}
	return msg
}

func convertDelta(delta *llm.StreamDelta) *schema.Message {
	msg := &schema.Message{
		Role:             schema.Assistant,
		Content:          delta.Content,
		ReasoningContent: delta.Reasoning,
	}
	if len(delta.ToolCalls) > 0 {
		msg.ToolCalls = make([]schema.ToolCall, len(delta.ToolCalls))
		for i, tc := range delta.ToolCalls {
			idx := i
			msg.ToolCalls[i] = schema.ToolCall{
				Index: &idx,
				ID:    tc.ID,
				Type:  tc.Type,
				Function: schema.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}
	return msg
}
