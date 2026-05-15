package llm

import "strings"

// ModelAliases maps short human-friendly names to their fully qualified model identifiers.
// These allow users to specify "opus" instead of "anthropic/claude-opus-4-20250514".
var ModelAliases = map[string]string{
	// Anthropic
	"opus":   "anthropic/claude-opus-4-20250514",
	"sonnet": "anthropic/claude-sonnet-4-20250514",
	"haiku":  "anthropic/claude-haiku-4-20250414",

	// OpenAI
	"gpt4o":      "openai/gpt-4o",
	"gpt4o-mini": "openai/gpt-4o-mini",
	"o1":         "openai/o1",
	"o3":         "openai/o3",
	"codex":      "openai/codex-mini",

	// Google
	"gemini":       "google/gemini-2.5-pro",
	"gemini-pro":   "google/gemini-2.5-pro",
	"flash":        "google/gemini-2.5-flash",
	"gemini-flash": "google/gemini-2.5-flash",

	// DeepSeek
	"deepseek": "deepseek/deepseek-chat",
	"r1":       "deepseek/deepseek-r1",

	// Meta
	"llama":    "meta-llama/llama-4-maverick",
	"maverick": "meta-llama/llama-4-maverick",
}

// ResolveModelAlias resolves a short alias to its full model name.
// If the input is not a known alias, it is returned unchanged.
func ResolveModelAlias(name string) string {
	if resolved, ok := ModelAliases[strings.ToLower(strings.TrimSpace(name))]; ok {
		return resolved
	}
	return name
}

// IsModelAlias returns true if the given name is a recognized short alias.
func IsModelAlias(name string) bool {
	_, ok := ModelAliases[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// ListModelAliases returns all known aliases sorted for display.
func ListModelAliases() map[string]string {
	result := make(map[string]string, len(ModelAliases))
	for k, v := range ModelAliases {
		result[k] = v
	}
	return result
}
