package evolution_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	orisstore "github.com/Colin4k1024/Oris/sdks/go/store"
	"github.com/Colin4k1024/hermesx/internal/evolution"
	"github.com/Colin4k1024/hermesx/internal/llm"
)

// ── Config ────────────────────────────────────────────────────────────────

func TestDefaultConfig(t *testing.T) {
	cfg := evolution.DefaultConfig()
	if cfg.Enabled {
		t.Error("default config should be disabled")
	}
	if cfg.StorageMode != "sqlite" {
		t.Errorf("unexpected storage mode: %s", cfg.StorageMode)
	}
	if cfg.MinConfidence != 0.5 {
		t.Errorf("unexpected min_confidence: %f", cfg.MinConfidence)
	}
	if cfg.ReplayThreshold != 0.75 {
		t.Errorf("unexpected replay_threshold: %f", cfg.ReplayThreshold)
	}
	if cfg.MaxGenesInPrompt != 3 {
		t.Errorf("unexpected max_genes_prompt: %d", cfg.MaxGenesInPrompt)
	}
}

// ── DetectTaskClass ───────────────────────────────────────────────────────

func msgs(userContent string) []llm.Message {
	return []llm.Message{{Role: "user", Content: userContent}}
}

func TestDetectTaskClass_Debug(t *testing.T) {
	cases := []string{
		"fix the error in main.go",
		"there is a bug when I run the server",
		"the server crashes on startup",
		"the function is broken, not working",
	}
	for _, c := range cases {
		got := evolution.DetectTaskClass(msgs(c), nil)
		if got != evolution.TaskClassCodingDebug {
			t.Errorf("input=%q: want %s, got %s", c, evolution.TaskClassCodingDebug, got)
		}
	}
}

func TestDetectTaskClass_Feature(t *testing.T) {
	cases := []string{
		"implement a new login endpoint",
		"create a function that parses CSV",
		"add rate limiting to the API",
		"build the user registration feature",
	}
	for _, c := range cases {
		got := evolution.DetectTaskClass(msgs(c), nil)
		if got != evolution.TaskClassCodingFeature {
			t.Errorf("input=%q: want %s, got %s", c, evolution.TaskClassCodingFeature, got)
		}
	}
}

func TestDetectTaskClass_Analysis(t *testing.T) {
	cases := []string{
		"explain how the middleware works",
		"review this code for issues",
		"what is the difference between sync.Map and a regular map",
		"why does this goroutine leak",
	}
	for _, c := range cases {
		got := evolution.DetectTaskClass(msgs(c), nil)
		if got != evolution.TaskClassAnalysis {
			t.Errorf("input=%q: want %s, got %s", c, evolution.TaskClassAnalysis, got)
		}
	}
}

func TestDetectTaskClass_Writing(t *testing.T) {
	cases := []string{
		"write documentation for the API",
		"update the readme with setup instructions",
		"add comments to this function",
	}
	for _, c := range cases {
		got := evolution.DetectTaskClass(msgs(c), nil)
		if got != evolution.TaskClassWritingDocs {
			t.Errorf("input=%q: want %s, got %s", c, evolution.TaskClassWritingDocs, got)
		}
	}
}

func TestDetectTaskClass_General(t *testing.T) {
	got := evolution.DetectTaskClass(msgs("hello, what can you do?"), nil)
	if got != evolution.TaskClassGeneral {
		t.Errorf("want %s, got %s", evolution.TaskClassGeneral, got)
	}
}

func TestDetectTaskClass_ToolOverride_Debug(t *testing.T) {
	// tool usage with a debug keyword → coding.debug
	got := evolution.DetectTaskClass(msgs("fix the error"), []string{"terminal"})
	if got != evolution.TaskClassCodingDebug {
		t.Errorf("want %s, got %s", evolution.TaskClassCodingDebug, got)
	}
}

func TestDetectTaskClass_ToolOverride_Feature(t *testing.T) {
	// coding tool + no debug keywords → coding.feature
	got := evolution.DetectTaskClass(msgs("hello there"), []string{"write_file"})
	if got != evolution.TaskClassCodingFeature {
		t.Errorf("want %s, got %s", evolution.TaskClassCodingFeature, got)
	}
}

