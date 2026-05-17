package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/middleware"
	"github.com/Colin4k1024/hermesx/internal/store"
	workflowrt "github.com/Colin4k1024/hermesx/internal/workflow"
)

type WorkflowHandler struct {
	store  store.WorkflowStore
	engine *workflowrt.Engine
}

func NewWorkflowHandler(s store.WorkflowStore) *WorkflowHandler {
	return &WorkflowHandler{store: s, engine: workflowrt.NewEngine(s, nil, nil)}
}

func NewWorkflowHandlerWithEngine(s store.WorkflowStore, engine *workflowrt.Engine) *WorkflowHandler {
	return &WorkflowHandler{store: s, engine: engine}
}

func (h *WorkflowHandler) ServeDefinitionsHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/workflow-definitions")
	path = strings.Trim(path, "/")
	parts := splitPath(path)
	switch {
	case r.Method == http.MethodPost && len(parts) == 0:
		h.createDefinition(w, r)
	case r.Method == http.MethodGet && len(parts) == 0:
		h.listDefinitions(w, r)
	case r.Method == http.MethodGet && len(parts) == 1:
		h.getDefinition(w, r, parts[0])
	case r.Method == http.MethodPut && len(parts) == 1:
		h.updateDefinition(w, r, parts[0])
	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "publish":
		h.publishDefinition(w, r, parts[0])
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *WorkflowHandler) ServeRunsHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/workflow-runs")
	path = strings.Trim(path, "/")
	parts := splitPath(path)
	switch {
	case r.Method == http.MethodPost && len(parts) == 0:
		h.startRun(w, r)
	case r.Method == http.MethodGet && len(parts) == 0:
		h.listRuns(w, r)
	case r.Method == http.MethodGet && len(parts) == 1:
		h.getRun(w, r, parts[0])
	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "cancel":
		h.cancelRun(w, r, parts[0])
	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "retry":
		h.retryRun(w, r, parts[0])
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *WorkflowHandler) ServeTasksHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/workflow-tasks")
	path = strings.Trim(path, "/")
	parts := splitPath(path)
	switch {
	case r.Method == http.MethodGet && len(parts) == 0:
		h.listTasks(w, r)
	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "complete":
		h.completeTask(w, r, parts[0])
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type workflowDefinitionRequest struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Graph       store.WorkflowGraph `json:"graph"`
}

func (h *WorkflowHandler) createDefinition(w http.ResponseWriter, r *http.Request) {
	tenantID, ac, ok := workflowAuthContext(w, r)
	if !ok {
		return
	}
	var req workflowDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if err := workflowrt.ValidateGraph(&req.Graph); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	graphJSON, _ := json.Marshal(req.Graph)
	def := &store.WorkflowDefinition{
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Status:      store.WorkflowDefinitionDraft,
		GraphJSON:   string(graphJSON),
		CreatedBy:   workflowIdentity(ac),
	}
	if err := h.store.CreateDefinition(r.Context(), def); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, def)
}

func (h *WorkflowHandler) updateDefinition(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, _, ok := workflowAuthContext(w, r)
	if !ok {
		return
	}
	def, err := h.store.GetDefinition(r.Context(), tenantID, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if def.Status == store.WorkflowDefinitionArchived {
		http.Error(w, "archived definition cannot be updated", http.StatusConflict)
		return
	}
	var req workflowDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if err := workflowrt.ValidateGraph(&req.Graph); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	graphJSON, _ := json.Marshal(req.Graph)
	def.Name = req.Name
	def.Description = req.Description
	def.Status = store.WorkflowDefinitionDraft
	def.GraphJSON = string(graphJSON)
	if err := h.store.UpdateDefinition(r.Context(), def); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, def)
}

