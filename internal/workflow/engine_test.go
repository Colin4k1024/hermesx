package workflow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

func TestValidateGraphRejectsInvalidShapes(t *testing.T) {
	t.Run("missing start", func(t *testing.T) {
		err := ValidateGraph(&store.WorkflowGraph{
			Nodes: []store.WorkflowNode{{ID: "end", Type: store.WorkflowNodeEnd}},
		})
		if err == nil || !strings.Contains(err.Error(), "exactly one start") {
			t.Fatalf("expected missing start error, got %v", err)
		}
	})

	t.Run("cycle", func(t *testing.T) {
		err := ValidateGraph(&store.WorkflowGraph{
			Nodes: []store.WorkflowNode{
				{ID: "start", Type: store.WorkflowNodeStart},
				{ID: "a", Type: store.WorkflowNodeServiceTask},
				{ID: "end", Type: store.WorkflowNodeEnd},
			},
			Edges: []store.WorkflowEdge{
				{From: "start", To: "a"},
				{From: "a", To: "start"},
				{From: "a", To: "end"},
			},
		})
		if err == nil || !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("expected cycle error, got %v", err)
		}
	})
}

func TestEngineHumanApprovalAndConditionalBranch(t *testing.T) {
	ctx := context.Background()
	mem := newMemWorkflowStore()
	graph := store.WorkflowGraph{
		Nodes: []store.WorkflowNode{
			{ID: "start", Type: store.WorkflowNodeStart},
			{ID: "approve", Type: store.WorkflowNodeHumanTask, AssigneeRole: "manager"},
			{ID: "approved", Type: store.WorkflowNodeEnd},
			{ID: "rejected", Type: store.WorkflowNodeEnd},
		},
		Edges: []store.WorkflowEdge{
			{From: "start", To: "approve"},
			{From: "approve", To: "approved", Outcome: "approve"},
			{From: "approve", To: "rejected", Outcome: "reject"},
		},
	}
	def, version := createPublishedDefinition(t, mem, "tenant-1", graph)
	engine := NewEngine(mem, nil, nil)

	run, steps, err := engine.StartRun(ctx, "tenant-1", def.ID, "alice", map[string]any{"days": 2})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if run.VersionID != version.ID || run.Status != store.WorkflowRunWaiting {
		t.Fatalf("unexpected run after start: %+v", run)
	}
	approve := findStep(t, steps, "approve")
	if approve.Status != store.WorkflowStepWaitingHuman {
		t.Fatalf("approve status = %s", approve.Status)
	}

	run, steps, err = engine.CompleteHumanTask(ctx, "tenant-1", approve.ID, store.HumanTaskOutcome{Outcome: "reject"})
	if err != nil {
		t.Fatalf("CompleteHumanTask: %v", err)
	}
	if run.Status != store.WorkflowRunCompleted {
		t.Fatalf("run status = %s, want completed", run.Status)
	}
	if got := findStep(t, steps, "approved").Status; got != store.WorkflowStepSkipped {
		t.Fatalf("approved status = %s, want skipped", got)
	}
	if got := findStep(t, steps, "rejected").Status; got != store.WorkflowStepSucceeded {
		t.Fatalf("rejected status = %s, want succeeded", got)
	}
}

func TestEngineParallelJoinCompletes(t *testing.T) {
	ctx := context.Background()
	mem := newMemWorkflowStore()
	graph := store.WorkflowGraph{
		Nodes: []store.WorkflowNode{
			{ID: "start", Type: store.WorkflowNodeStart},
			{ID: "a", Type: store.WorkflowNodeServiceTask, Config: map[string]any{"url": "http://svc/a"}},
			{ID: "b", Type: store.WorkflowNodeServiceTask, Config: map[string]any{"url": "http://svc/b"}},
			{ID: "end", Type: store.WorkflowNodeEnd},
		},
		Edges: []store.WorkflowEdge{
			{From: "start", To: "a"},
			{From: "start", To: "b"},
			{From: "a", To: "end"},
			{From: "b", To: "end"},
		},
	}
	def, _ := createPublishedDefinition(t, mem, "tenant-1", graph)
	engine := NewEngine(mem, fakeHTTPClient{status: http.StatusOK, body: `{}`}, nil)
	run, steps, err := engine.StartRun(ctx, "tenant-1", def.ID, "alice", nil)
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if run.Status != store.WorkflowRunCompleted {
		t.Fatalf("run status = %s", run.Status)
	}
	for _, nodeID := range []string{"a", "b", "end"} {
		if got := findStep(t, steps, nodeID).Status; got != store.WorkflowStepSucceeded {
			t.Fatalf("%s status = %s", nodeID, got)
		}
	}
}

