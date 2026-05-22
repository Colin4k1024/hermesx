package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/safety"
	"github.com/Colin4k1024/hermesx/internal/secrets"
	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/Colin4k1024/hermesx/internal/tools"
	"github.com/Colin4k1024/hermesx/internal/toolsets"
)

// AgentExecutor keeps agent-task execution swappable for tests and future
// provider-specific routing while preserving deterministic workflow control.
type AgentExecutor interface {
	Execute(ctx context.Context, tenantID, userID string, node store.WorkflowNode, payload map[string]any) (map[string]any, error)
}

// HTTPExecutor keeps service-task execution swappable for tests.
type HTTPExecutor interface {
	Do(req *http.Request) (*http.Response, error)
}

// Engine is the fixed SOP workflow runtime.
type Engine struct {
	store         store.WorkflowStore
	httpClient    HTTPExecutor
	agentExecutor AgentExecutor
}

func NewEngine(s store.WorkflowStore, httpClient HTTPExecutor, agentExecutor AgentExecutor) *Engine {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if agentExecutor == nil {
		agentExecutor = defaultAgentExecutor{}
	}
	return &Engine{store: s, httpClient: httpClient, agentExecutor: agentExecutor}
}

type defaultAgentExecutor struct{}

func (defaultAgentExecutor) Execute(ctx context.Context, tenantID, userID string, node store.WorkflowNode, payload map[string]any) (map[string]any, error) {
	model, _ := node.Config["model"].(string)
	if model == "" {
		model = os.Getenv("LLM_MODEL")
	}
	apiMode := os.Getenv("HERMES_API_MODE")
	baseURL := os.Getenv("LLM_API_URL")
	apiKey := os.Getenv("LLM_API_KEY")

	var client *llm.Client
	var err error
	if apiMode != "" {
		client, err = llm.NewClientWithMode(model, baseURL, apiKey, "", llm.APIMode(apiMode))
	} else {
		client, err = llm.NewClientWithParams(model, baseURL, apiKey, "")
	}
	if err != nil {
		return nil, fmt.Errorf("create workflow eino client: %w", err)
	}

	executor := newEinoAgentExecutorFromClient(client, defaultWorkflowToolEntries(), safety.NewInterceptorChain(safety.NewInMemoryPolicyStore()), secrets.NewLeakScanner())
	executor.apiKey = apiKey
	return executor.Execute(ctx, tenantID, userID, node, payload)
}