func (h *WorkflowHandler) publishDefinition(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, ac, ok := workflowAuthContext(w, r)
	if !ok {
		return
	}
	def, err := h.store.GetDefinition(r.Context(), tenantID, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if _, err := workflowrt.ParseGraph(def.GraphJSON); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	version := &store.WorkflowVersion{
		TenantID:     tenantID,
		DefinitionID: def.ID,
		GraphJSON:    def.GraphJSON,
		PublishedBy:  workflowIdentity(ac),
	}
	if err := h.store.CreateVersion(r.Context(), version); err != nil {
		http.Error(w, "publish failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, version)
}

func (h *WorkflowHandler) listDefinitions(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := workflowAuthContext(w, r)
	if !ok {
		return
	}
	defs, err := h.store.ListDefinitions(r.Context(), tenantID)
	if err != nil {
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workflow_definitions": defs, "count": len(defs)})
}

func (h *WorkflowHandler) getDefinition(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, _, ok := workflowAuthContext(w, r)
	if !ok {
		return
	}
	def, err := h.store.GetDefinition(r.Context(), tenantID, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, def)
}

type startWorkflowRunRequest struct {
	DefinitionID string         `json:"definition_id"`
	Input        map[string]any `json:"input,omitempty"`
}

func (h *WorkflowHandler) startRun(w http.ResponseWriter, r *http.Request) {
	tenantID, ac, ok := workflowAuthContext(w, r)
	if !ok {
		return
	}
	var req startWorkflowRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.DefinitionID) == "" {
		http.Error(w, "definition_id is required", http.StatusBadRequest)
		return
	}
	run, steps, err := h.engine.StartRun(r.Context(), tenantID, req.DefinitionID, workflowIdentity(ac), req.Input)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "published workflow version not found", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"run": run, "steps": steps})
}

func (h *WorkflowHandler) listRuns(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := workflowAuthContext(w, r)
	if !ok {
		return
	}
	opts := store.WorkflowRunListOptions{
		DefinitionID: r.URL.Query().Get("definition_id"),
		Status:       r.URL.Query().Get("status"),
	}
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 {
		opts.Limit = n
	}
	if n, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && n >= 0 {
		opts.Offset = n
	}
	runs, total, err := h.store.ListRuns(r.Context(), tenantID, opts)
	if err != nil {
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workflow_runs": runs, "total": total})
}

func (h *WorkflowHandler) getRun(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, _, ok := workflowAuthContext(w, r)
	if !ok {
		return
	}
	run, err := h.store.GetRun(r.Context(), tenantID, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	steps, err := h.store.ListStepRuns(r.Context(), tenantID, id)
	if err != nil {
		http.Error(w, "list steps failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "steps": steps})
}

func (h *WorkflowHandler) cancelRun(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, _, ok := workflowAuthContext(w, r)
	if !ok {
		return
	}
	run, err := h.engine.Cancel(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "cancel failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *WorkflowHandler) retryRun(w http.ResponseWriter, r *http.Request, id string) {
	tenantID, _, ok := workflowAuthContext(w, r)
	if !ok {
		return
	}
	run, steps, err := h.engine.Retry(r.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "steps": steps})
}

func (h *WorkflowHandler) listTasks(w http.ResponseWriter, r *http.Request) {
	tenantID, ac, ok := workflowAuthContext(w, r)
	if !ok {
		return
	}
	tasks, err := h.store.ListPendingHumanTasks(r.Context(), tenantID, workflowIdentity(ac), ac.Roles)
	if err != nil {
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workflow_tasks": tasks, "count": len(tasks)})
}

func (h *WorkflowHandler) completeTask(w http.ResponseWriter, r *http.Request, stepID string) {
	tenantID, ac, ok := workflowAuthContext(w, r)
	if !ok {
		return
	}
	step, err := h.store.GetStepRun(r.Context(), tenantID, stepID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !canCompleteStep(ac, step) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req store.HumanTaskOutcome
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Outcome) == "" {
		http.Error(w, "outcome is required", http.StatusBadRequest)
		return
	}
	run, steps, err := h.engine.CompleteHumanTask(r.Context(), tenantID, stepID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "steps": steps})
}

func canCompleteStep(ac *auth.AuthContext, step *store.WorkflowStepRun) bool {
	if ac == nil {
		return false
	}
	if ac.HasRole("admin") {
		return true
	}
	if step.AssigneeUserID != "" && step.AssigneeUserID == workflowIdentity(ac) {
		return true
	}
	return step.AssigneeRole != "" && ac.HasRole(step.AssigneeRole)
}

func workflowAuthContext(w http.ResponseWriter, r *http.Request) (string, *auth.AuthContext, bool) {
	tenantID := middleware.TenantFromContext(r.Context())
	if tenantID == "" {
		http.Error(w, "tenant context required", http.StatusBadRequest)
		return "", nil, false
	}
	ac, ok := auth.FromContext(r.Context())
	if !ok || ac == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", nil, false
	}
	return tenantID, ac, true
}

func workflowIdentity(ac *auth.AuthContext) string {
	if ac == nil {
		return ""
	}
	if ac.UserID != "" {
		return ac.UserID
	}
	return ac.Identity
}

func splitPath(path string) []string {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
