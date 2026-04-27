package llm

import "context"

// Transport abstracts LLM provider communication protocols.
type Transport interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamDelta, <-chan error)
	Name() string
}
