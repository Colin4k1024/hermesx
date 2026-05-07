package transports

import (
	"context"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

// AnthropicTransport implements llm.Transport using the native Anthropic Messages API.
// It delegates to llm.AnthropicClient which handles the full protocol (SSE parsing,
// prompt caching, message alternation enforcement, etc.).
type AnthropicTransport struct {
	client *llm.AnthropicClient
}

// NewAnthropicTransport creates a transport for the Anthropic Messages API.
func NewAnthropicTransport(model, baseURL, apiKey, provider string) *AnthropicTransport {
	return &AnthropicTransport{
		client: llm.NewAnthropicClient(model, baseURL, apiKey, provider),
	}
}

func (t *AnthropicTransport) Name() string { return "anthropic" }

func (t *AnthropicTransport) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return t.client.CreateChatCompletion(ctx, req)
}

func (t *AnthropicTransport) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamDelta, <-chan error) {
	return t.client.CreateChatCompletionStream(ctx, req)
}

var _ llm.Transport = (*AnthropicTransport)(nil)
