package agent

import (
	"testing"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

func TestMessagesContainImages_NoImages(t *testing.T) {
	msgs := []llm.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	if messagesContainImages(msgs) {
		t.Error("expected false for messages without images")
	}
}

func TestMessagesContainImages_WithImages(t *testing.T) {
	msgs := []llm.Message{
		{Role: "user", Content: "what is this?", ImageURLs: []string{"https://example.com/img.png"}},
	}
	if !messagesContainImages(msgs) {
		t.Error("expected true for messages with images")
	}
}

func TestMessagesContainImages_EmptySlice(t *testing.T) {
	msgs := []llm.Message{
		{Role: "user", Content: "look", ImageURLs: []string{}},
	}
	if messagesContainImages(msgs) {
		t.Error("expected false for empty ImageURLs slice")
	}
}

func TestMultimodalRouter_Disabled(t *testing.T) {
	r := &MultimodalRouter{Enabled: false}
	msgs := []llm.Message{
		{Role: "user", Content: "what is this?", ImageURLs: []string{"https://example.com/img.png"}},
	}
	if r.NeedsVisionRouting(msgs, "openai/gpt-3.5-turbo") {
		t.Error("disabled router should never route")
	}
}

func TestMultimodalRouter_NoImages(t *testing.T) {
	r := &MultimodalRouter{Enabled: true}
	msgs := []llm.Message{
		{Role: "user", Content: "hello"},
	}
	if r.NeedsVisionRouting(msgs, "openai/gpt-3.5-turbo") {
		t.Error("should not route when no images present")
	}
}

func TestMultimodalRouter_ModelSupportsVision(t *testing.T) {
	r := &MultimodalRouter{Enabled: true}
	msgs := []llm.Message{
		{Role: "user", Content: "what is this?", ImageURLs: []string{"https://example.com/img.png"}},
	}
	// gpt-4o supports vision in KnownModels
	if r.NeedsVisionRouting(msgs, "openai/gpt-4o") {
		t.Error("should not route when model supports vision")
	}
}

func TestMultimodalRouter_ModelLacksVision(t *testing.T) {
	r := &MultimodalRouter{Enabled: true}
	msgs := []llm.Message{
		{Role: "user", Content: "what is this?", ImageURLs: []string{"https://example.com/img.png"}},
	}
	// Use a model that doesn't support vision — unknown models default to SupportsVision=false
	if !r.NeedsVisionRouting(msgs, "unknownprovider/text-only-model") {
		t.Error("should route when model lacks vision and images present")
	}
}

func TestVisionClientForRequest_NoRouter(t *testing.T) {
	a := &AIAgent{
		multimodalRouter: nil,
		model:            "openai/gpt-3.5-turbo",
	}
	msgs := []llm.Message{
		{Role: "user", Content: "img", ImageURLs: []string{"https://example.com/img.png"}},
	}
	if a.visionClientForRequest(msgs) != nil {
		t.Error("should return nil when router is nil")
	}
}

func TestVisionClientForRequest_NoAuxiliary(t *testing.T) {
	a := &AIAgent{
		multimodalRouter: DefaultMultimodalRouter(),
		auxiliaryClient:  nil,
		model:            "unknownprovider/text-only-model",
	}
	msgs := []llm.Message{
		{Role: "user", Content: "img", ImageURLs: []string{"https://example.com/img.png"}},
	}
	if a.visionClientForRequest(msgs) != nil {
		t.Error("should return nil when auxiliary client is nil")
	}
}

func TestVisionClientForRequest_NoVisionClient(t *testing.T) {
	a := &AIAgent{
		multimodalRouter: DefaultMultimodalRouter(),
		auxiliaryClient:  &AuxiliaryClient{}, // no vision client set
		model:            "unknownprovider/text-only-model",
	}
	msgs := []llm.Message{
		{Role: "user", Content: "img", ImageURLs: []string{"https://example.com/img.png"}},
	}
	if a.visionClientForRequest(msgs) != nil {
		t.Error("should return nil when vision client not configured")
	}
}

func TestDefaultMultimodalRouter(t *testing.T) {
	r := DefaultMultimodalRouter()
	if !r.Enabled {
		t.Error("default router should be enabled")
	}
}