func TestEngineServiceFailurePausesAndRetryResumes(t *testing.T) {
	ctx := context.Background()
	mem := newMemWorkflowStore()
	graph := store.WorkflowGraph{
		Nodes: []store.WorkflowNode{
			{ID: "start", Type: store.WorkflowNodeStart},
			{ID: "sync", Type: store.WorkflowNodeServiceTask, Config: map[string]any{"url": "http://svc/sync"}},
			{ID: "end", Type: store.WorkflowNodeEnd},
		},
		Edges: []store.WorkflowEdge{{From: "start", To: "sync"}, {From: "sync", To: "end"}},
	}
	def, _ := createPublishedDefinition(t, mem, "tenant-1", graph)
	client := &switchHTTPClient{status: http.StatusInternalServerError, body: `{"error":"down"}`}
	engine := NewEngine(mem, client, nil)
	run, steps, err := engine.StartRun(ctx, "tenant-1", def.ID, "alice", nil)
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if run.Status != store.WorkflowRunPaused {
		t.Fatalf("run status = %s, want paused", run.Status)
	}
	if got := findStep(t, steps, "sync").Status; got != store.WorkflowStepFailed {
		t.Fatalf("sync status = %s, want failed", got)
	}

	client.status = http.StatusOK
	client.body = `{"variables":{"synced":true}}`
	run, steps, err = engine.Retry(ctx, "tenant-1", run.ID)
	if err != nil {
		t.Fatalf("Retry: %v", err)
	}
	if run.Status != store.WorkflowRunCompleted {
		t.Fatalf("run status = %s, want completed", run.Status)
	}
	var vars map[string]any
	_ = json.Unmarshal([]byte(run.VariablesJSON), &vars)
	if vars["synced"] != true {
		t.Fatalf("variables = %#v, want synced=true", vars)
	}
	if got := findStep(t, steps, "sync").Attempt; got != 2 {
		t.Fatalf("sync attempts = %d, want 2", got)
	}
}

func TestEngineRunRemainsPinnedToPublishedVersion(t *testing.T) {
	ctx := context.Background()
	mem := newMemWorkflowStore()
	v1 := store.WorkflowGraph{
		Nodes: []store.WorkflowNode{
			{ID: "start", Type: store.WorkflowNodeStart},
			{ID: "approve", Type: store.WorkflowNodeHumanTask, AssigneeRole: "manager"},
			{ID: "end", Type: store.WorkflowNodeEnd},
		},
		Edges: []store.WorkflowEdge{{From: "start", To: "approve"}, {From: "approve", To: "end", Outcome: "approve"}},
	}
	def, firstVersion := createPublishedDefinition(t, mem, "tenant-1", v1)
	engine := NewEngine(mem, nil, nil)
	run, steps, err := engine.StartRun(ctx, "tenant-1", def.ID, "alice", nil)
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	v2 := store.WorkflowGraph{
		Nodes: []store.WorkflowNode{
			{ID: "start", Type: store.WorkflowNodeStart},
			{ID: "approve", Type: store.WorkflowNodeHumanTask, AssigneeRole: "manager"},
			{ID: "sync", Type: store.WorkflowNodeServiceTask, Config: map[string]any{"url": "http://svc/sync"}},
			{ID: "end", Type: store.WorkflowNodeEnd},
		},
		Edges: []store.WorkflowEdge{
			{From: "start", To: "approve"},
			{From: "approve", To: "sync", Outcome: "approve"},
			{From: "sync", To: "end"},
		},
	}
	rawV2, _ := json.Marshal(v2)
	def.GraphJSON = string(rawV2)
	def.Status = store.WorkflowDefinitionDraft
	if err := mem.UpdateDefinition(ctx, def); err != nil {
		t.Fatalf("UpdateDefinition: %v", err)
	}
	newVersion := &store.WorkflowVersion{TenantID: "tenant-1", DefinitionID: def.ID, GraphJSON: string(rawV2)}
	if err := mem.CreateVersion(ctx, newVersion); err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}

	approve := findStep(t, steps, "approve")
	run, steps, err = engine.CompleteHumanTask(ctx, "tenant-1", approve.ID, store.HumanTaskOutcome{Outcome: "approve"})
	if err != nil {
		t.Fatalf("CompleteHumanTask: %v", err)
	}
	if run.VersionID != firstVersion.ID {
		t.Fatalf("run version = %s, want %s", run.VersionID, firstVersion.ID)
	}
	if run.Status != store.WorkflowRunCompleted {
		t.Fatalf("run status = %s", run.Status)
	}
	for _, step := range steps {
		if step.NodeID == "sync" {
			t.Fatal("v1 run unexpectedly gained v2 step")
		}
	}
}

