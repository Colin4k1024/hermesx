package eino

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestBlocksJSON_Empty(t *testing.T) {
	result := BlocksJSON(nil)
	if result != "" {
		t.Errorf("BlocksJSON(nil) = %q, want empty", result)
	}
}

func TestBlocksJSON_NonEmpty(t *testing.T) {
	blocks := []AgenticBlock{
		{Type: "text", Data: map[string]any{"content": "hello"}},
	}
	result := BlocksJSON(blocks)
	if result == "" {
		t.Error("BlocksJSON with blocks should return non-empty JSON")
	}
}

func TestCapture_JSON_Empty(t *testing.T) {
	c := NewCapture()
	result := c.JSON()
	if result != "" {
		t.Errorf("JSON() on empty capture = %q, want empty", result)
	}
}

func TestCapture_JSON_WithBlocks(t *testing.T) {
	c := NewCapture()
	c.AddBlocks([]AgenticBlock{
		{Type: "reasoning", Data: map[string]any{"text": "thinking..."}},
	})
	result := c.JSON()
	if result == "" {
		t.Error("JSON() should return non-empty string when blocks exist")
	}
}

func TestCapture_Reset(t *testing.T) {
	c := NewCapture()
	c.AddBlocks([]AgenticBlock{{Type: "text"}})
	c.Reset()
	if result := c.JSON(); result != "" {
		t.Errorf("after Reset, JSON() = %q, want empty", result)
	}
}

func TestAgenticBlocksFromMessage_Nil(t *testing.T) {
	blocks := AgenticBlocksFromMessage(nil)
	if len(blocks) != 0 {
		t.Errorf("AgenticBlocksFromMessage(nil) = %v, want empty", blocks)
	}
}

func TestAgenticBlocksFromAgenticMessage_Nil(t *testing.T) {
	blocks := AgenticBlocksFromAgenticMessage(nil)
	if len(blocks) != 0 {
		t.Errorf("AgenticBlocksFromAgenticMessage(nil) should return nil, got %v", blocks)
	}
}

func TestAgenticBlocksFromAgenticMessage_Empty(t *testing.T) {
	blocks := AgenticBlocksFromAgenticMessage(&schema.AgenticMessage{})
	if len(blocks) != 0 {
		t.Errorf("expected empty blocks for message with no content blocks, got %v", blocks)
	}
}

func TestMessageToAgentic_Nil(t *testing.T) {
	result := MessageToAgentic(nil)
	if result != nil {
		t.Error("MessageToAgentic(nil) should return nil")
	}
}

func TestMessageToAgentic_Text(t *testing.T) {
	msg := &schema.Message{Role: schema.Assistant, Content: "hello"}
	agentic := MessageToAgentic(msg)
	if agentic == nil {
		t.Fatal("expected non-nil AgenticMessage")
	}
	if agentic.Role != schema.AgenticRoleTypeAssistant {
		t.Errorf("role = %v, want assistant", agentic.Role)
	}
}

func TestAgenticToMessage_Nil(t *testing.T) {
	result := AgenticToMessage(nil)
	if result != nil {
		t.Error("AgenticToMessage(nil) should return nil")
	}
}

func TestAgenticToMessage_Simple(t *testing.T) {
	msg := &schema.AgenticMessage{
		Role: schema.AgenticRoleTypeAssistant,
	}
	result := AgenticToMessage(msg)
	if result == nil {
		t.Fatal("expected non-nil Message")
	}
	if string(result.Role) != string(schema.AgenticRoleTypeAssistant) {
		t.Errorf("role = %q, want %q", result.Role, schema.AgenticRoleTypeAssistant)
	}
}

func TestSanitizeContentBlock(t *testing.T) {
	block := &schema.ContentBlock{}
	result := sanitizeContentBlock(block)
	if result.Type == "" && result.Data == nil {
		// Empty block - just verify no panic
	}
	_ = result
}

func TestContentBlockFromBlock_Reasoning(t *testing.T) {
	block := AgenticBlock{
		Type: string(schema.ContentBlockTypeReasoning),
		Data: map[string]any{"text": "thinking..."},
	}
	result := contentBlockFromBlock(block)
	if result == nil {
		t.Fatal("expected non-nil ContentBlock")
	}
}

func TestContentBlockFromBlock_UserInputText(t *testing.T) {
	block := AgenticBlock{
		Type: string(schema.ContentBlockTypeUserInputText),
		Data: map[string]any{"text": "hello"},
	}
	result := contentBlockFromBlock(block)
	if result == nil {
		t.Fatal("expected non-nil ContentBlock for user input")
	}
}

func TestContentBlockFromBlock_AssistantGenText(t *testing.T) {
	block := AgenticBlock{
		Type: string(schema.ContentBlockTypeAssistantGenText),
		Data: map[string]any{"text": "response"},
	}
	result := contentBlockFromBlock(block)
	if result == nil {
		t.Fatal("expected non-nil ContentBlock for assistant text")
	}
}

func TestContentBlockFromBlock_Default(t *testing.T) {
	block := AgenticBlock{
		Type: "unknown-type",
		Data: map[string]any{"content": "fallback"},
	}
	result := contentBlockFromBlock(block)
	if result == nil {
		t.Fatal("expected non-nil ContentBlock for default case")
	}
}

func TestSanitizeMap_RemovesSecretFields(t *testing.T) {
	m := map[string]any{
		"api_key":   "secret-value",
		"safe_key":  "safe-value",
		"signature": "sig-value",
	}
	sanitizeMap(m)
	if _, ok := m["api_key"]; ok {
		t.Error("api_key should be removed by sanitizeMap")
	}
	if _, ok := m["signature"]; ok {
		t.Error("signature should be removed by sanitizeMap")
	}
	if _, ok := m["safe_key"]; !ok {
		t.Error("safe_key should be preserved")
	}
}
