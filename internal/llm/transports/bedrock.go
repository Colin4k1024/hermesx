package transports

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	brdocument "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	brtypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/hermes-agent/hermes-agent-go/internal/llm"
)

// BedrockTransport implements llm.Transport for AWS Bedrock Converse API.
type BedrockTransport struct {
	client *bedrockruntime.Client
	model  string
	region string
}

// NewBedrockTransport creates a transport using the AWS SDK v2 credential chain.
func NewBedrockTransport(model, region string) (*BedrockTransport, error) {
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Strip provider prefix for Bedrock model IDs (e.g. "bedrock/anthropic.claude-3" -> "anthropic.claude-3")
	bedrockModel := model
	if strings.HasPrefix(bedrockModel, "bedrock/") {
		bedrockModel = strings.TrimPrefix(bedrockModel, "bedrock/")
	}

	return &BedrockTransport{
		client: bedrockruntime.NewFromConfig(cfg),
		model:  bedrockModel,
		region: region,
	}, nil
}

func (t *BedrockTransport) Name() string { return "bedrock" }

func (t *BedrockTransport) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	input, err := t.buildConverseInput(req)
	if err != nil {
		return nil, err
	}

	resp, err := t.client.Converse(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock converse failed: %w", err)
	}

	return t.parseConverseOutput(resp)
}

func (t *BedrockTransport) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamDelta, <-chan error) {
	deltaCh := make(chan llm.StreamDelta, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(deltaCh)
		defer close(errCh)

		input, err := t.buildConverseStreamInput(req)
		if err != nil {
			errCh <- err
			return
		}

		resp, err := t.client.ConverseStream(ctx, input)
		if err != nil {
			errCh <- fmt.Errorf("bedrock stream failed: %w", err)
			return
		}

		stream := resp.GetStream()
		defer stream.Close()

		var toolCalls []llm.ToolCall

		for event := range stream.Events() {
			switch e := event.(type) {
			case *brtypes.ConverseStreamOutputMemberContentBlockDelta:
				if textDelta, ok := e.Value.Delta.(*brtypes.ContentBlockDeltaMemberText); ok {
					deltaCh <- llm.StreamDelta{Content: textDelta.Value}
				}
				if toolDelta, ok := e.Value.Delta.(*brtypes.ContentBlockDeltaMemberToolUse); ok {
					if len(toolCalls) > 0 {
						last := &toolCalls[len(toolCalls)-1]
						last.Function.Arguments += derefString(toolDelta.Value.Input)
					}
				}
			case *brtypes.ConverseStreamOutputMemberContentBlockStart:
				if toolStart, ok := e.Value.Start.(*brtypes.ContentBlockStartMemberToolUse); ok {
					toolCalls = append(toolCalls, llm.ToolCall{
						ID:   derefString(toolStart.Value.ToolUseId),
						Type: "function",
						Function: llm.FunctionCall{
							Name: derefString(toolStart.Value.Name),
						},
					})
				}
			case *brtypes.ConverseStreamOutputMemberMessageStop:
				deltaCh <- llm.StreamDelta{Done: true, ToolCalls: toolCalls}
				return
			}
		}

		if err := stream.Err(); err != nil {
			errCh <- err
			return
		}

		deltaCh <- llm.StreamDelta{Done: true, ToolCalls: toolCalls}
	}()

	return deltaCh, errCh
}

func (t *BedrockTransport) buildConverseInput(req llm.ChatRequest) (*bedrockruntime.ConverseInput, error) {
	msgs, system := t.convertMessages(req.Messages)
	tools := t.convertTools(req.Tools)

	input := &bedrockruntime.ConverseInput{
		ModelId:  &t.model,
		Messages: msgs,
	}

	if len(system) > 0 {
		input.System = system
	}

	if len(tools) > 0 {
		input.ToolConfig = &brtypes.ToolConfiguration{
			Tools: tools,
		}
	}

	if req.MaxTokens > 0 {
		mt := int32(req.MaxTokens)
		input.InferenceConfig = &brtypes.InferenceConfiguration{
			MaxTokens: &mt,
		}
	}

	return input, nil
}

func (t *BedrockTransport) buildConverseStreamInput(req llm.ChatRequest) (*bedrockruntime.ConverseStreamInput, error) {
	msgs, system := t.convertMessages(req.Messages)
	tools := t.convertTools(req.Tools)

	input := &bedrockruntime.ConverseStreamInput{
		ModelId:  &t.model,
		Messages: msgs,
	}

	if len(system) > 0 {
		input.System = system
	}

	if len(tools) > 0 {
		input.ToolConfig = &brtypes.ToolConfiguration{
			Tools: tools,
		}
	}

	if req.MaxTokens > 0 {
		mt := int32(req.MaxTokens)
		input.InferenceConfig = &brtypes.InferenceConfiguration{
			MaxTokens: &mt,
		}
	}

	return input, nil
}

