package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/google/uuid"
)

type myWorkflowStore struct{ db *sql.DB }

func (s *myWorkflowStore) CreateDefinition(ctx context.Context, def *store.WorkflowDefinition) error {
	if def.ID == "" {
		def.ID = uuid.New().String()
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO workflow_definitions
		 (id, tenant_id, name, description, status, graph_json, latest_version_id, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		def.ID, def.TenantID, def.Name, def.Description, def.Status, jsonOrEmpty(def.GraphJSON), nullStr(def.LatestVersionID), def.CreatedBy); err != nil {
		return fmt.Errorf("mysql create workflow definition: %w", err)
	}
	return s.db.QueryRowContext(ctx,
		`SELECT created_at, updated_at FROM workflow_definitions WHERE tenant_id=? AND id=?`, def.TenantID, def.ID).
		Scan(&def.CreatedAt, &def.UpdatedAt)
}

func (s *myWorkflowStore) UpdateDefinition(ctx context.Context, def *store.WorkflowDefinition) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE workflow_definitions
		 SET name=?, description=?, status=?, graph_json=?, latest_version_id=?
		 WHERE tenant_id=? AND id=?`,
		def.Name, def.Description, def.Status, jsonOrEmpty(def.GraphJSON), nullStr(def.LatestVersionID), def.TenantID, def.ID)
	if err != nil {
		return fmt.Errorf("mysql update workflow definition: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return s.db.QueryRowContext(ctx,
		`SELECT created_at, updated_at FROM workflow_definitions WHERE tenant_id=? AND id=?`, def.TenantID, def.ID).
		Scan(&def.CreatedAt, &def.UpdatedAt)
}

func (s *myWorkflowStore) GetDefinition(ctx context.Context, tenantID, definitionID string) (*store.WorkflowDefinition, error) {
	def := &store.WorkflowDefinition{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, name, description, status, graph_json, COALESCE(latest_version_id,''), created_by, created_at, updated_at
		 FROM workflow_definitions WHERE tenant_id=? AND id=?`, tenantID, definitionID).
		Scan(&def.ID, &def.TenantID, &def.Name, &def.Description, &def.Status, &def.GraphJSON,
			&def.LatestVersionID, &def.CreatedBy, &def.CreatedAt, &def.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mysql get workflow definition: %w", err)
	}
	return def, nil
}

func (s *myWorkflowStore) ListDefinitions(ctx context.Context, tenantID string) ([]*store.WorkflowDefinition, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, name, description, status, graph_json, COALESCE(latest_version_id,''), created_by, created_at, updated_at
		 FROM workflow_definitions WHERE tenant_id=? ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("mysql list workflow definitions: %w", err)
	}
	defer rows.Close()
	var defs []*store.WorkflowDefinition
	for rows.Next() {
		def := &store.WorkflowDefinition{}
		if err := rows.Scan(&def.ID, &def.TenantID, &def.Name, &def.Description, &def.Status, &def.GraphJSON,
			&def.LatestVersionID, &def.CreatedBy, &def.CreatedAt, &def.UpdatedAt); err != nil {
			return nil, fmt.Errorf("mysql scan workflow definition: %w", err)
		}
		defs = append(defs, def)
	}
	return defs, rows.Err()
}

