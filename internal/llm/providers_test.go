package llm

import (
	"os"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/config"
)

func TestResolveProviderOpenRouter(t *testing.T) {
	os.Setenv("OPENROUTER_API_KEY", "test-key-123")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Unsetenv("OPENROUTER_API_KEY")

	cfg := &config.Config{}
	provider, baseURL, apiKey := ResolveProvider(cfg)
	if provider != "openrouter" {
		t.Errorf("Expected provider 'openrouter', got '%s'", provider)
	}
	if baseURL != OpenRouterBaseURL {
		t.Errorf("Expected OpenRouter base URL, got '%s'", baseURL)
	}
	if apiKey != "test-key-123" {
		t.Errorf("Expected api key 'test-key-123', got '%s'", apiKey)
	}
}

func TestResolveProviderExplicit(t *testing.T) {
	cfg := &config.Config{
		BaseURL: "https://custom.api.com/v1",
		APIKey:  "custom-key",
	}
	provider, baseURL, apiKey := ResolveProvider(cfg)
	if provider != "custom" {
		t.Errorf("Expected provider 'custom', got '%s'", provider)
	}
	if baseURL != "https://custom.api.com/v1" {
		t.Errorf("Expected custom base URL, got '%s'", baseURL)
	}
	if apiKey != "custom-key" {
		t.Errorf("Expected 'custom-key', got '%s'", apiKey)
	}
}

func TestIsOpenRouter(t *testing.T) {
	if !IsOpenRouter("https://openrouter.ai/api/v1") {
		t.Error("Expected true for OpenRouter URL")
	}
	if IsOpenRouter("https://api.openai.com/v1") {
		t.Error("Expected false for OpenAI URL")
	}
}

func TestIsAnthropic(t *testing.T) {
	if !IsAnthropic("anthropic") {
		t.Error("Expected true for anthropic provider")
	}
	if IsAnthropic("openai") {
		t.Error("Expected false for openai provider")
	}
}

func TestModelSupportsReasoning(t *testing.T) {
	reasoning := []string{
		"anthropic/claude-sonnet-4-20250514",
		"openai/o1",
		"deepseek/deepseek-r1",
	}
	for _, m := range reasoning {
		if !ModelSupportsReasoning(m) {
			t.Errorf("Expected %s to support reasoning", m)
		}
	}

	noReasoning := []string{
		"openai/gpt-4o",
		"meta-llama/llama-3-70b",
	}
	for _, m := range noReasoning {
		if ModelSupportsReasoning(m) {
			t.Errorf("Expected %s NOT to support reasoning", m)
		}
	}
}

func TestDetectAPIMode(t *testing.T) {
	if detectAPIMode("anthropic", "", "") != APIModeAnthropic {
		t.Error("Explicit 'anthropic' should return APIModeAnthropic")
	}
	if detectAPIMode("", "anthropic", "") != APIModeAnthropic {
		t.Error("Provider 'anthropic' should return APIModeAnthropic")
	}
	if detectAPIMode("", "", "https://api.anthropic.com/v1") != APIModeAnthropic {
		t.Error("Anthropic URL should return APIModeAnthropic")
	}
	if detectAPIMode("", "", "https://openrouter.ai/api/v1") != APIModeOpenAI {
		t.Error("OpenRouter URL should return APIModeOpenAI")
	}
	if detectAPIMode("", "", "") != APIModeOpenAI {
		t.Error("Default should be APIModeOpenAI")
	}
}

func TestResolveProviderAnthropicFromEnv(t *testing.T) {
	os.Unsetenv("OPENROUTER_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	cfg := &config.Config{}
	provider, baseURL, apiKey := ResolveProvider(cfg)
	if provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got '%s'", provider)
	}
	if baseURL != AnthropicBaseURL {
		t.Errorf("Expected Anthropic base URL, got '%s'", baseURL)
	}
	if apiKey != "test-anthropic-key" {
		t.Errorf("Expected api key 'test-anthropic-key', got '%s'", apiKey)
	}
}

func TestResolveProviderOpenAIFromEnv(t *testing.T) {
	os.Unsetenv("OPENROUTER_API_KEY")
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Unsetenv("OPENAI_API_KEY")

	cfg := &config.Config{}
	provider, baseURL, apiKey := ResolveProvider(cfg)
	if provider != "openai" {
		t.Errorf("Expected provider 'openai', got '%s'", provider)
	}
	if baseURL != OpenAIBaseURL {
		t.Errorf("Expected OpenAI base URL, got '%s'", baseURL)
	}
	if apiKey != "test-openai-key" {
		t.Errorf("Expected api key 'test-openai-key', got '%s'", apiKey)
	}
}

func TestResolveProviderFromModelPrefix(t *testing.T) {
	os.Unsetenv("OPENROUTER_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "test-ant-key")
	os.Unsetenv("OPENAI_API_KEY")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	cfg := &config.Config{Model: "anthropic/claude-sonnet-4-20250514"}
	provider, _, apiKey := ResolveProvider(cfg)
	if provider != "anthropic" {
		t.Errorf("Expected provider 'anthropic' from model prefix, got '%s'", provider)
	}
	if apiKey != "test-ant-key" {
		t.Errorf("Expected 'test-ant-key', got '%s'", apiKey)
	}
}

func TestResolveProviderNoKeys(t *testing.T) {
	os.Unsetenv("OPENROUTER_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")

	cfg := &config.Config{}
	provider, baseURL, apiKey := ResolveProvider(cfg)
	if provider != "" || baseURL != "" || apiKey != "" {
		t.Errorf("Expected empty results when no keys, got provider=%q baseURL=%q", provider, baseURL)
	}
}

func TestResolveProviderConfigKey(t *testing.T) {
	os.Unsetenv("OPENROUTER_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")

	cfg := &config.Config{
		APIKey: "config-key-123",
	}
	provider, baseURL, apiKey := ResolveProvider(cfg)
	if apiKey != "config-key-123" {
		t.Errorf("Expected 'config-key-123', got '%s'", apiKey)
	}
	if baseURL == "" {
		t.Error("Expected non-empty baseURL")
	}
	if provider == "" {
		t.Error("Expected non-empty provider")
	}
}

func TestModelSupportsReasoning_More(t *testing.T) {
	if !ModelSupportsReasoning("openai/o3") {
		t.Error("Expected o3 to support reasoning")
	}
	if !ModelSupportsReasoning("deepseek/deepseek-r1") {
		t.Error("Expected deepseek-r1 to support reasoning")
	}
	if !ModelSupportsReasoning("qwq-32b") {
		t.Error("Expected qwq to support reasoning")
	}
	if ModelSupportsReasoning("openai/gpt-4o-mini") {
		t.Error("Expected gpt-4o-mini NOT to support reasoning")
	}
	if ModelSupportsReasoning("google/gemini-2.5-flash") {
		t.Error("Expected gemini-flash NOT to support reasoning")
	}
}

func TestBaseURLConstants(t *testing.T) {
	if OpenRouterBaseURL == "" {
		t.Error("Expected non-empty OpenRouter base URL")
	}
	if OpenAIBaseURL == "" {
		t.Error("Expected non-empty OpenAI base URL")
	}
	if AnthropicBaseURL == "" {
		t.Error("Expected non-empty Anthropic base URL")
	}
}