func (t *BedrockTransport) convertMessages(messages []llm.Message) ([]brtypes.Message, []brtypes.SystemContentBlock) {
	var brMsgs []brtypes.Message
	var system []brtypes.SystemContentBlock

	for _, m := range messages {
		if m.Role == "system" {
			system = append(system, &brtypes.SystemContentBlockMemberText{Value: m.Content})
			continue
		}

		role := brtypes.ConversationRoleUser
		if m.Role == "assistant" {
			role = brtypes.ConversationRoleAssistant
		}

		var content []brtypes.ContentBlock

		if m.Role == "tool" {
			// Tool results in Bedrock use toolResult content blocks
			resultDoc := jsonStrToSmithyDoc(m.Content)
			content = append(content, &brtypes.ContentBlockMemberToolResult{
				Value: brtypes.ToolResultBlock{
					ToolUseId: &m.ToolCallID,
					Content: []brtypes.ToolResultContentBlock{
						&brtypes.ToolResultContentBlockMemberJson{Value: resultDoc},
					},
				},
			})
			role = brtypes.ConversationRoleUser
		} else if m.Content != "" {
			content = append(content, &brtypes.ContentBlockMemberText{Value: m.Content})
		}

		// Convert tool calls to toolUse content blocks
		for _, tc := range m.ToolCalls {
			inputDoc := jsonStrToSmithyDoc(tc.Function.Arguments)
			content = append(content, &brtypes.ContentBlockMemberToolUse{
				Value: brtypes.ToolUseBlock{
					ToolUseId: &tc.ID,
					Name:      &tc.Function.Name,
					Input:     inputDoc,
				},
			})
		}

		if len(content) > 0 {
			brMsgs = append(brMsgs, brtypes.Message{
				Role:    role,
				Content: content,
			})
		}
	}

	return brMsgs, system
}

func (t *BedrockTransport) convertTools(tools []llm.ToolDef) []brtypes.Tool {
	var result []brtypes.Tool
	for _, td := range tools {
		name := td.Name
		desc := td.Description
		schemaDoc := toSmithyDoc(td.Parameters)

		result = append(result, &brtypes.ToolMemberToolSpec{
			Value: brtypes.ToolSpecification{
				Name:        &name,
				Description: &desc,
				InputSchema: &brtypes.ToolInputSchemaMemberJson{Value: schemaDoc},
			},
		})
	}
	return result
}

func (t *BedrockTransport) parseConverseOutput(resp *bedrockruntime.ConverseOutput) (*llm.ChatResponse, error) {
	result := &llm.ChatResponse{
		FinishReason: string(resp.StopReason),
	}

	if resp.Usage != nil {
		result.Usage = llm.Usage{
			PromptTokens:     int(derefInt32(resp.Usage.InputTokens)),
			CompletionTokens: int(derefInt32(resp.Usage.OutputTokens)),
			TotalTokens:      int(derefInt32(resp.Usage.InputTokens) + derefInt32(resp.Usage.OutputTokens)),
		}
	}

	// Map Bedrock stop reasons to standard format
	switch resp.StopReason {
	case brtypes.StopReasonEndTurn:
		result.FinishReason = "stop"
	case brtypes.StopReasonToolUse:
		result.FinishReason = "tool_calls"
	case brtypes.StopReasonMaxTokens:
		result.FinishReason = "length"
	}

	if resp.Output != nil {
		if msgOutput, ok := resp.Output.(*brtypes.ConverseOutputMemberMessage); ok {
			for _, block := range msgOutput.Value.Content {
				switch b := block.(type) {
				case *brtypes.ContentBlockMemberText:
					result.Content += b.Value
				case *brtypes.ContentBlockMemberToolUse:
					argsJSON, _ := json.Marshal(b.Value.Input)
					result.ToolCalls = append(result.ToolCalls, llm.ToolCall{
						ID:   derefString(b.Value.ToolUseId),
						Type: "function",
						Function: llm.FunctionCall{
							Name:      derefString(b.Value.Name),
							Arguments: string(argsJSON),
						},
					})
				}
			}
		}
	}

	return result, nil
}

// toSmithyDoc converts a Go value to a Bedrock document.Interface.
func toSmithyDoc(v any) brdocument.Interface {
	if v == nil {
		return brdocument.NewLazyDocument(map[string]any{})
	}
	return brdocument.NewLazyDocument(v)
}

// jsonStrToSmithyDoc converts a JSON string to a Bedrock document.Interface.
func jsonStrToSmithyDoc(s string) brdocument.Interface {
	if s == "" {
		s = "{}"
	}
	var raw any
	json.Unmarshal([]byte(s), &raw)
	return brdocument.NewLazyDocument(raw)
}

func derefInt32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

var _ llm.Transport = (*BedrockTransport)(nil)

func init() {
	slog.Debug("Bedrock transport registered")
}
