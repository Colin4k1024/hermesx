package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/store"
)

var allowedNodeTypes = map[string]struct{}{
	store.WorkflowNodeStart:       {},
	store.WorkflowNodeHumanTask:   {},
	store.WorkflowNodeServiceTask: {},
	store.WorkflowNodeAgentTask:   {},
	store.WorkflowNodeEnd:         {},
}

var allowedConditionOps = map[string]struct{}{
	"":         {},
	"eq":       {},
	"ne":       {},
	"gt":       {},
	"gte":      {},
	"lt":       {},
	"lte":      {},
	"exists":   {},
	"contains": {},
}

// ParseGraph decodes a workflow graph from its immutable JSON representation.
func ParseGraph(raw string) (*store.WorkflowGraph, error) {
	var graph store.WorkflowGraph
	if err := json.Unmarshal([]byte(raw), &graph); err != nil {
		return nil, fmt.Errorf("invalid workflow graph JSON: %w", err)
	}
	if err := ValidateGraph(&graph); err != nil {
		return nil, err
	}
	return &graph, nil
}

// ValidateGraph ensures the workflow is a well-formed DAG with exactly one start
// and at least one end node.
func ValidateGraph(graph *store.WorkflowGraph) error {
	if graph == nil {
		return errors.New("workflow graph is required")
	}
	if len(graph.Nodes) == 0 {
		return errors.New("workflow graph must contain nodes")
	}

	nodes := make(map[string]store.WorkflowNode, len(graph.Nodes))
	startCount, endCount := 0, 0
	for _, node := range graph.Nodes {
		if strings.TrimSpace(node.ID) == "" {
			return errors.New("workflow node id is required")
		}
		if _, exists := nodes[node.ID]; exists {
			return fmt.Errorf("duplicate workflow node id %q", node.ID)
		}
		if _, ok := allowedNodeTypes[node.Type]; !ok {
			return fmt.Errorf("invalid workflow node type %q", node.Type)
		}
		if node.Type == store.WorkflowNodeStart {
			startCount++
		}
		if node.Type == store.WorkflowNodeEnd {
			endCount++
		}
		if node.Type == store.WorkflowNodeHumanTask &&
			strings.TrimSpace(node.AssigneeUserID) == "" &&
			strings.TrimSpace(node.AssigneeRole) == "" {
			return fmt.Errorf("human task %q requires assignee_user_id or assignee_role", node.ID)
		}
		nodes[node.ID] = node
	}
	if startCount != 1 {
		return fmt.Errorf("workflow graph must contain exactly one start node, got %d", startCount)
	}
	if endCount == 0 {
		return errors.New("workflow graph must contain at least one end node")
	}

	adj := make(map[string][]string)
	incoming := make(map[string]int)
	for _, edge := range graph.Edges {
		if _, ok := nodes[edge.From]; !ok {
			return fmt.Errorf("edge references unknown source node %q", edge.From)
		}
		if _, ok := nodes[edge.To]; !ok {
			return fmt.Errorf("edge references unknown target node %q", edge.To)
		}
		if edge.From == edge.To {
			return fmt.Errorf("self-loop edge on node %q", edge.From)
		}
		if edge.Condition != nil {
			if strings.TrimSpace(edge.Condition.Path) == "" {
				return errors.New("workflow condition path is required")
			}
			if _, ok := allowedConditionOps[edge.Condition.Op]; !ok {
				return fmt.Errorf("invalid workflow condition op %q", edge.Condition.Op)
			}
		}
		adj[edge.From] = append(adj[edge.From], edge.To)
		incoming[edge.To]++
	}

	for _, node := range graph.Nodes {
		if node.Type != store.WorkflowNodeStart && incoming[node.ID] == 0 {
			return fmt.Errorf("node %q is unreachable from start", node.ID)
		}
	}

	visited := map[string]bool{}
	onStack := map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if onStack[id] {
			return fmt.Errorf("workflow graph contains a cycle at node %q", id)
		}
		if visited[id] {
			return nil
		}
		visited[id] = true
		onStack[id] = true
		for _, next := range adj[id] {
			if err := visit(next); err != nil {
				return err
			}
		}
		onStack[id] = false
		return nil
	}
	for _, node := range graph.Nodes {
		if node.Type == store.WorkflowNodeStart {
			if err := visit(node.ID); err != nil {
				return err
			}
			break
		}
	}
	for _, node := range graph.Nodes {
		if !visited[node.ID] {
			return fmt.Errorf("node %q is not reachable from start", node.ID)
		}
	}
	return nil
}

func indexGraph(graph *store.WorkflowGraph) (map[string]store.WorkflowNode, map[string][]store.WorkflowEdge, map[string][]store.WorkflowEdge) {
	nodes := make(map[string]store.WorkflowNode, len(graph.Nodes))
	outgoing := make(map[string][]store.WorkflowEdge)
	incoming := make(map[string][]store.WorkflowEdge)
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}
	for _, edge := range graph.Edges {
		outgoing[edge.From] = append(outgoing[edge.From], edge)
		incoming[edge.To] = append(incoming[edge.To], edge)
	}
	return nodes, outgoing, incoming
}

func terminalStepStatus(status string) bool {
	return slices.Contains([]string{store.WorkflowStepSucceeded, store.WorkflowStepSkipped}, status)
}
