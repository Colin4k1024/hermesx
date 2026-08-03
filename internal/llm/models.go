package llm

import "sync"

// ModelMeta holds metadata for a known model.
type ModelMeta struct {
	ContextLength  int
	MaxOutput      int
	SupportsTools  bool
	SupportsVision bool
}

// knownModelsMu protects concurrent access to knownModels.
var knownModelsMu sync.RWMutex

// knownModels maps model identifiers to their metadata.
// Use GetModelMeta / SetModelMeta for thread-safe access.
var knownModels = map[string]ModelMeta{
	"anthropic/claude-opus-4-20250514":   {ContextLength: 200000, MaxOutput: 32000, SupportsTools: true, SupportsVision: true},
	"anthropic/claude-sonnet-4-20250514": {ContextLength: 200000, MaxOutput: 16000, SupportsTools: true, SupportsVision: true},
	"anthropic/claude-haiku-4-20250414":  {ContextLength: 200000, MaxOutput: 8192, SupportsTools: true, SupportsVision: true},
	"openai/gpt-4o":                      {ContextLength: 128000, MaxOutput: 16384, SupportsTools: true, SupportsVision: true},
	"openai/gpt-4o-mini":                 {ContextLength: 128000, MaxOutput: 16384, SupportsTools: true, SupportsVision: true},
	"openai/o1":                          {ContextLength: 200000, MaxOutput: 100000, SupportsTools: true, SupportsVision: true},
	"openai/o3":                          {ContextLength: 200000, MaxOutput: 100000, SupportsTools: true, SupportsVision: true},
	"google/gemini-2.5-pro":              {ContextLength: 1048576, MaxOutput: 65536, SupportsTools: true, SupportsVision: true},
	"google/gemini-2.5-flash":            {ContextLength: 1048576, MaxOutput: 65536, SupportsTools: true, SupportsVision: true},
	"deepseek/deepseek-chat":             {ContextLength: 65536, MaxOutput: 8192, SupportsTools: true, SupportsVision: false},
	"deepseek/deepseek-r1":               {ContextLength: 65536, MaxOutput: 8192, SupportsTools: true, SupportsVision: false},
	"meta-llama/llama-4-maverick":        {ContextLength: 1048576, MaxOutput: 32768, SupportsTools: true, SupportsVision: true},

	// Bedrock model IDs
	"bedrock/anthropic.claude-3-5-sonnet": {ContextLength: 200000, MaxOutput: 8192, SupportsTools: true, SupportsVision: true},
	"bedrock/anthropic.claude-3-haiku":    {ContextLength: 200000, MaxOutput: 4096, SupportsTools: true, SupportsVision: true},
	"bedrock/anthropic.claude-3-opus":     {ContextLength: 200000, MaxOutput: 4096, SupportsTools: true, SupportsVision: true},
	"bedrock/meta.llama3-70b-instruct":    {ContextLength: 8192, MaxOutput: 2048, SupportsTools: true, SupportsVision: false},
	"bedrock/amazon.titan-text-premier":   {ContextLength: 32000, MaxOutput: 8192, SupportsTools: false, SupportsVision: false},

	// Codex / Responses API
	"openai/codex-mini": {ContextLength: 200000, MaxOutput: 100000, SupportsTools: true, SupportsVision: false},
}

// LookupKnownModel returns metadata for a known model from the hardcoded table. Thread-safe.
func LookupKnownModel(model string) (ModelMeta, bool) {
	knownModelsMu.RLock()
	defer knownModelsMu.RUnlock()
	meta, ok := knownModels[model]
	return meta, ok
}

// SetModelMeta adds or updates a model's metadata (e.g. during catalog hot-reload). Thread-safe.
func SetModelMeta(model string, meta ModelMeta) {
	knownModelsMu.Lock()
	defer knownModelsMu.Unlock()
	knownModels[model] = meta
}

// AllKnownModels returns a snapshot of all known models. Thread-safe.
func AllKnownModels() map[string]ModelMeta {
	knownModelsMu.RLock()
	defer knownModelsMu.RUnlock()
	snapshot := make(map[string]ModelMeta, len(knownModels))
	for k, v := range knownModels {
		snapshot[k] = v
	}
	return snapshot
}

// EstimateTokens gives a rough token count for a string.
// Uses the ~4 chars per token heuristic.
func EstimateTokens(text string) int {
	return len(text) / 4
}