func createPublishedDefinition(t *testing.T, mem *memWorkflowStore, tenantID string, graph store.WorkflowGraph) (*store.WorkflowDefinition, *store.WorkflowVersion) {
	t.Helper()
	raw, _ := json.Marshal(graph)
	def := &store.WorkflowDefinition{
		TenantID:  tenantID,
		Name:      "leave",
		Status:    store.WorkflowDefinitionDraft,
		GraphJSON: string(raw),
	}
	if err := mem.CreateDefinition(context.Background(), def); err != nil {
		t.Fatalf("CreateDefinition: %v", err)
	}
	version := &store.WorkflowVersion{TenantID: tenantID, DefinitionID: def.ID, GraphJSON: string(raw)}
	if err := mem.CreateVersion(context.Background(), version); err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}
	return def, version
}

func findStep(t *testing.T, steps []*store.WorkflowStepRun, nodeID string) *store.WorkflowStepRun {
	t.Helper()
	for _, step := range steps {
		if step.NodeID == nodeID {
			return step
		}
	}
	t.Fatalf("step %s not found", nodeID)
	return nil
}

type fakeHTTPClient struct {
	status int
	body   string
}

func (c fakeHTTPClient) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: c.status,
		Body:       io.NopCloser(strings.NewReader(c.body)),
	}, nil
}

type switchHTTPClient struct {
	status int
	body   string
}

func (c *switchHTTPClient) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: c.status,
		Body:       io.NopCloser(strings.NewReader(c.body)),
	}, nil
}

type memWorkflowStore struct {
	mu          sync.Mutex
	defs        map[string]*store.WorkflowDefinition
	versions    map[string]*store.WorkflowVersion
	runs        map[string]*store.WorkflowRun
	steps       map[string]*store.WorkflowStepRun
	stepOrder   map[string][]string
	nextVersion map[string]int
	seq         int
}

func newMemWorkflowStore() *memWorkflowStore {
	return &memWorkflowStore{
		defs:        map[string]*store.WorkflowDefinition{},
		versions:    map[string]*store.WorkflowVersion{},
		runs:        map[string]*store.WorkflowRun{},
		steps:       map[string]*store.WorkflowStepRun{},
		stepOrder:   map[string][]string{},
		nextVersion: map[string]int{},
	}
}

func (m *memWorkflowStore) nextID(prefix string) string {
	m.seq++
	return prefix + "-" + strconv.Itoa(m.seq)
}

func (m *memWorkflowStore) CreateDefinition(_ context.Context, def *store.WorkflowDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if def.ID == "" {
		def.ID = m.nextID("def")
	}
	now := time.Now()
	def.CreatedAt, def.UpdatedAt = now, now
	cp := *def
	m.defs[def.ID] = &cp
	return nil
}

func (m *memWorkflowStore) UpdateDefinition(_ context.Context, def *store.WorkflowDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.defs[def.ID]; !ok {
		return store.ErrNotFound
	}
	def.UpdatedAt = time.Now()
	cp := *def
	m.defs[def.ID] = &cp
	return nil
}

func (m *memWorkflowStore) GetDefinition(_ context.Context, tenantID, id string) (*store.WorkflowDefinition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	def, ok := m.defs[id]
	if !ok || def.TenantID != tenantID {
		return nil, store.ErrNotFound
	}
	cp := *def
	return &cp, nil
}

