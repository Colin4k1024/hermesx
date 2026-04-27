package transports

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/hermes-agent/hermes-agent-go/internal/llm"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAITransport implements llm.Transport for OpenAI-compatible APIs.
type OpenAITransport struct {
	client  *openai.Client
	model   string
	baseURL string
}

// NewOpenAITransport creates a transport for any OpenAI-compatible endpoint.
func NewOpenAITransport(model, baseURL, apiKey string) *OpenAITransport {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	return &OpenAITransport{
		client:  openai.NewClientWithConfig(cfg),
		model:   model,
		baseURL: baseURL,
	}
}

func (t *OpenAITransport) Name() string { return "openai" }

func (t *OpenAITransport) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	apiReq := t.buildRequest(req)
	apiReq.Stream = false

	resp, err := t.client.CreateChatCompletion(ctx, apiReq)
	if err != nil {
		return nil, fmt.Errorf("API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return &llm.ChatResponse{FinishReason: "stop"}, nil
	}

	choice := resp.Choices[0]
	result := &llm.ChatResponse{
		Content:      choice.Message.Content,
		FinishReason: string(choice.FinishReason),
		Usage: llm.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	for _, tc := range choice.Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, llm.ToolCall{
			ID:   tc.ID,
			Type: string(tc.Type),
			Function: llm.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	return result, nil
}

func (t *OpenAITransport) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamDelta, <-chan error) {
	deltaCh := make(chan llm.StreamDelta, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(deltaCh)
		defer close(errCh)

		apiReq := t.buildRequest(req)
		apiReq.Stream = true

		stream, err := t.client.CreateChatCompletionStream(ctx, apiReq)
		if err != nil {
			errCh <- fmt.Errorf("stream creation failed: %w", err)
			return
		}
		defer stream.Close()

		toolCalls := make(map[int]*llm.ToolCall)

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				var finalCalls []llm.ToolCall
				for _, tc := range toolCalls {
					finalCalls = append(finalCalls, *tc)
				}
				deltaCh <- llm.StreamDelta{Done: true, ToolCalls: finalCalls}
				return
			}
			if err != nil {
				errCh <- err
				return
			}

			if len(resp.Choices) == 0 {
				continue
			}

			delta := resp.Choices[0].Delta

			if delta.Content != "" {
				deltaCh <- llm.StreamDelta{Content: delta.Content}
			}

			for _, tc := range delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				existing, ok := toolCalls[idx]
				if !ok {
					existing = &llm.ToolCall{
						ID:   tc.ID,
						Type: string(tc.Type),
						Function: llm.FunctionCall{
							Name: tc.Function.Name,
						},
					}
					toolCalls[idx] = existing
				}
				if tc.Function.Name != "" {
					existing.Function.Name = tc.Function.Name
				}
				existing.Function.Arguments += tc.Function.Arguments
			}
		}
	}()

	return deltaCh, errCh
}

func (t *OpenAITransport) buildRequest(req llm.ChatRequest) openai.ChatCompletionRequest {
	var msgs []openai.ChatCompletionMessage
	for _, m := range req.Messages {
		msg := openai.ChatCompletionMessage{
			Role: m.Role,
		}

		if len(m.ImageURLs) > 0 {
			var parts []openai.ChatMessagePart
			if m.Content != "" {
				parts = append(parts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: m.Content,
				})
			}
			for _, imgURL := range m.ImageURLs {
				parts = append(parts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL:    imgURL,
						Detail: openai.ImageURLDetailAuto,
					},
				})
			}
			msg.MultiContent = parts
		} else {
			msg.Content = m.Content
		}

		if m.ToolCallID != "" {
			msg.ToolCallID = m.ToolCallID
		}

		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
				ID:   tc.ID,
				Type: openai.ToolType(tc.Type),
				Function: openai.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}

		msgs = append(msgs, msg)
	}

	var oaiTools []openai.Tool
	for _, td := range req.Tools {
		oaiTools = append(oaiTools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        td.Name,
				Description: td.Description,
				Parameters:  td.ToRawParameters(),
			},
		})
	}

	apiReq := openai.ChatCompletionRequest{
		Model:    t.model,
		Messages: msgs,
		Tools:    oaiTools,
	}

	if req.MaxTokens > 0 {
		apiReq.MaxTokens = req.MaxTokens
	}
	if req.Temperature != nil {
		apiReq.Temperature = *req.Temperature
	}

	return apiReq
}

var _ llm.Transport = (*OpenAITransport)(nil)

func init() {
	slog.Debug("OpenAI transport registered")
}

// ConvertToolDefsToJSON converts ToolDefs to the map format used by the tools registry.
func ConvertToolDefsToJSON(tools []llm.ToolDef) []map[string]any {
	var result []map[string]any
	for _, td := range tools {
		b, _ := json.Marshal(td)
		var m map[string]any
		json.Unmarshal(b, &m)
		result = append(result, map[string]any{
			"type":     "function",
			"function": m,
		})
	}
	return result
}
