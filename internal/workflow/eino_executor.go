package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	eino "github.com/Colin4k1024/hermesx/internal/eino"
	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/safety"
	"github.com/Colin4k1024/hermesx/internal/secrets"
	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

var _ AgentExecutor = (*EinoAgentExecutor)(nil)

// EinoAgentExecutor implements AgentExecutor using the Eino-based ReAct agent.
// Safety infrastructure is mandatory — construct via NewEinoAgentExecutor.
type EinoAgentExecutor struct {
	transport         llm.Transport
	tools             []*tools.ToolEntry
	safetyInterceptor safety.SafetyInterceptor
	leakScanner       *secrets.LeakScanner
}

// NewEinoAgentExecutor creates an executor backed by Eino's ReAct agent.
// Safety parameters are required; pass nil only for testing without safety.
func NewEinoAgentExecutor(transport llm.Transport, toolEntries []*tools.ToolEntry, interceptor safety.SafetyInterceptor, scanner *secrets.LeakScanner) *EinoAgentExecutor {
	return &EinoAgentExecutor{
		transport:         transport,
		tools:             toolEntries,
		safetyInterceptor: interceptor,
		leakScanner:       scanner,
	}
}

func (e *EinoAgentExecutor) Execute(ctx context.Context, tenantID, userID string, node store.WorkflowNode, payload map[string]any) (map[string]any, error) {
	prompt, _ := node.Config["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("agent task requires config.prompt")
	}

	serialized, _ := json.Marshal(payload)
	fullPrompt := prompt + "\n\nWorkflow context JSON:\n" + string(serialized)

	model, _ := node.Config["model"].(string)
	if model == "" {
		model = os.Getenv("LLM_MODEL")
	}

	maxIter := 20
	if mi, ok := node.Config["max_iterations"].(float64); ok && mi > 0 {
		maxIter = int(mi)
	}
	if maxIter > 50 {
		maxIter = 50
	}

	var toolEntries []*tools.ToolEntry
	if toolNames, ok := node.Config["tools"].([]any); ok && len(toolNames) > 0 {
		allowed := make(map[string]bool, len(toolNames))
		for _, tn := range toolNames {
			if name, ok := tn.(string); ok {
				allowed[name] = true
			}
		}
		for _, t := range e.tools {
			if allowed[t.Name] {
				toolEntries = append(toolEntries, t)
			}
		}
	} else {
		toolEntries = e.tools
	}

	transport := e.transport
	if transport == nil {
		return nil, fmt.Errorf("eino executor: transport is required")
	}

	agent, err := eino.NewEinoAgent(ctx,
		eino.WithTransport(transport),
		eino.WithModel(model),
		eino.WithTools(toolEntries),
		eino.WithMaxIterations(maxIter),
		eino.WithTenantID(tenantID),
		eino.WithUserID(userID),
		eino.WithSessionID(fmt.Sprintf("workflow-%s", node.ID)),
		eino.WithSafetyInterceptor(e.safetyInterceptor),
		eino.WithLeakScanner(e.leakScanner),
	)
	if err != nil {
		return nil, fmt.Errorf("eino executor: create agent: %w", err)
	}

	result, err := agent.RunConversationSafe(ctx, fullPrompt, nil)
	if err != nil {
		return nil, err
	}

	return map[string]any{"response": result.FinalResponse}, nil
}