func TestDetectTaskClass_EmptyMessages(t *testing.T) {
	got := evolution.DetectTaskClass(nil, nil)
	if got != evolution.TaskClassGeneral {
		t.Errorf("want general for empty messages, got %s", got)
	}
}

// ── GeneStore (SQLite) ────────────────────────────────────────────────────

func openTestStore(t *testing.T) *evolution.GeneStore {
	t.Helper()
	dir := t.TempDir()
	cfg := evolution.Config{
		StorageMode: "sqlite",
		DBPath:      filepath.Join(dir, "test_evolution.db"),
	}
	gs, err := evolution.Open(cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })
	return gs
}

func TestGeneStore_OpenClose(t *testing.T) {
	gs := openTestStore(t)
	if gs == nil {
		t.Fatal("expected non-nil GeneStore")
	}
}

func TestGeneStore_SaveAndQuery(t *testing.T) {
	gs := openTestStore(t)
	ctx := context.Background()

	gene := orisstore.Gene{
		GeneID:       "aabb1122",
		Name:         "test-gene",
		TaskClass:    evolution.TaskClassCodingFeature,
		Confidence:   0.9,
		Strategy:     map[string]any{"insight": "always write tests first"},
		Source:       "test",
		UseCount:     2,
		SuccessCount: 2,
	}

	if err := gs.Save(ctx, "", gene); err != nil {
		t.Fatalf("Save: %v", err)
	}

	results, err := gs.QueryTop(ctx, "", evolution.TaskClassCodingFeature, 0.5, 5)
	if err != nil {
		t.Fatalf("QueryTop: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].GeneID != gene.GeneID {
		t.Errorf("unexpected gene_id: %s", results[0].GeneID)
	}
}

func TestGeneStore_QueryTop_FiltersByConfidence(t *testing.T) {
	gs := openTestStore(t)
	ctx := context.Background()

	lowGene := orisstore.Gene{
		GeneID:     "low00001",
		Name:       "low-confidence",
		TaskClass:  evolution.TaskClassCodingDebug,
		Confidence: 0.3,
		Strategy:   map[string]any{"insight": "low confidence strategy"},
		Source:     "test",
	}
	highGene := orisstore.Gene{
		GeneID:     "high0001",
		Name:       "high-confidence",
		TaskClass:  evolution.TaskClassCodingDebug,
		Confidence: 0.9,
		Strategy:   map[string]any{"insight": "high confidence strategy"},
		Source:     "test",
	}

	_ = gs.Save(ctx, "", lowGene)
	_ = gs.Save(ctx, "", highGene)

	results, err := gs.QueryTop(ctx, "", evolution.TaskClassCodingDebug, 0.75, 10)
	if err != nil {
		t.Fatalf("QueryTop: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (above 0.75), got %d", len(results))
	}
	if results[0].GeneID != highGene.GeneID {
		t.Errorf("expected high-confidence gene, got %s", results[0].GeneID)
	}
}

func TestGeneStore_QueryTop_WrongClass(t *testing.T) {
	gs := openTestStore(t)
	ctx := context.Background()

	gene := orisstore.Gene{
		GeneID:    "cc334455",
		Name:      "coding-gene",
		TaskClass: evolution.TaskClassCodingFeature,
		Strategy:  map[string]any{"insight": "some insight"},
		Source:    "test",
	}
	_ = gs.Save(ctx, "", gene)

	results, err := gs.QueryTop(ctx, "", evolution.TaskClassAnalysis, 0.0, 10)
	if err != nil {
		t.Fatalf("QueryTop: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for different task class, got %d", len(results))
	}
}

func TestGeneStore_RecordOutcome_UpdatesConfidence(t *testing.T) {
	gs := openTestStore(t)
	ctx := context.Background()

	gene := orisstore.Gene{
		GeneID:       "dd556677",
		Name:         "outcome-gene",
		TaskClass:    evolution.TaskClassGeneral,
		Confidence:   1.0,
		Strategy:     map[string]any{"insight": "be concise"},
		Source:       "test",
		UseCount:     1,
		SuccessCount: 1,
	}
	_ = gs.Save(ctx, "", gene)

	// Record a failure — confidence should drop to 0.5 (1 success / 2 uses).
	if err := gs.RecordOutcome(ctx, "", gene.GeneID, false); err != nil {
		t.Fatalf("RecordOutcome: %v", err)
	}

	results, err := gs.QueryTop(ctx, "", evolution.TaskClassGeneral, 0.0, 5)
	if err != nil {
		t.Fatalf("QueryTop after RecordOutcome: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("gene disappeared after RecordOutcome")
	}
	got := results[0].Confidence
	want := 0.5
	if got != want {
		t.Errorf("confidence after 1 success 1 failure: want %.2f, got %.2f", want, got)
	}
}

func TestGeneStore_OpenMissingSQLiteDir(t *testing.T) {
	dir := t.TempDir()
	cfg := evolution.Config{
		StorageMode: "sqlite",
		DBPath:      filepath.Join(dir, "nested", "deep", "evolution.db"),
	}
	gs, err := evolution.Open(cfg)
	if err != nil {
		t.Fatalf("Open should create missing directories: %v", err)
	}
	_ = gs.Close()
}

func TestGeneStore_MySQLDSN_RequiredError(t *testing.T) {
	cfg := evolution.Config{StorageMode: "mysql", MySQLDSN: ""}
	_, err := evolution.Open(cfg)
	if err == nil {
		t.Fatal("expected error for empty mysql_dsn")
	}
}

// ── Improver ─────────────────────────────────────────────────────────────

// mockCompleter fulfills the chatCompleter interface used by Improver.
type mockCompleter struct {
	response string
	err      error
	called   bool
}

func (m *mockCompleter) CreateChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	return &llm.ChatResponse{Content: m.response}, nil
}

func newTestImprover(t *testing.T, mc *mockCompleter) (*evolution.Improver, *evolution.GeneStore) {
	t.Helper()
	gs := openTestStore(t)
	cfg := evolution.DefaultConfig()
	cfg.Enabled = true
	cfg.ReplayThreshold = 0.75
	cfg.MaxGenesInPrompt = 3
	imp := evolution.NewImprover(gs, mc, cfg)
	return imp, gs
}

func TestImprover_PreTurnEnrich_NoGenes(t *testing.T) {
	imp, _ := newTestImprover(t, nil)
	strategies := imp.PreTurnEnrich(context.Background(), "", evolution.TaskClassGeneral)
	if len(strategies) != 0 {
		t.Errorf("expected no strategies with empty store, got %d", len(strategies))
	}
}

func TestImprover_PreTurnEnrich_BelowThreshold(t *testing.T) {
	imp, gs := newTestImprover(t, nil)
	ctx := context.Background()

	// Gene exists but confidence is below replay threshold (0.75).
	gene := orisstore.Gene{
		GeneID:     "low00099",
		Name:       "low-gene",
		TaskClass:  evolution.TaskClassCodingFeature,
		Confidence: 0.6,
		Strategy:   map[string]any{"insight": "some low strategy"},
		Source:     "test",
	}
	_ = gs.Save(ctx, "", gene)

	strategies := imp.PreTurnEnrich(ctx, "", evolution.TaskClassCodingFeature)
	if len(strategies) != 0 {
		t.Errorf("expected no strategies below threshold, got %d", len(strategies))
	}
}

func TestImprover_PreTurnEnrich_ReturnsStrategies(t *testing.T) {
	imp, gs := newTestImprover(t, nil)
	ctx := context.Background()

	gene := orisstore.Gene{
		GeneID:     "high0099",
		Name:       "high-gene",
		TaskClass:  evolution.TaskClassCodingFeature,
		Confidence: 0.9,
		Strategy:   map[string]any{"insight": "use table-driven tests"},
		Source:     "test",
	}
	_ = gs.Save(ctx, "", gene)

	strategies := imp.PreTurnEnrich(ctx, "", evolution.TaskClassCodingFeature)
	if len(strategies) != 1 {
		t.Fatalf("expected 1 strategy, got %d", len(strategies))
	}
	if strategies[0] != "use table-driven tests" {
		t.Errorf("unexpected strategy text: %s", strategies[0])
	}
}

func TestImprover_PreTurnEnrich_MaxGenesLimit(t *testing.T) {
	imp, gs := newTestImprover(t, nil)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = gs.Save(ctx, "", orisstore.Gene{
			GeneID:     "limgn" + string(rune('0'+i)),
			Name:       "gene",
			TaskClass:  evolution.TaskClassAnalysis,
			Confidence: 0.9,
			Strategy:   map[string]any{"insight": "strategy"},
			Source:     "test",
		})
	}

	strategies := imp.PreTurnEnrich(ctx, "", evolution.TaskClassAnalysis)
	if len(strategies) > 3 {
		t.Errorf("expected at most 3 strategies (MaxGenesInPrompt), got %d", len(strategies))
	}
}

