package agentruntime

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/Colin4k1024/hermesx/internal/agent"
	"github.com/Colin4k1024/hermesx/internal/config"
	"github.com/Colin4k1024/hermesx/internal/eino"
	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/secrets"
	"github.com/Colin4k1024/hermesx/internal/skills"
	"github.com/Colin4k1024/hermesx/internal/tools"
	"github.com/Colin4k1024/hermesx/internal/toolsets"
)

// Runtime is the agent surface shared by legacy and Eino-backed entrypoints.
type Runtime interface {
	RunConversation(ctx context.Context, userMessage string, history []llm.Message) (*agent.ConversationResult, error)
	Chat(ctx context.Context, message string) (string, error)
	SessionID() string
	Model() string
	Close()
}

// SkillRuntime adds slash-skill helpers used by gateway command routing.
type SkillRuntime interface {
	Runtime
	IsSkill(input string) bool
	InjectSkill(skillName string) (string, error)
}

// Options configures an Eino-backed runtime.
type Options struct {
	Model            string
	BaseURL          string
	APIKey           string
	Provider         string
	APIMode          string
	MaxIterations    int
	Platform         string
	SessionID        string
	SystemPrompt     string
	SkillLoader      skills.SkillLoader
	SkipContextFiles bool
	SkipMemory       bool
	EnabledToolsets  []string
	DisabledToolsets []string
	TenantID         string
	UserID           string
	MemoryProvider   tools.MemoryProvider
	SoulContent      string
	SecretResolver   secrets.SecretResolver
	HTTPTransport    *http.Transport
	Transport        llm.Transport
	Callbacks        *agent.StreamCallbacks
}

// EinoRuntime adapts the Eino agent to the shared runtime interface.
type EinoRuntime struct {
	agent       *eino.EinoAgent
	sessionID   string
	model       string
	skillLoader skills.SkillLoader
}

// NewEino constructs an Eino-backed runtime using Hermes configuration defaults.
func NewEino(ctx context.Context, opts Options) (*EinoRuntime, error) {
	cfg := config.Load()
	if opts.Platform == "" {
		opts.Platform = "api"
	}
	if opts.SessionID == "" {
		opts.SessionID = uuid.New().String()
	}
	if opts.MaxIterations <= 0 {
		opts.MaxIterations = cfg.MaxIterations
	}
	model := firstNonEmpty(opts.Model, cfg.Model)
	baseURL := firstNonEmpty(opts.BaseURL, cfg.BaseURL)
	apiKey := firstNonEmpty(opts.APIKey, cfg.APIKey)
	provider := firstNonEmpty(opts.Provider, cfg.Provider)
	apiMode := firstNonEmpty(opts.APIMode, cfg.APIMode)

	llmClient, err := buildLLMClient(opts.Transport, model, baseURL, apiKey, provider, apiMode)
	if err != nil {
		return nil, err
	}

	systemPrompt := agent.BuildSystemPrompt(agent.SystemPromptOptions{
		Platform:              opts.Platform,
		EphemeralSystemPrompt: opts.SystemPrompt,
		SkillLoader:           opts.SkillLoader,
		SkipContextFiles:      opts.SkipContextFiles,
		SkipMemory:            opts.SkipMemory,
		SoulContent:           opts.SoulContent,
		MemoryProvider:        opts.MemoryProvider,
	})

	einoOpts := []eino.Option{
		eino.WithTransport(llmClient.GetTransport()),
		eino.WithModel(llmClient.Model()),
		eino.WithProvider(llmClient.Provider()),
		eino.WithBaseURL(llmClient.BaseURL()),
		eino.WithAPIKey(apiKey),
		eino.WithAPIMode(string(llmClient.APIMode())),
		eino.WithTenantID(opts.TenantID),
		eino.WithUserID(opts.UserID),
		eino.WithSessionID(opts.SessionID),
		eino.WithPlatform(opts.Platform),
		eino.WithSystemPrompt(systemPrompt),
		eino.WithTools(resolveToolEntries(opts.EnabledToolsets, opts.DisabledToolsets, true)),
		eino.WithMemoryProvider(opts.MemoryProvider),
		eino.WithMaxIterations(opts.MaxIterations),
		eino.WithSecretResolver(opts.SecretResolver),
	}
	if opts.HTTPTransport != nil {
		einoOpts = append(einoOpts, eino.WithHTTPTransport(opts.HTTPTransport))
	}

	einoAgent, err := eino.NewEinoAgent(ctx, einoOpts...)
	if err != nil {
		return nil, err
	}
	if opts.Callbacks != nil {
		einoAgent.SetCallbacks(AdaptCallbacks(opts.Callbacks))
	}

	return &EinoRuntime{
		agent:       einoAgent,
		sessionID:   opts.SessionID,
		model:       llmClient.Model(),
		skillLoader: opts.SkillLoader,
	}, nil
}