func (m *memWorkflowStore) ListDefinitions(_ context.Context, tenantID string) ([]*store.WorkflowDefinition, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*store.WorkflowDefinition
	for _, def := range m.defs {
		if def.TenantID == tenantID {
			cp := *def
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *memWorkflowStore) CreateVersion(_ context.Context, version *store.WorkflowVersion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if version.ID == "" {
		version.ID = m.nextID("ver")
	}
	m.nextVersion[version.DefinitionID]++
	version.Version = m.nextVersion[version.DefinitionID]
	version.PublishedAt = time.Now()
	cp := *version
	m.versions[version.ID] = &cp
	if def := m.defs[version.DefinitionID]; def != nil {
		def.Status = store.WorkflowDefinitionPublished
		def.LatestVersionID = version.ID
	}
	return nil
}

func (m *memWorkflowStore) GetVersion(_ context.Context, tenantID, id string) (*store.WorkflowVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.versions[id]
	if !ok || v.TenantID != tenantID {
		return nil, store.ErrNotFound
	}
	cp := *v
	return &cp, nil
}

func (m *memWorkflowStore) GetLatestVersion(_ context.Context, tenantID, definitionID string) (*store.WorkflowVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	def := m.defs[definitionID]
	if def == nil || def.TenantID != tenantID || def.LatestVersionID == "" {
		return nil, store.ErrNotFound
	}
	v := m.versions[def.LatestVersionID]
	cp := *v
	return &cp, nil
}

func (m *memWorkflowStore) CreateRun(_ context.Context, run *store.WorkflowRun, steps []*store.WorkflowStepRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if run.ID == "" {
		run.ID = m.nextID("run")
	}
	now := time.Now()
	run.StartedAt, run.UpdatedAt = now, now
	cpRun := *run
	m.runs[run.ID] = &cpRun
	for _, step := range steps {
		if step.ID == "" {
			step.ID = m.nextID("step")
		}
		step.RunID = run.ID
		step.TenantID = run.TenantID
		step.CreatedAt, step.UpdatedAt = now, now
		cp := *step
		m.steps[step.ID] = &cp
		m.stepOrder[run.ID] = append(m.stepOrder[run.ID], step.ID)
	}
	return nil
}

func (m *memWorkflowStore) GetRun(_ context.Context, tenantID, id string) (*store.WorkflowRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[id]
	if !ok || run.TenantID != tenantID {
		return nil, store.ErrNotFound
	}
	cp := *run
	return &cp, nil
}

func (m *memWorkflowStore) ListRuns(_ context.Context, tenantID string, _ store.WorkflowRunListOptions) ([]*store.WorkflowRun, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*store.WorkflowRun
	for _, run := range m.runs {
		if run.TenantID == tenantID {
			cp := *run
			out = append(out, &cp)
		}
	}
	return out, len(out), nil
}

func (m *memWorkflowStore) UpdateRun(_ context.Context, run *store.WorkflowRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.runs[run.ID]; !ok {
		return store.ErrNotFound
	}
	run.UpdatedAt = time.Now()
	cp := *run
	m.runs[run.ID] = &cp
	return nil
}

func (m *memWorkflowStore) GetStepRun(_ context.Context, tenantID, id string) (*store.WorkflowStepRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	step, ok := m.steps[id]
	if !ok || step.TenantID != tenantID {
		return nil, store.ErrNotFound
	}
	cp := *step
	return &cp, nil
}

func (m *memWorkflowStore) ListStepRuns(_ context.Context, tenantID, runID string) ([]*store.WorkflowStepRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*store.WorkflowStepRun
	for _, id := range m.stepOrder[runID] {
		step := m.steps[id]
		if step.TenantID == tenantID {
			cp := *step
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *memWorkflowStore) UpdateStepRun(_ context.Context, step *store.WorkflowStepRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.steps[step.ID]; !ok {
		return store.ErrNotFound
	}
	step.UpdatedAt = time.Now()
	cp := *step
	m.steps[step.ID] = &cp
	return nil
}

func (m *memWorkflowStore) ListPendingHumanTasks(_ context.Context, tenantID, userID string, roles []string) ([]*store.WorkflowStepRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*store.WorkflowStepRun
	for _, step := range m.steps {
		if step.TenantID != tenantID || step.Status != store.WorkflowStepWaitingHuman {
			continue
		}
		if step.AssigneeUserID == userID || contains(roles, step.AssigneeRole) {
			cp := *step
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *memWorkflowStore) DeleteAllByTenant(_ context.Context, tenantID string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for id, step := range m.steps {
		if step.TenantID == tenantID {
			delete(m.steps, id)
			n++
		}
	}
	for id, run := range m.runs {
		if run.TenantID == tenantID {
			delete(m.runs, id)
			n++
		}
	}
	for id, version := range m.versions {
		if version.TenantID == tenantID {
			delete(m.versions, id)
			n++
		}
	}
	for id, def := range m.defs {
		if def.TenantID == tenantID {
			delete(m.defs, id)
			n++
		}
	}
	return n, nil
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

var _ store.WorkflowStore = (*memWorkflowStore)(nil)
