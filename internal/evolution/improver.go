package evolution

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"strings"
	"time"

	orisstore "github.com/Colin4k1024/Oris/sdks/go/store"
	"github.com/Colin4k1024/hermesx/internal/llm"
)

// chatCompleter is the minimal LLM interface required by the Improver.
// Structurally compatible with the same-named interface in internal/agent.
type chatCompleter interface {
	CreateChatCompletion(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

// Improver is the Oris evolution path, running in parallel with SelfImprover.
// It stores and replays high-confidence behavioral strategies as Oris genes.
type Improver struct {
	store     *GeneStore
	completer chatCompleter // optional auxiliary LLM for insight generation
	cfg       Config
}

// NewImprover creates an Improver. completer may be nil (disables LLM insight generation).
func NewImprover(gs *GeneStore, completer chatCompleter, cfg Config) *Improver {
	return &Improver{store: gs, completer: completer, cfg: cfg}
}

// PreTurnEnrich queries the gene store for high-confidence strategies and returns their text.
// tenantID scopes the query to prevent cross-tenant gene leakage (B2).
// Insight text is sanitized before use to prevent prompt injection (B1).
func (imp *Improver) PreTurnEnrich(ctx context.Context, tenantID string, taskClass TaskClass) []string {
	if imp.store == nil {
		return nil
	}
	genes, err := imp.store.QueryTop(ctx, tenantID, taskClass, imp.cfg.ReplayThreshold, imp.cfg.MaxGenesInPrompt)
	if err != nil {
		slog.Debug("evolution: pre-turn gene query failed", "error", err)
		return nil
	}
	var strategies []string
	for _, g := range genes {
		if raw, ok := g.Strategy["insight"].(string); ok && raw != "" {
			if text := sanitizeInsight(raw); text != "" { // B1: sanitize before injecting into prompt
				strategies = append(strategies, text)
			}
		}
	}
	if len(strategies) > 0 {
		slog.Debug("evolution: injecting gene strategies",
			"task_class", taskClass, "count", len(strategies))
	}
	return strategies
}

// PostTurnRecord is called asynchronously after RunConversation completes.
// tenantID scopes all store operations to prevent cross-tenant gene sharing (B2).
func (imp *Improver) PostTurnRecord(ctx context.Context, tenantID string, messages []llm.Message, completed bool) {
	if imp.store == nil {
		return
	}
	taskClass := DetectTaskClass(messages, nil)

	// Check for existing high-confidence gene — update stats only.
	existing, err := imp.store.QueryTop(ctx, tenantID, taskClass, imp.cfg.ReplayThreshold, 1)
	if err != nil {
		slog.Debug("evolution: post-turn query failed", "error", err)
		return
	}
	if len(existing) > 0 {
		if err := imp.store.RecordOutcome(ctx, tenantID, existing[0].GeneID, completed); err != nil {
			slog.Debug("evolution: record outcome failed", "error", err)
		}
		return
	}

	// No qualifying gene — generate a new insight if completer is available.
	if imp.completer == nil {
		return
	}
	insight, err := imp.generateInsight(ctx, messages, taskClass)
	if err != nil || insight == "" {
		slog.Debug("evolution: insight generation failed", "error", err)
		return
	}

	now := time.Now().UTC()
	geneID := geneIDFor(tenantID, taskClass, insight)
	successCount := 0
	if completed {
		successCount = 1
	}
	gene := orisstore.Gene{
		GeneID:       geneID,
		Name:         fmt.Sprintf("hermesx-%s-%s", taskClass, geneID[:8]),
		TaskClass:    string(taskClass), // tenantTaskClass applied inside store.Save
		Confidence:   float64(successCount),
		Strategy:     map[string]any{"insight": insight},
		Source:       "local",
		UseCount:     1,
		SuccessCount: successCount,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := imp.store.Save(ctx, tenantID, gene); err != nil {
		slog.Warn("evolution: save gene failed", "gene_id", geneID, "error", err)
		return
	}
	slog.Info("evolution: new gene saved",
		"gene_id", geneID, "task_class", taskClass, "completed", completed)
}

// generateInsight asks the auxiliary LLM to distill one behavioral strategy.
// The returned insight is sanitized to prevent prompt injection (B1).
func (imp *Improver) generateInsight(ctx context.Context, messages []llm.Message, taskClass TaskClass) (string, error) {
	prompt := buildInsightPrompt(messages, taskClass)
	resp, err := imp.completer.CreateChatCompletion(ctx, llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", err
	}
	return sanitizeInsight(strings.TrimSpace(resp.Content)), nil // B1: sanitize LLM output
}

func buildInsightPrompt(messages []llm.Message, taskClass TaskClass) string {
	var sb strings.Builder
	sb.WriteString("Analyze the following conversation (task class: ")
	sb.WriteString(string(taskClass))
	sb.WriteString(") and distill ONE concise, actionable behavioral strategy (max 2 sentences) ")
	sb.WriteString("that would improve similar future conversations. ")
	sb.WriteString("Focus on approach clarity, efficiency, and accuracy. ")
	sb.WriteString("Output only the strategy text, no preamble.\n\n")
	for _, m := range messages {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		content := m.Content
		if len([]rune(content)) > 200 {
			content = string([]rune(content)[:200]) + "..."
		}
		fmt.Fprintf(&sb, "[%s]: %s\n", m.Role, content)
	}
	return sb.String()
}

// geneIDFor derives a stable 8-byte hex ID from tenantID, taskClass, and insight (B2).
func geneIDFor(tenantID string, taskClass TaskClass, insight string) string {
	h := sha256.Sum256([]byte(tenantID + "|" + string(taskClass) + "|" + insight))
	return fmt.Sprintf("%x", h[:8])
}
