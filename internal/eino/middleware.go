package eino

import (
	"context"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

// RunConversationWithCallbacks runs through the ADK streaming runner so HermesX
// callbacks receive model/tool lifecycle events.
func (e *EinoAgent) RunConversationWithCallbacks(ctx context.Context, userMessage string, history []llm.Message) (*ConversationResult, error) {
	return e.RunConversation(ctx, userMessage, history)
}