func buildLLMClient(transport llm.Transport, model, baseURL, apiKey, provider, apiMode string) (*llm.Client, error) {
	if transport != nil {
		return llm.NewClientWithTransport(model, baseURL, apiKey, provider, transport), nil
	}
	if apiMode != "" {
		return llm.NewClientWithMode(model, baseURL, apiKey, provider, llm.APIMode(apiMode))
	}
	return llm.NewClientWithParams(model, baseURL, apiKey, provider)
}

// RunConversation executes one turn through Eino and returns the legacy result shape.
func (r *EinoRuntime) RunConversation(ctx context.Context, userMessage string, history []llm.Message) (*agent.ConversationResult, error) {
	result, err := r.agent.RunConversationSafe(ctx, userMessage, history)
	if err != nil {
		return nil, err
	}
	return toLegacyResult(result), nil
}

// Chat runs a one-turn chat and returns the final response.
func (r *EinoRuntime) Chat(ctx context.Context, message string) (string, error) {
	result, err := r.RunConversation(ctx, message, nil)
	if err != nil {
		return "", err
	}
	return result.FinalResponse, nil
}

func (r *EinoRuntime) SessionID() string { return r.sessionID }

func (r *EinoRuntime) Model() string { return r.model }

func (r *EinoRuntime) Close() {}

func (r *EinoRuntime) IsSkill(input string) bool {
	return isSkill(input, r.skillLoader)
}

func (r *EinoRuntime) InjectSkill(skillName string) (string, error) {
	return injectSkill(skillName, r.skillLoader)
}

// LegacyRuntime adapts AIAgent to the shared runtime interface for staged CLI migration.
type LegacyRuntime struct {
	agent *agent.AIAgent
}

func NewLegacy(opts ...agent.AgentOption) (*LegacyRuntime, error) {
	ag, err := agent.New(opts...)
	if err != nil {
		return nil, err
	}
	return &LegacyRuntime{agent: ag}, nil
}

func (r *LegacyRuntime) RunConversation(ctx context.Context, userMessage string, history []llm.Message) (*agent.ConversationResult, error) {
	return r.agent.RunConversationWithContext(ctx, userMessage, history)
}

func (r *LegacyRuntime) Chat(ctx context.Context, message string) (string, error) {
	result, err := r.RunConversation(ctx, message, nil)
	if err != nil {
		return "", err
	}
	return result.FinalResponse, nil
}

func (r *LegacyRuntime) SessionID() string { return r.agent.SessionID() }

func (r *LegacyRuntime) Model() string { return r.agent.Model() }

func (r *LegacyRuntime) Close() { r.agent.Close() }

func (r *LegacyRuntime) IsSkill(input string) bool { return r.agent.IsSkill(input) }

func (r *LegacyRuntime) InjectSkill(skillName string) (string, error) {
	return r.agent.InjectSkill(skillName)
}

func (r *LegacyRuntime) Agent() *agent.AIAgent { return r.agent }

