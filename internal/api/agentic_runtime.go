package api

import (
	"context"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/adk"

	"github.com/Colin4k1024/hermesx/internal/skills"
	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

const agenticMemoryGuidance = `

## Memory System
You have persistent memory that is stored securely per-user. Use the memory tool to manage it.

When the user asks about themselves, read memory first. When the user asks you to remember something, save it immediately.
`

const agenticSessionSearchGuidance = `

## Session Search
You can search past conversations using the session_search tool when the user references prior work.`

type systemPromptProvider interface {
	SystemPromptBlock() string
}

func (h *chatHandler) buildAgenticSystemPrompt(ctx context.Context, soulContent string, loader skills.SkillLoader, memProvider tools.MemoryProvider) string {
	var sb strings.Builder
	sb.WriteString(defaultSoul)
	sb.WriteString(agenticMemoryGuidance)
	sb.WriteString(agenticSessionSearchGuidance)

	if strings.TrimSpace(soulContent) != "" {
		sb.WriteString("\n\n## Persona\n")
		sb.WriteString(soulContent)
	}

	if sp, ok := memProvider.(systemPromptProvider); ok {
		if block := sp.SystemPromptBlock(); block != "" {
			sb.WriteString("\n\n")
			sb.WriteString(block)
		}
	}

	if loader != nil {
		loaded, err := loader.LoadAll(ctx)
		if err != nil {
			slog.Debug("agentic skills load failed", "error", err)
		} else if len(loaded) > 0 {
			sb.WriteString("\n\n## Available Skills\n")
			sb.WriteString(skills.BuildSkillsPrompt(loaded))
		}
	}

	return sb.String()
}

type agentCheckpointProvider interface {
	AgentCheckpoints() store.AgentCheckpointStore
}

func storeCheckpointAdapter(s store.Store) adk.CheckPointStore {
	provider, ok := s.(agentCheckpointProvider)
	if !ok || provider.AgentCheckpoints() == nil {
		return nil
	}
	return store.NewEinoCheckPointStore(provider.AgentCheckpoints())
}

func (h *chatHandler) sendMsgWithMeta(ctx context.Context, tenantID, sessionID string, role, content, reasoning, agenticBlocks string) int64 {
	id, err := h.store.Messages().Append(ctx, tenantID, sessionID, &store.Message{
		TenantID:      tenantID,
		SessionID:     sessionID,
		Role:          role,
		Content:       content,
		Reasoning:     reasoning,
		AgenticBlocks: agenticBlocks,
	})
	if err != nil {
		slog.Error("sendMsg_FAILED", "tenant", tenantID, "session", sessionID, "role", role, "error", err)
		return 0
	}
	slog.Info("sendMsg_OK", "tenant", tenantID, "session", sessionID, "role", role, "msg_id", id)
	return id
}
