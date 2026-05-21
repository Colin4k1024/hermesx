package eino

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"

	"github.com/Colin4k1024/hermesx/internal/eino/modeladapter"
	"github.com/Colin4k1024/hermesx/internal/llm"
)

// AgenticBlock is the API/storage-safe representation of an Eino v0.9 content block.
type AgenticBlock struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

// Capture records agentic blocks observed while a model call or adapter runs.
type Capture struct {
	mu     sync.Mutex
	blocks []AgenticBlock
}

func NewCapture() *Capture {
	return &Capture{}
}

func (c *Capture) Reset() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.blocks = c.blocks[:0]
}

func (c *Capture) AddBlocks(blocks []AgenticBlock) {
	if c == nil || len(blocks) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.blocks = append(c.blocks, blocks...)
}

func (c *Capture) AddAgenticBlocks(blocks []modeladapter.AgenticBlock) {
	if c == nil || len(blocks) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	converted := make([]AgenticBlock, 0, len(blocks))
	for _, block := range blocks {
		converted = append(converted, AgenticBlock{
			Type: block.Type,
			Data: block.Data,
		})
	}
	c.blocks = append(c.blocks, converted...)
}

func (c *Capture) Blocks() []AgenticBlock {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.blocks) == 0 {
		return nil
	}
	out := make([]AgenticBlock, len(c.blocks))
	copy(out, c.blocks)
	return out
}

func (c *Capture) JSON() string {
	data, err := json.Marshal(c.Blocks())
	if err != nil || len(data) == 0 || string(data) == "null" {
		return ""
	}
	return string(data)
}

func BlocksJSON(blocks []AgenticBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	data, err := json.Marshal(blocks)
	if err != nil {
		return ""
	}
	return string(data)
}

func AgenticBlocksFromMessage(msg *schema.Message) []AgenticBlock {
	if msg == nil {
		return nil
	}
	var blocks []AgenticBlock
	if msg.ReasoningContent != "" {
		blocks = append(blocks, AgenticBlock{
			Type: string(schema.ContentBlockTypeReasoning),
			Data: map[string]any{"text": msg.ReasoningContent},
		})
	}
	if msg.Content != "" {
		switch msg.Role {
		case schema.User:
			blocks = append(blocks, AgenticBlock{
				Type: string(schema.ContentBlockTypeUserInputText),
				Data: map[string]any{"text": msg.Content},
			})
		case schema.Tool:
			blocks = append(blocks, AgenticBlock{
				Type: string(schema.ContentBlockTypeFunctionToolResult),
				Data: map[string]any{
					"call_id": msg.ToolCallID,
					"name":    msg.ToolName,
					"content": msg.Content,
				},
			})
		default:
			blocks = append(blocks, AgenticBlock{
				Type: string(schema.ContentBlockTypeAssistantGenText),
				Data: map[string]any{"text": msg.Content},
			})
		}
	}
	for _, tc := range msg.ToolCalls {
		blocks = append(blocks, AgenticBlock{
			Type: string(schema.ContentBlockTypeFunctionToolCall),
			Data: map[string]any{
				"call_id":   tc.ID,
				"name":      tc.Function.Name,
				"arguments": tc.Function.Arguments,
			},
		})
	}
	return blocks
}

func AgenticBlocksFromAgenticMessage(msg *schema.AgenticMessage) []AgenticBlock {
	if msg == nil || len(msg.ContentBlocks) == 0 {
		return nil
	}
	blocks := make([]AgenticBlock, 0, len(msg.ContentBlocks))
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		blocks = append(blocks, sanitizeContentBlock(block))
	}
	return blocks
}

func MessageToAgentic(msg *schema.Message) *schema.AgenticMessage {
	if msg == nil {
		return nil
	}
	role := schema.AgenticRoleType(msg.Role)
	if role == "" {
		role = schema.AgenticRoleTypeAssistant
	}
	am := &schema.AgenticMessage{Role: role}
	switch msg.Role {
	case schema.System:
		am.Role = schema.AgenticRoleTypeSystem
	case schema.User:
		am.Role = schema.AgenticRoleTypeUser
	case schema.Assistant, schema.Tool:
		am.Role = schema.AgenticRoleTypeAssistant
	}
	for _, block := range AgenticBlocksFromMessage(msg) {
		am.ContentBlocks = append(am.ContentBlocks, contentBlockFromBlock(block))
	}
	if len(am.ContentBlocks) == 0 && msg.Content != "" {
		am.ContentBlocks = append(am.ContentBlocks, schema.NewContentBlock(&schema.UserInputText{Text: msg.Content}))
	}
	return am
}

