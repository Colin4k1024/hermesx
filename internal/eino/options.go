package eino

import (
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

// WithSafetyInterceptor sets the safety interceptor for input/output validation.
func WithSafetyInterceptor(interceptor safety.SafetyInterceptor) Option {
	return func(c *agentConfig) { c.safetyInterceptor = interceptor }
}

// WithLeakScanner sets the secret leak scanner for tool output redaction.
func WithLeakScanner(scanner *secrets.LeakScanner) Option {
	return func(c *agentConfig) { c.leakScanner = scanner }
}