func TestImprover_PostTurnRecord_NewGene(t *testing.T) {
	mc := &mockCompleter{response: "always handle errors explicitly"}
	imp, gs := newTestImprover(t, mc)
	ctx := context.Background()

	conversation := []llm.Message{
		{Role: "user", Content: "implement a retry function"},
		{Role: "assistant", Content: "Here is the retry function..."},
	}

	imp.PostTurnRecord(ctx, "", conversation, true)

	// Completer should have been called to generate an insight.
	if !mc.called {
		t.Error("expected LLM completer to be called for new gene")
	}

	// Gene should now be queryable.
	results, err := gs.QueryTop(ctx, "", evolution.TaskClassCodingFeature, 0.0, 5)
	if err != nil {
		t.Fatalf("QueryTop: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected new gene to be saved after PostTurnRecord")
	}
}

func TestImprover_PostTurnRecord_ExistingGene_NoLLMCall(t *testing.T) {
	mc := &mockCompleter{response: "irrelevant"}
	imp, gs := newTestImprover(t, mc)
	ctx := context.Background()

	// Pre-seed a high-confidence gene so PostTurnRecord finds it.
	gene := orisstore.Gene{
		GeneID:       "existing1",
		Name:         "existing",
		TaskClass:    evolution.TaskClassCodingFeature,
		Confidence:   0.9,
		Strategy:     map[string]any{"insight": "write small functions"},
		Source:       "test",
		UseCount:     3,
		SuccessCount: 3,
	}
	_ = gs.Save(ctx, "", gene)

	conversation := []llm.Message{
		{Role: "user", Content: "implement a helper function"},
	}

	imp.PostTurnRecord(ctx, "", conversation, true)

	if mc.called {
		t.Error("LLM should not be called when high-confidence gene already exists")
	}

	// Gene use count should have incremented.
	results, _ := gs.QueryTop(ctx, "", evolution.TaskClassCodingFeature, 0.75, 5)
	if len(results) == 0 {
		t.Fatal("gene should still be queryable after outcome record")
	}
	if results[0].UseCount <= 3 {
		t.Errorf("expected use_count > 3 after RecordOutcome, got %d", results[0].UseCount)
	}
}

func TestImprover_PostTurnRecord_LLMError_NoGeneStored(t *testing.T) {
	mc := &mockCompleter{err: os.ErrDeadlineExceeded}
	imp, gs := newTestImprover(t, mc)
	ctx := context.Background()

	conversation := []llm.Message{
		{Role: "user", Content: "explain goroutines"},
	}

	imp.PostTurnRecord(ctx, "", conversation, true)

	// LLM was attempted but failed — no gene should be stored.
	results, _ := gs.QueryTop(ctx, "", evolution.TaskClassAnalysis, 0.0, 5)
	if len(results) != 0 {
		t.Errorf("expected no gene on LLM error, got %d", len(results))
	}
}