func AgenticToMessage(msg *schema.AgenticMessage) *schema.Message {
	if msg == nil {
		return nil
	}
	out := &schema.Message{Role: schema.RoleType(msg.Role)}
	if out.Role == "" {
		out.Role = schema.Assistant
	}
	if msg.ResponseMeta != nil {
		out.ResponseMeta = &schema.ResponseMeta{Usage: msg.ResponseMeta.TokenUsage}
	}
	var text strings.Builder
	var reasoning strings.Builder
	for _, block := range msg.ContentBlocks {
		if block == nil {
			continue
		}
		switch block.Type {
		case schema.ContentBlockTypeAssistantGenText:
			if block.AssistantGenText != nil {
				text.WriteString(block.AssistantGenText.Text)
			}
		case schema.ContentBlockTypeUserInputText:
			if block.UserInputText != nil {
				text.WriteString(block.UserInputText.Text)
			}
		case schema.ContentBlockTypeReasoning:
			if block.Reasoning != nil {
				reasoning.WriteString(block.Reasoning.Text)
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
		case schema.ContentBlockTypeFunctionToolResult:
			if block.FunctionToolResult != nil {
				out.Role = schema.Tool
				out.ToolCallID = block.FunctionToolResult.CallID
				out.ToolName = block.FunctionToolResult.Name
				for _, content := range block.FunctionToolResult.Content {
					if content != nil && content.Text != nil {
						text.WriteString(content.Text.Text)
					}
				}
			}
		}
	}
	out.Content = text.String()
	out.ReasoningContent = reasoning.String()
	return out
}

func SchemaMessagesToLLM(messages []*schema.Message) []llm.Message {
	out := make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		m := llm.Message{
			Role:             string(msg.Role),
			Content:          msg.Content,
			ToolCallID:       msg.ToolCallID,
			ToolName:         msg.ToolName,
			ReasoningContent: msg.ReasoningContent,
			Reasoning:        msg.ReasoningContent,
		}
		if msg.ResponseMeta != nil {
			m.FinishReason = msg.ResponseMeta.FinishReason
		}
		for _, tc := range msg.ToolCalls {
			m.ToolCalls = append(m.ToolCalls, llm.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: llm.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
		out = append(out, m)
	}
	return out
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
			strings.Contains(lower, "secret") {
			delete(m, k)
			continue
		}
		switch vv := v.(type) {
		case map[string]any:
			sanitizeMap(vv)
		case []any:
			for _, item := range vv {
				if mm, ok := item.(map[string]any); ok {
					sanitizeMap(mm)
				}
			}
		}
	}
}

func contentBlockFromBlock(block AgenticBlock) *schema.ContentBlock {
	switch schema.ContentBlockType(block.Type) {
	case schema.ContentBlockTypeReasoning:
		text, _ := block.Data["text"].(string)
		return schema.NewContentBlock(&schema.Reasoning{Text: text})
	case schema.ContentBlockTypeUserInputText:
		text, _ := block.Data["text"].(string)
		return schema.NewContentBlock(&schema.UserInputText{Text: text})
	case schema.ContentBlockTypeAssistantGenText:
		text, _ := block.Data["text"].(string)
		return schema.NewContentBlock(&schema.AssistantGenText{Text: text})
	case schema.ContentBlockTypeFunctionToolCall:
		callID, _ := block.Data["call_id"].(string)
		name, _ := block.Data["name"].(string)
		args, _ := block.Data["arguments"].(string)
		return schema.NewContentBlock(&schema.FunctionToolCall{CallID: callID, Name: name, Arguments: args})
	default:
		text, _ := block.Data["text"].(string)
		if text == "" {
			text, _ = block.Data["content"].(string)
		}
		return schema.NewContentBlock(&schema.AssistantGenText{Text: text})
	}
}
