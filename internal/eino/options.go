package eino

import (
	"github.com/cloudwego/eino/adk"

	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/safety"
	"github.com/Colin4k1024/hermesx/internal/secrets"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

// Option configures an EinoAgent.
type Option func(*agentConfig)

// WithTransport sets the LLM transport.
func WithTransport(t llm.Transport) Option {
	return func(c *agentConfig) { c.transport = t }
}

// WithModel sets the model name.
func WithModel(model string) Option {
	return func(c *agentConfig) { c.modelName = model }
}

// WithProvider sets the provider name used for observability and failover defaults.
func WithProvider(provider string) Option {
	return func(c *agentConfig) { c.provider = provider }
}

// WithBaseURL sets the provider base URL used for failover defaults.
func WithBaseURL(baseURL string) Option {
	return func(c *agentConfig) { c.baseURL = baseURL }
}

// WithAPIKey sets the API key used for failover defaults.
func WithAPIKey(apiKey string) Option {
	return func(c *agentConfig) { c.apiKey = apiKey }
}

// WithAPIMode sets the API mode used for failover defaults.
func WithAPIMode(mode string) Option {
	return func(c *agentConfig) { c.apiMode = mode }
}

// WithFallbackModels configures ADK model failover.
func WithFallbackModels(models []FallbackModel) Option {
	return func(c *agentConfig) { c.fallbackModels = models }
}

// WithTools sets the tool entries.
func WithTools(entries []*tools.ToolEntry) Option {
	return func(c *agentConfig) { c.toolEntries = entries }
}

// WithMaxIterations sets the maximum ReAct loop iterations.
func WithMaxIterations(n int) Option {
	return func(c *agentConfig) {
		if n > 0 {
			c.maxIterations = n
		}
	}
}

// WithSystemPrompt sets the system prompt injected before user messages.
func WithSystemPrompt(prompt string) Option {
	return func(c *agentConfig) { c.systemPrompt = prompt }
}

// WithTenantID sets the tenant ID for multi-tenant isolation.
func WithTenantID(id string) Option {
	return func(c *agentConfig) { c.tenantID = id }
}

// WithUserID sets the user ID.
func WithUserID(id string) Option {
	return func(c *agentConfig) { c.userID = id }
}

// WithSessionID sets the session ID.
func WithSessionID(id string) Option {
	return func(c *agentConfig) { c.sessionID = id }
}

// WithPlatform sets the platform identifier exposed to tools.
func WithPlatform(platform string) Option {
	return func(c *agentConfig) { c.platform = platform }
}

// WithMemoryProvider injects a per-user memory provider into tool calls.
func WithMemoryProvider(provider tools.MemoryProvider) Option {
	return func(c *agentConfig) { c.memoryProvider = provider }
}

// WithCheckpointStore enables ADK cancel/interruption checkpoint persistence.
func WithCheckpointStore(store adk.CheckPointStore) Option {
	return func(c *agentConfig) { c.checkpointStore = store }
}

// WithSafetyInterceptor sets the safety interceptor for input/output validation.
func WithSafetyInterceptor(interceptor safety.SafetyInterceptor) Option {
	return func(c *agentConfig) { c.safetyInterceptor = interceptor }
}

// WithLeakScanner sets the secret leak scanner for tool output redaction.
func WithLeakScanner(scanner *secrets.LeakScanner) Option {
	return func(c *agentConfig) { c.leakScanner = scanner }
}
