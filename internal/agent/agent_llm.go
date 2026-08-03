package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Colin4k1024/hermesx/internal/config"
	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/tools"
	"github.com/Colin4k1024/hermesx/internal/toolsets"
)

func (a *AIAgent) buildAPIMessages(messages []llm.Message) []llm.Message {
	apiMessages := make([]llm.Message, 0, len(messages)+1)

	// System prompt
	apiMessages = append(apiMessages, llm.Message{
		Role:    "system",
		Content: a.systemPrompt,
	})

	// Conversation messages
	apiMessages = append(apiMessages, messages...)

	return apiMessages
}

func (a *AIAgent) buildToolDefs(cfg *config.Config) {
	// Resolve which tools to enable
	toolNames := resolveTools(a.enabledToolsets, a.disabledToolsets)
	a.validTools = toolNames

	// Get OpenAI-format definitions
	defs := tools.Registry().GetDefinitions(toolNames, a.quietMode)

	a.toolDefs = make([]llm.ToolDef, 0, len(defs))
	for _, d := range defs {
		fnDef, ok := d["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := fnDef["name"].(string)
		desc, _ := fnDef["description"].(string)
		var params map[string]any
		if p, ok := fnDef["parameters"]; ok {
			if pm, ok := p.(map[string]any); ok {
				params = pm
			} else {
				b, _ := json.Marshal(p)
				json.Unmarshal(b, &params)
			}
		}
		a.toolDefs = append(a.toolDefs, llm.ToolDef{
			Name:        name,
			Description: desc,
			Parameters:  params,
		})
	}
}

func (a *AIAgent) streamingAPICall(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	deltaCh, errCh := a.client.CreateChatCompletionStream(ctx, req)
	return a.consumeStream(deltaCh, errCh)
}

// consumeStream drains a streaming delta channel and collects the response.
func (a *AIAgent) consumeStream(deltaCh <-chan llm.StreamDelta, errCh <-chan error) (*llm.ChatResponse, error) {
	resp := &llm.ChatResponse{}
	var contentBuilder []byte

	for delta := range deltaCh {
		if delta.Done {
			resp.ToolCalls = delta.ToolCalls
			break
		}

		if delta.Content != "" {
			contentBuilder = append(contentBuilder, delta.Content...)
			a.streamHandler.FireStreamDelta(delta.Content)
		}

		if delta.Reasoning != "" {
			a.streamHandler.FireReasoning(delta.Reasoning)
			resp.Reasoning += delta.Reasoning
		}
	}

	// Block until the stream wrapper closes errCh (always happens after deltaCh drains).
	if err := <-errCh; err != nil {
		return nil, err
	}

	resp.Content = string(contentBuilder)

	if len(resp.ToolCalls) > 0 {
		resp.FinishReason = "tool_calls"
	} else {
		resp.FinishReason = "stop"
	}

	return resp, nil
}

func resolveTools(enabled, disabled []string) map[string]bool {
	var toolList []string

	if len(enabled) > 0 {
		toolList = toolsets.ResolveMultipleToolsets(enabled)
	} else {
		// Default: use hermes-cli toolset (which equals CoreTools)
		toolList = toolsets.ResolveToolset("hermesx-cli")
	}

	result := make(map[string]bool, len(toolList))
	for _, t := range toolList {
		result[t] = true
	}

	// Remove disabled toolset tools
	if len(disabled) > 0 {
		disabledTools := toolsets.ResolveMultipleToolsets(disabled)
		for _, t := range disabledTools {
			delete(result, t)
		}
	}

	return result
}

// ResumeSession loads history from a previous session and resumes it.
func (a *AIAgent) ResumeSession(sessionID string) error {
	a.resumeSessionID = sessionID
	return a.loadResumedSession()
}

// loadResumedSession loads messages from the session DB for a resumed session.
func (a *AIAgent) loadResumedSession() error {
	if a.sessionDB == nil {
		return fmt.Errorf("session DB not available")
	}
	if a.resumeSessionID == "" {
		return nil
	}

	// Verify the session exists
	sess, err := a.sessionDB.GetSession(a.resumeSessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}
	if sess == nil {
		return fmt.Errorf("session %s not found", a.resumeSessionID)
	}

	// Use the resumed session's ID going forward
	a.sessionID = a.resumeSessionID

	slog.Info("Resumed session", "session_id", a.sessionID)
	return nil
}

// tryFallbackModels attempts each fallback model in order after the primary fails.
func (a *AIAgent) tryFallbackModels(ctx context.Context, req llm.ChatRequest, primaryErr error) (*llm.ChatResponse, error) {
	if len(a.fallbackModels) == 0 {
		return nil, primaryErr
	}

	for _, fb := range a.fallbackModels {
		slog.Warn("Primary model failed, trying fallback",
			"primary_error", primaryErr,
			"fallback_model", fb.Model)

		apiKey := fb.APIKey
		if apiKey == "" {
			apiKey = a.apiKey
		}
		baseURL := fb.BaseURL
		if baseURL == "" {
			baseURL = a.baseURL
		}

		fbClient, err := llm.NewClientWithParams(fb.Model, baseURL, apiKey, fb.Provider)
		if err != nil {
			slog.Warn("Failed to create fallback client", "model", fb.Model, "error", err)
			continue
		}

		var resp *llm.ChatResponse
		var fbErr error

		if req.Stream && a.streamHandler.HasStreamConsumers() {
			deltaCh, errCh := fbClient.CreateChatCompletionStream(ctx, req)
			resp, fbErr = a.consumeStream(deltaCh, errCh)
		} else {
			resp, fbErr = fbClient.CreateChatCompletion(ctx, req)
		}

		if fbErr != nil {
			slog.Warn("Fallback model also failed", "model", fb.Model, "error", fbErr)
			primaryErr = fbErr
			continue
		}

		slog.Info("Fallback model succeeded", "model", fb.Model)
		return resp, nil
	}

	return nil, primaryErr
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