// AdaptCallbacks maps legacy callbacks to the Eino callback surface.
func AdaptCallbacks(cb *agent.StreamCallbacks) *eino.StreamCallbacks {
	if cb == nil {
		return nil
	}
	return &eino.StreamCallbacks{
		OnStreamDelta:  cb.OnStreamDelta,
		OnReasoning:    cb.OnReasoning,
		OnToolStart:    cb.OnToolStart,
		OnToolComplete: cb.OnToolComplete,
		OnStep:         cb.OnStep,
		OnStatus:       cb.OnStatus,
		OnError:        cb.OnError,
	}
}

func toLegacyResult(result *eino.ConversationResult) *agent.ConversationResult {
	if result == nil {
		return nil
	}
	return &agent.ConversationResult{
		FinalResponse:    result.FinalResponse,
		LastReasoning:    result.LastReasoning,
		Messages:         result.Messages,
		APICalls:         result.APICalls,
		Completed:        result.Completed,
		Interrupted:      result.Interrupted,
		Model:            result.Model,
		Provider:         result.Provider,
		BaseURL:          result.BaseURL,
		InputTokens:      result.InputTokens,
		OutputTokens:     result.OutputTokens,
		CacheReadTokens:  result.CacheReadTokens,
		CacheWriteTokens: result.CacheWriteTokens,
		ReasoningTokens:  result.ReasoningTokens,
		TotalTokens:      result.TotalTokens,
	}
}

func resolveToolEntries(enabled, disabled []string, quiet bool) []*tools.ToolEntry {
	toolNames := resolveTools(enabled, disabled)
	entries := make([]*tools.ToolEntry, 0, len(toolNames))
	for name := range toolNames {
		entry := tools.Registry().Lookup(name)
		if entry == nil {
			continue
		}
		if !toolAvailable(entry, quiet) {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

func resolveTools(enabled, disabled []string) map[string]bool {
	var toolList []string
	if len(enabled) > 0 {
		toolList = toolsets.ResolveMultipleToolsets(enabled)
	} else {
		toolList = toolsets.ResolveToolset("hermesx-cli")
	}

	result := make(map[string]bool, len(toolList))
	for _, toolName := range toolList {
		result[toolName] = true
	}
	if len(disabled) > 0 {
		for _, toolName := range toolsets.ResolveMultipleToolsets(disabled) {
			delete(result, toolName)
		}
	}
	return result
}

func toolAvailable(entry *tools.ToolEntry, quiet bool) (ok bool) {
	if entry == nil || entry.CheckFn == nil {
		return true
	}
	defer func() {
		if rec := recover(); rec != nil {
			if !quiet {
				slog.Debug("Tool unavailable (check panic)", "tool", entry.Name, "panic", rec)
			}
			ok = false
		}
	}()
	ok = entry.CheckFn()
	if !ok && !quiet {
		slog.Debug("Tool unavailable (check failed)", "tool", entry.Name)
	}
	return ok
}

func isSkill(input string, loader skills.SkillLoader) bool {
	if loader == nil {
		return agent.IsSkillCommand(input)
	}
	name, ok := skillNameFromInput(input)
	if !ok {
		return false
	}
	_, err := loader.Find(context.Background(), name)
	return err == nil
}

func injectSkill(skillName string, loader skills.SkillLoader) (string, error) {
	if loader == nil {
		return agent.InjectSkillAsUserMessage(skillName)
	}
	name, ok := skillNameFromInput(skillName)
	if !ok {
		name = skillName
	}
	entry, err := loader.Find(context.Background(), name)
	if err != nil {
		return "", fmt.Errorf("skill %q not found: %w", name, err)
	}
	return formatSkillAsUserMessage(entry), nil
}

func skillNameFromInput(input string) (string, bool) {
	if !strings.HasPrefix(input, "/") {
		return "", false
	}
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", false
	}
	name := strings.TrimPrefix(parts[0], "/")
	return name, name != ""
}

func formatSkillAsUserMessage(entry *skills.SkillEntry) string {
	if entry == nil || entry.Meta == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Skill activated: %s", entry.Meta.Name))
	if entry.Meta.Description != "" {
		sb.WriteString(fmt.Sprintf(" - %s", entry.Meta.Description))
	}
	sb.WriteString("]\n\n")
	sb.WriteString(entry.Body)
	return sb.String()
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
