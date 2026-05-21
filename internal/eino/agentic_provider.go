package eino

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/agenticclaude"
	"github.com/cloudwego/eino-ext/components/model/agenticgemini"
	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
	"github.com/cloudwego/eino/components/model"
	"google.golang.org/genai"
)

// AgenticProviderConfig configures provider-native Eino v0.9 AgenticModel paths.
type AgenticProviderConfig struct {
	Provider   string
	APIMode    string
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
	MaxTokens  int
}

// NewAgenticProviderModel constructs an Eino Ext AgenticModel for providers
// that expose native block-oriented APIs.
func NewAgenticProviderModel(ctx context.Context, cfg AgenticProviderConfig) (model.AgenticModel, error) {
	provider := strings.ToLower(cfg.Provider)
	mode := strings.ToLower(cfg.APIMode)
	switch {
	case provider == "anthropic" || mode == "anthropic":
		maxTokens := cfg.MaxTokens
		if maxTokens <= 0 {
			maxTokens = 4096
		}
		return agenticclaude.New(ctx, &agenticclaude.Config{
			BaseURL:    cfg.BaseURL,
			APIKey:     cfg.APIKey,
			Model:      cfg.Model,
			MaxTokens:  maxTokens,
			HTTPClient: cfg.HTTPClient,
		})
	case provider == "gemini" || mode == "gemini":
		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:     cfg.APIKey,
			HTTPClient: cfg.HTTPClient,
		})
		if err != nil {
			return nil, err
		}
		return agenticgemini.NewAgenticModel(ctx, &agenticgemini.Config{
			Client: client,
			Model:  cfg.Model,
		})
	case provider == "openai" || mode == "openai" || mode == "responses" || mode == "codex" || provider == "":
		return agenticopenai.New(ctx, &agenticopenai.Config{
			BaseURL:    cfg.BaseURL,
			APIKey:     cfg.APIKey,
			Model:      cfg.Model,
			HTTPClient: cfg.HTTPClient,
		})
	default:
		return nil, fmt.Errorf("unsupported agentic provider %q", cfg.Provider)
	}
}