func defaultWorkflowToolEntries() []*tools.ToolEntry {
	names := toolsets.ResolveToolset("hermesx-cli")
	entries := make([]*tools.ToolEntry, 0, len(names))
	for _, name := range names {
		entry := tools.Registry().Lookup(name)
		if entry == nil {
			continue
		}
		if entry.CheckFn != nil && !entry.CheckFn() {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// StartRun starts a workflow instance on the latest published version.
func (e *Engine) StartRun(ctx context.Context, tenantID, definitionID, startedBy string, input map[string]any) (*store.WorkflowRun, []*store.WorkflowStepRun, error) {
	version, err := e.store.GetLatestVersion(ctx, tenantID, definitionID)
	if err != nil {
		return nil, nil, fmt.Errorf("workflow definition has no published version: %w", err)
	}
	graph, err := ParseGraph(version.GraphJSON)
	if err != nil {
		return nil, nil, err
	}
	inputJSON := mustJSON(input)
	run := &store.WorkflowRun{
		TenantID:      tenantID,
		DefinitionID:  definitionID,
		VersionID:     version.ID,
		Status:        store.WorkflowRunRunning,
		StartedBy:     startedBy,
		InputJSON:     inputJSON,
		VariablesJSON: "{}",
	}
	steps := make([]*store.WorkflowStepRun, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		status := store.WorkflowStepPending
		if node.Type == store.WorkflowNodeStart {
			status = store.WorkflowStepReady
		}
		steps = append(steps, &store.WorkflowStepRun{
			NodeID:         node.ID,
			NodeType:       node.Type,
			Status:         status,
			AssigneeUserID: node.AssigneeUserID,
			AssigneeRole:   node.AssigneeRole,
			InputJSON:      "{}",
			OutputJSON:     "{}",
		})
	}
	if err := e.store.CreateRun(ctx, run, steps); err != nil {
		return nil, nil, err
	}
	if err := e.advance(ctx, run, graph); err != nil {
		return nil, nil, err
	}
	freshRun, err := e.store.GetRun(ctx, tenantID, run.ID)
	if err != nil {
		return nil, nil, err
	}
	freshSteps, err := e.store.ListStepRuns(ctx, tenantID, run.ID)
	if err != nil {
		return nil, nil, err
	}
	return freshRun, freshSteps, nil
}

// CompleteHumanTask resolves a waiting human step and continues the workflow.
func (e *Engine) CompleteHumanTask(ctx context.Context, tenantID, stepRunID string, outcome store.HumanTaskOutcome) (*store.WorkflowRun, []*store.WorkflowStepRun, error) {
	step, err := e.store.GetStepRun(ctx, tenantID, stepRunID)
	if err != nil {
		return nil, nil, err
	}
	if step.NodeType != store.WorkflowNodeHumanTask || step.Status != store.WorkflowStepWaitingHuman {
		return nil, nil, errors.New("workflow step is not waiting for human input")
	}
	run, graph, err := e.loadRunAndGraph(ctx, tenantID, step.RunID)
	if err != nil {
		return nil, nil, err
	}
	if run.Status == store.WorkflowRunCancelled {
		return nil, nil, errors.New("workflow run is cancelled")
	}
	now := time.Now()
	step.Status = store.WorkflowStepSucceeded
	step.Outcome = outcome.Outcome
	step.OutputJSON = mustJSON(outcome.Output)
	step.CompletedAt = &now
	if err := e.store.UpdateStepRun(ctx, step); err != nil {
		return nil, nil, err
	}
	if len(outcome.Variables) > 0 {
		vars := decodeMap(run.VariablesJSON)
		mergeMap(vars, outcome.Variables)
		run.VariablesJSON = mustJSON(vars)
		if err := e.store.UpdateRun(ctx, run); err != nil {
			return nil, nil, err
		}
	}
	if err := e.advance(ctx, run, graph); err != nil {
		return nil, nil, err
	}
	return e.snapshot(ctx, tenantID, run.ID)
}

// Retry resumes a paused run by re-queueing its failed steps.
func (e *Engine) Retry(ctx context.Context, tenantID, runID string) (*store.WorkflowRun, []*store.WorkflowStepRun, error) {
	run, graph, err := e.loadRunAndGraph(ctx, tenantID, runID)
	if err != nil {
		return nil, nil, err
	}
	if run.Status != store.WorkflowRunPaused {
		return nil, nil, errors.New("workflow run is not paused")
	}
	steps, err := e.store.ListStepRuns(ctx, tenantID, runID)
	if err != nil {
		return nil, nil, err
	}
	for _, step := range steps {
		if step.Status == store.WorkflowStepFailed {
			step.Status = store.WorkflowStepReady
			step.Error = ""
			step.CompletedAt = nil
			if err := e.store.UpdateStepRun(ctx, step); err != nil {
				return nil, nil, err
			}
		}
	}
	run.Status = store.WorkflowRunRunning
	run.Error = ""
	if err := e.store.UpdateRun(ctx, run); err != nil {
		return nil, nil, err
	}
	if err := e.advance(ctx, run, graph); err != nil {
		return nil, nil, err
	}
	return e.snapshot(ctx, tenantID, runID)
}

func (e *Engine) Cancel(ctx context.Context, tenantID, runID string) (*store.WorkflowRun, error) {
	run, err := e.store.GetRun(ctx, tenantID, runID)
	if err != nil {
		return nil, err
	}
	if run.Status == store.WorkflowRunCompleted || run.Status == store.WorkflowRunCancelled {
		return run, nil
	}
	now := time.Now()
	run.Status = store.WorkflowRunCancelled
	run.CompletedAt = &now
	if err := e.store.UpdateRun(ctx, run); err != nil {
		return nil, err
	}
	return run, nil
}

func (e *Engine) loadRunAndGraph(ctx context.Context, tenantID, runID string) (*store.WorkflowRun, *store.WorkflowGraph, error) {
	run, err := e.store.GetRun(ctx, tenantID, runID)
	if err != nil {
		return nil, nil, err
	}
	version, err := e.store.GetVersion(ctx, tenantID, run.VersionID)
	if err != nil {
		return nil, nil, err
	}
	graph, err := ParseGraph(version.GraphJSON)
	if err != nil {
		return nil, nil, err
	}
	return run, graph, nil
}

func (e *Engine) snapshot(ctx context.Context, tenantID, runID string) (*store.WorkflowRun, []*store.WorkflowStepRun, error) {
	run, err := e.store.GetRun(ctx, tenantID, runID)
	if err != nil {
		return nil, nil, err
	}
	steps, err := e.store.ListStepRuns(ctx, tenantID, runID)
	return run, steps, err
}

func (e *Engine) advance(ctx context.Context, run *store.WorkflowRun, graph *store.WorkflowGraph) error {
	nodes, _, incoming := indexGraph(graph)
	for {
		steps, err := e.store.ListStepRuns(ctx, run.TenantID, run.ID)
		if err != nil {
			return err
		}
		stepByNode := make(map[string]*store.WorkflowStepRun, len(steps))
		for _, step := range steps {
			stepByNode[step.NodeID] = step
		}
		changed, err := e.resolvePending(ctx, run, graph, incoming, stepByNode)
		if err != nil {
			return err
		}
		if changed {
			continue
		}
		var ready *store.WorkflowStepRun
		for _, step := range steps {
			if step.Status == store.WorkflowStepReady {
				ready = step
				break
			}
		}
		if ready == nil {
			return e.refreshRunStatus(ctx, run, steps, nodes)
		}
		if err := e.executeReady(ctx, run, nodes[ready.NodeID], ready); err != nil {
			return err
		}
		if run.Status == store.WorkflowRunPaused {
			return nil
		}
	}
}

func (e *Engine) resolvePending(ctx context.Context, run *store.WorkflowRun, graph *store.WorkflowGraph, incoming map[string][]store.WorkflowEdge, steps map[string]*store.WorkflowStepRun) (bool, error) {
	changed := false
	contextMap := e.buildContext(run, steps)
	for _, node := range graph.Nodes {
		step := steps[node.ID]
		if step == nil || step.Status != store.WorkflowStepPending {
			continue
		}
		edges := incoming[node.ID]
		if len(edges) == 0 {
			continue
		}
		allTerminal := true
		anySelected := false
		for _, edge := range edges {
			source := steps[edge.From]
			if source == nil || !terminalStepStatus(source.Status) {
				allTerminal = false
				break
			}
			selected, err := edgeSelected(edge, source, contextMap)
			if err != nil {
				return false, err
			}
			if selected {
				anySelected = true
			}
		}
		if !allTerminal {
			continue
		}
		if anySelected {
			step.Status = store.WorkflowStepReady
		} else {
			now := time.Now()
			step.Status = store.WorkflowStepSkipped
			step.CompletedAt = &now
		}
		if err := e.store.UpdateStepRun(ctx, step); err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}

func edgeSelected(edge store.WorkflowEdge, source *store.WorkflowStepRun, contextMap map[string]any) (bool, error) {
	if source.Status != store.WorkflowStepSucceeded {
		return false, nil
	}
	if edge.Outcome != "" && edge.Outcome != source.Outcome {
		return false, nil
	}
	return conditionMatches(edge.Condition, contextMap)
}

func (e *Engine) executeReady(ctx context.Context, run *store.WorkflowRun, node store.WorkflowNode, step *store.WorkflowStepRun) error {
	now := time.Now()
	step.Status = store.WorkflowStepRunning
	step.Attempt++
	step.StartedAt = &now
	if err := e.store.UpdateStepRun(ctx, step); err != nil {
		return err
	}

	switch node.Type {
	case store.WorkflowNodeStart, store.WorkflowNodeEnd:
		step.Status = store.WorkflowStepSucceeded
		step.CompletedAt = &now
		step.OutputJSON = "{}"
		return e.store.UpdateStepRun(ctx, step)
	case store.WorkflowNodeHumanTask:
		step.Status = store.WorkflowStepWaitingHuman
		return e.store.UpdateStepRun(ctx, step)
	case store.WorkflowNodeServiceTask:
		output, err := e.executeServiceTask(ctx, run, node)
		return e.finishAutomatedStep(ctx, run, step, output, err)
	case store.WorkflowNodeAgentTask:
		output, err := e.agentExecutor.Execute(ctx, run.TenantID, run.StartedBy, node, e.stepPayload(run))
		return e.finishAutomatedStep(ctx, run, step, output, err)
	default:
		return fmt.Errorf("unsupported workflow node type %q", node.Type)
	}
}

func (e *Engine) executeServiceTask(ctx context.Context, run *store.WorkflowRun, node store.WorkflowNode) (map[string]any, error) {
	rawURL, _ := node.Config["url"].(string)
	if strings.TrimSpace(rawURL) == "" {
		return nil, errors.New("service task requires config.url")
	}
	method, _ := node.Config["method"].(string)
	if method == "" {
		method = http.MethodPost
	}
	body := mustJSON(e.stepPayload(run))
	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if headers, ok := node.Config["headers"].(map[string]any); ok {
		for k, v := range headers {
			req.Header.Set(k, fmt.Sprint(v))
		}
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("service task returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	var output map[string]any
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, fmt.Errorf("service task response must be JSON object: %w", err)
	}
	return output, nil
}

func (e *Engine) finishAutomatedStep(ctx context.Context, run *store.WorkflowRun, step *store.WorkflowStepRun, output map[string]any, execErr error) error {
	now := time.Now()
	if execErr != nil {
		step.Status = store.WorkflowStepFailed
		step.Error = execErr.Error()
		step.CompletedAt = &now
		if err := e.store.UpdateStepRun(ctx, step); err != nil {
			return err
		}
		run.Status = store.WorkflowRunPaused
		run.Error = execErr.Error()
		return e.store.UpdateRun(ctx, run)
	}
	step.Status = store.WorkflowStepSucceeded
	step.OutputJSON = mustJSON(output)
	step.CompletedAt = &now
	if err := e.store.UpdateStepRun(ctx, step); err != nil {
		return err
	}
	if variables, ok := output["variables"].(map[string]any); ok && len(variables) > 0 {
		current := decodeMap(run.VariablesJSON)
		mergeMap(current, variables)
		run.VariablesJSON = mustJSON(current)
		return e.store.UpdateRun(ctx, run)
	}
	return nil
}

func (e *Engine) refreshRunStatus(ctx context.Context, run *store.WorkflowRun, steps []*store.WorkflowStepRun, nodes map[string]store.WorkflowNode) error {
	if run.Status == store.WorkflowRunCancelled || run.Status == store.WorkflowRunPaused {
		return nil
	}
	waiting := false
	allTerminal := true
	endSucceeded := false
	for _, step := range steps {
		if step.Status == store.WorkflowStepWaitingHuman {
			waiting = true
		}
		if !terminalStepStatus(step.Status) {
			allTerminal = false
		}
		if nodes[step.NodeID].Type == store.WorkflowNodeEnd && step.Status == store.WorkflowStepSucceeded {
			endSucceeded = true
		}
	}
	switch {
	case waiting:
		run.Status = store.WorkflowRunWaiting
	case allTerminal && endSucceeded:
		now := time.Now()
		run.Status = store.WorkflowRunCompleted
		run.CompletedAt = &now
	case allTerminal:
		run.Status = store.WorkflowRunPaused
		run.Error = "workflow completed without reaching an end node"
	default:
		run.Status = store.WorkflowRunRunning
	}
	return e.store.UpdateRun(ctx, run)
}

func (e *Engine) buildContext(run *store.WorkflowRun, steps map[string]*store.WorkflowStepRun) map[string]any {
	contextMap := map[string]any{
		"input":     decodeMap(run.InputJSON),
		"variables": decodeMap(run.VariablesJSON),
		"steps":     map[string]any{},
	}
	stepMap := contextMap["steps"].(map[string]any)
	for nodeID, step := range steps {
		stepMap[nodeID] = map[string]any{
			"output":  decodeMap(step.OutputJSON),
			"outcome": step.Outcome,
			"status":  step.Status,
		}
	}
	return contextMap
}

func (e *Engine) stepPayload(run *store.WorkflowRun) map[string]any {
	return map[string]any{
		"run_id":    run.ID,
		"input":     decodeMap(run.InputJSON),
		"variables": decodeMap(run.VariablesJSON),
	}
}

func mustJSON(v any) string {
	if v == nil {
		return "{}"
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func decodeMap(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func mergeMap(dst, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}