func (s *myWorkflowStore) CreateVersion(ctx context.Context, version *store.WorkflowVersion) error {
	if version.ID == "" {
		version.ID = uuid.New().String()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql begin version tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version),0)+1 FROM workflow_versions WHERE tenant_id=? AND definition_id=?`,
		version.TenantID, version.DefinitionID).Scan(&version.Version); err != nil {
		return fmt.Errorf("mysql next workflow version: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO workflow_versions
		 (id, tenant_id, definition_id, version, graph_json, published_by)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		version.ID, version.TenantID, version.DefinitionID, version.Version, jsonOrEmpty(version.GraphJSON), version.PublishedBy); err != nil {
		return fmt.Errorf("mysql create workflow version: %w", err)
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE workflow_definitions SET status='published', latest_version_id=? WHERE tenant_id=? AND id=?`,
		version.ID, version.TenantID, version.DefinitionID)
	if err != nil {
		return fmt.Errorf("mysql attach workflow version: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	if err := tx.QueryRowContext(ctx, `SELECT published_at FROM workflow_versions WHERE id=?`, version.ID).
		Scan(&version.PublishedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *myWorkflowStore) GetVersion(ctx context.Context, tenantID, versionID string) (*store.WorkflowVersion, error) {
	v := &store.WorkflowVersion{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, definition_id, version, graph_json, published_by, published_at
		 FROM workflow_versions WHERE tenant_id=? AND id=?`, tenantID, versionID).
		Scan(&v.ID, &v.TenantID, &v.DefinitionID, &v.Version, &v.GraphJSON, &v.PublishedBy, &v.PublishedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mysql get workflow version: %w", err)
	}
	return v, nil
}

func (s *myWorkflowStore) GetLatestVersion(ctx context.Context, tenantID, definitionID string) (*store.WorkflowVersion, error) {
	v := &store.WorkflowVersion{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, definition_id, version, graph_json, published_by, published_at
		 FROM workflow_versions WHERE tenant_id=? AND definition_id=? ORDER BY version DESC LIMIT 1`,
		tenantID, definitionID).
		Scan(&v.ID, &v.TenantID, &v.DefinitionID, &v.Version, &v.GraphJSON, &v.PublishedBy, &v.PublishedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mysql latest workflow version: %w", err)
	}
	return v, nil
}

func (s *myWorkflowStore) CreateRun(ctx context.Context, run *store.WorkflowRun, steps []*store.WorkflowStepRun) error {
	if run.ID == "" {
		run.ID = uuid.New().String()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql begin run tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO workflow_runs
		 (id, tenant_id, definition_id, version_id, status, started_by, input_json, variables_json, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.TenantID, run.DefinitionID, run.VersionID, run.Status, run.StartedBy,
		jsonOrEmpty(run.InputJSON), jsonOrEmpty(run.VariablesJSON), run.Error); err != nil {
		return fmt.Errorf("mysql create workflow run: %w", err)
	}
	for _, step := range steps {
		if step.ID == "" {
			step.ID = uuid.New().String()
		}
		step.TenantID = run.TenantID
		step.RunID = run.ID
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO workflow_step_runs
			 (id, tenant_id, run_id, node_id, node_type, status, attempt, assignee_user_id, assignee_role,
			  input_json, output_json, outcome, error)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			step.ID, step.TenantID, step.RunID, step.NodeID, step.NodeType, step.Status, step.Attempt,
			step.AssigneeUserID, step.AssigneeRole, jsonOrEmpty(step.InputJSON), jsonOrEmpty(step.OutputJSON), step.Outcome, step.Error); err != nil {
			return fmt.Errorf("mysql create workflow step: %w", err)
		}
	}
	if err := tx.QueryRowContext(ctx,
		`SELECT started_at, updated_at FROM workflow_runs WHERE id=?`, run.ID).Scan(&run.StartedAt, &run.UpdatedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *myWorkflowStore) GetRun(ctx context.Context, tenantID, runID string) (*store.WorkflowRun, error) {
	run := &store.WorkflowRun{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, definition_id, version_id, status, started_by, input_json, variables_json,
		        error, started_at, completed_at, updated_at
		 FROM workflow_runs WHERE tenant_id=? AND id=?`, tenantID, runID).
		Scan(&run.ID, &run.TenantID, &run.DefinitionID, &run.VersionID, &run.Status, &run.StartedBy,
			&run.InputJSON, &run.VariablesJSON, &run.Error, &run.StartedAt, &run.CompletedAt, &run.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mysql get workflow run: %w", err)
	}
	return run, nil
}

func (s *myWorkflowStore) ListRuns(ctx context.Context, tenantID string, opts store.WorkflowRunListOptions) ([]*store.WorkflowRun, int, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	where := []string{"tenant_id=?"}
	args := []any{tenantID}
	if opts.DefinitionID != "" {
		where = append(where, "definition_id=?")
		args = append(args, opts.DefinitionID)
	}
	if opts.Status != "" {
		where = append(where, "status=?")
		args = append(args, opts.Status)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_runs WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("mysql count workflow runs: %w", err)
	}
	queryArgs := append(append([]any{}, args...), limit, opts.Offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, definition_id, version_id, status, started_by, input_json, variables_json,
		        error, started_at, completed_at, updated_at
		 FROM workflow_runs WHERE `+whereSQL+` ORDER BY started_at DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("mysql list workflow runs: %w", err)
	}
	defer rows.Close()
	var runs []*store.WorkflowRun
	for rows.Next() {
		run := &store.WorkflowRun{}
		if err := rows.Scan(&run.ID, &run.TenantID, &run.DefinitionID, &run.VersionID, &run.Status, &run.StartedBy,
			&run.InputJSON, &run.VariablesJSON, &run.Error, &run.StartedAt, &run.CompletedAt, &run.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("mysql scan workflow run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, total, rows.Err()
}

func (s *myWorkflowStore) UpdateRun(ctx context.Context, run *store.WorkflowRun) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE workflow_runs SET status=?, variables_json=?, error=?, completed_at=? WHERE tenant_id=? AND id=?`,
		run.Status, jsonOrEmpty(run.VariablesJSON), run.Error, run.CompletedAt, run.TenantID, run.ID)
	if err != nil {
		return fmt.Errorf("mysql update workflow run: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return s.db.QueryRowContext(ctx, `SELECT updated_at FROM workflow_runs WHERE tenant_id=? AND id=?`, run.TenantID, run.ID).
		Scan(&run.UpdatedAt)
}

func (s *myWorkflowStore) GetStepRun(ctx context.Context, tenantID, stepRunID string) (*store.WorkflowStepRun, error) {
	step := &store.WorkflowStepRun{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, run_id, node_id, node_type, status, attempt, assignee_user_id, assignee_role,
		        input_json, output_json, outcome, error, started_at, completed_at, created_at, updated_at
		 FROM workflow_step_runs WHERE tenant_id=? AND id=?`, tenantID, stepRunID).
		Scan(&step.ID, &step.TenantID, &step.RunID, &step.NodeID, &step.NodeType, &step.Status, &step.Attempt,
			&step.AssigneeUserID, &step.AssigneeRole, &step.InputJSON, &step.OutputJSON, &step.Outcome, &step.Error,
			&step.StartedAt, &step.CompletedAt, &step.CreatedAt, &step.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mysql get workflow step: %w", err)
	}
	return step, nil
}

func (s *myWorkflowStore) ListStepRuns(ctx context.Context, tenantID, runID string) ([]*store.WorkflowStepRun, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, tenant_id, run_id, node_id, node_type, status, attempt, assignee_user_id, assignee_role,
		        input_json, output_json, outcome, error, started_at, completed_at, created_at, updated_at
		 FROM workflow_step_runs WHERE tenant_id=? AND run_id=? ORDER BY created_at ASC`, tenantID, runID)
	if err != nil {
		return nil, fmt.Errorf("mysql list workflow steps: %w", err)
	}
	defer rows.Close()
	var steps []*store.WorkflowStepRun
	for rows.Next() {
		step := &store.WorkflowStepRun{}
		if err := rows.Scan(&step.ID, &step.TenantID, &step.RunID, &step.NodeID, &step.NodeType, &step.Status, &step.Attempt,
			&step.AssigneeUserID, &step.AssigneeRole, &step.InputJSON, &step.OutputJSON, &step.Outcome, &step.Error,
			&step.StartedAt, &step.CompletedAt, &step.CreatedAt, &step.UpdatedAt); err != nil {
			return nil, fmt.Errorf("mysql scan workflow step: %w", err)
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (s *myWorkflowStore) UpdateStepRun(ctx context.Context, step *store.WorkflowStepRun) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE workflow_step_runs
		 SET status=?, attempt=?, input_json=?, output_json=?, outcome=?, error=?, started_at=?, completed_at=?
		 WHERE tenant_id=? AND id=?`,
		step.Status, step.Attempt, jsonOrEmpty(step.InputJSON), jsonOrEmpty(step.OutputJSON), step.Outcome,
		step.Error, step.StartedAt, step.CompletedAt, step.TenantID, step.ID)
	if err != nil {
		return fmt.Errorf("mysql update workflow step: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return s.db.QueryRowContext(ctx, `SELECT updated_at FROM workflow_step_runs WHERE tenant_id=? AND id=?`, step.TenantID, step.ID).
		Scan(&step.UpdatedAt)
}

func (s *myWorkflowStore) ListPendingHumanTasks(ctx context.Context, tenantID, userID string, roles []string) ([]*store.WorkflowStepRun, error) {
	args := []any{tenantID, userID}
	query := `SELECT id, tenant_id, run_id, node_id, node_type, status, attempt, assignee_user_id, assignee_role,
		        input_json, output_json, outcome, error, started_at, completed_at, created_at, updated_at
		 FROM workflow_step_runs
		 WHERE tenant_id=? AND status='waiting_human' AND (assignee_user_id=?`
	if len(roles) > 0 {
		query += ` OR assignee_role IN (` + strings.TrimRight(strings.Repeat("?,", len(roles)), ",") + `)`
		for _, role := range roles {
			args = append(args, role)
		}
	}
	query += `) ORDER BY created_at ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql list pending human tasks: %w", err)
	}
	defer rows.Close()
	var steps []*store.WorkflowStepRun
	for rows.Next() {
		step := &store.WorkflowStepRun{}
		if err := rows.Scan(&step.ID, &step.TenantID, &step.RunID, &step.NodeID, &step.NodeType, &step.Status, &step.Attempt,
			&step.AssigneeUserID, &step.AssigneeRole, &step.InputJSON, &step.OutputJSON, &step.Outcome, &step.Error,
			&step.StartedAt, &step.CompletedAt, &step.CreatedAt, &step.UpdatedAt); err != nil {
			return nil, fmt.Errorf("mysql scan pending human task: %w", err)
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (s *myWorkflowStore) DeleteAllByTenant(ctx context.Context, tenantID string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck
	var total int64
	for _, table := range []string{"workflow_step_runs", "workflow_runs", "workflow_versions", "workflow_definitions"} {
		res, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE tenant_id=?`, tenantID)
		if err != nil {
			return 0, fmt.Errorf("mysql delete %s: %w", table, err)
		}
		n, _ := res.RowsAffected()
		total += n
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return total, nil
}

func jsonOrEmpty(v string) string {
	if strings.TrimSpace(v) == "" {
		return "{}"
	}
	return v
}

var _ store.WorkflowStore = (*myWorkflowStore)(nil)
