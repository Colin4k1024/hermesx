package pg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgWorkflowStore struct{ pool *pgxpool.Pool }

func (s *pgWorkflowStore) CreateDefinition(ctx context.Context, def *store.WorkflowDefinition) error {
	if def.ID == "" {
		def.ID = uuid.New().String()
	}
	return withTenantTx(ctx, s.pool, def.TenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`INSERT INTO workflow_definitions
			 (id, tenant_id, name, description, status, graph_json, latest_version_id, created_by)
			 VALUES ($1, $2, $3, $4, $5, $6::jsonb, NULLIF($7,'' )::uuid, $8)
			 RETURNING created_at, updated_at`,
			def.ID, def.TenantID, def.Name, def.Description, def.Status, def.GraphJSON, def.LatestVersionID, def.CreatedBy,
		).Scan(&def.CreatedAt, &def.UpdatedAt)
	})
}

func (s *pgWorkflowStore) UpdateDefinition(ctx context.Context, def *store.WorkflowDefinition) error {
	return withTenantTx(ctx, s.pool, def.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE workflow_definitions
			 SET name=$3, description=$4, status=$5, graph_json=$6::jsonb,
			     latest_version_id=NULLIF($7,'')::uuid, updated_at=now()
			 WHERE tenant_id=$1 AND id=$2`,
			def.TenantID, def.ID, def.Name, def.Description, def.Status, def.GraphJSON, def.LatestVersionID)
		if err != nil {
			return fmt.Errorf("pg update workflow definition: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return store.ErrNotFound
		}
		return tx.QueryRow(ctx,
			`SELECT created_at, updated_at FROM workflow_definitions WHERE tenant_id=$1 AND id=$2`,
			def.TenantID, def.ID).Scan(&def.CreatedAt, &def.UpdatedAt)
	})
}

func (s *pgWorkflowStore) GetDefinition(ctx context.Context, tenantID, definitionID string) (*store.WorkflowDefinition, error) {
	def := &store.WorkflowDefinition{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, description, status, graph_json::text,
		        COALESCE(latest_version_id::text,''), created_by, created_at, updated_at
		 FROM workflow_definitions WHERE tenant_id=$1 AND id=$2`,
		tenantID, definitionID).
		Scan(&def.ID, &def.TenantID, &def.Name, &def.Description, &def.Status, &def.GraphJSON,
			&def.LatestVersionID, &def.CreatedBy, &def.CreatedAt, &def.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pg get workflow definition: %w", err)
	}
	return def, nil
}

func (s *pgWorkflowStore) ListDefinitions(ctx context.Context, tenantID string) ([]*store.WorkflowDefinition, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, name, description, status, graph_json::text,
		        COALESCE(latest_version_id::text,''), created_by, created_at, updated_at
		 FROM workflow_definitions WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("pg list workflow definitions: %w", err)
	}
	defer rows.Close()

	var defs []*store.WorkflowDefinition
	for rows.Next() {
		def := &store.WorkflowDefinition{}
		if err := rows.Scan(&def.ID, &def.TenantID, &def.Name, &def.Description, &def.Status, &def.GraphJSON,
			&def.LatestVersionID, &def.CreatedBy, &def.CreatedAt, &def.UpdatedAt); err != nil {
			return nil, fmt.Errorf("pg scan workflow definition: %w", err)
		}
		defs = append(defs, def)
	}
	return defs, rows.Err()
}

func (s *pgWorkflowStore) CreateVersion(ctx context.Context, version *store.WorkflowVersion) error {
	if version.ID == "" {
		version.ID = uuid.New().String()
	}
	return withTenantTx(ctx, s.pool, version.TenantID, func(tx pgx.Tx) error {
		var nextVersion int
		if err := tx.QueryRow(ctx,
			`SELECT COALESCE(MAX(version), 0) + 1 FROM workflow_versions WHERE tenant_id=$1 AND definition_id=$2`,
			version.TenantID, version.DefinitionID).Scan(&nextVersion); err != nil {
			return fmt.Errorf("pg next workflow version: %w", err)
		}
		version.Version = nextVersion
		if err := tx.QueryRow(ctx,
			`INSERT INTO workflow_versions
			 (id, tenant_id, definition_id, version, graph_json, published_by)
			 VALUES ($1,$2,$3,$4,$5::jsonb,$6)
			 RETURNING published_at`,
			version.ID, version.TenantID, version.DefinitionID, version.Version, version.GraphJSON, version.PublishedBy,
		).Scan(&version.PublishedAt); err != nil {
			return fmt.Errorf("pg create workflow version: %w", err)
		}
		tag, err := tx.Exec(ctx,
			`UPDATE workflow_definitions
			 SET status='published', latest_version_id=$3, updated_at=now()
			 WHERE tenant_id=$1 AND id=$2`,
			version.TenantID, version.DefinitionID, version.ID)
		if err != nil {
			return fmt.Errorf("pg attach workflow version: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return store.ErrNotFound
		}
		return nil
	})
}

func (s *pgWorkflowStore) GetVersion(ctx context.Context, tenantID, versionID string) (*store.WorkflowVersion, error) {
	v := &store.WorkflowVersion{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, definition_id, version, graph_json::text, published_by, published_at
		 FROM workflow_versions WHERE tenant_id=$1 AND id=$2`, tenantID, versionID).
		Scan(&v.ID, &v.TenantID, &v.DefinitionID, &v.Version, &v.GraphJSON, &v.PublishedBy, &v.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pg get workflow version: %w", err)
	}
	return v, nil
}

func (s *pgWorkflowStore) GetLatestVersion(ctx context.Context, tenantID, definitionID string) (*store.WorkflowVersion, error) {
	v := &store.WorkflowVersion{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, definition_id, version, graph_json::text, published_by, published_at
		 FROM workflow_versions
		 WHERE tenant_id=$1 AND definition_id=$2
		 ORDER BY version DESC LIMIT 1`,
		tenantID, definitionID).
		Scan(&v.ID, &v.TenantID, &v.DefinitionID, &v.Version, &v.GraphJSON, &v.PublishedBy, &v.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pg latest workflow version: %w", err)
	}
	return v, nil
}

func (s *pgWorkflowStore) CreateRun(ctx context.Context, run *store.WorkflowRun, steps []*store.WorkflowStepRun) error {
	if run.ID == "" {
		run.ID = uuid.New().String()
	}
	return withTenantTx(ctx, s.pool, run.TenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO workflow_runs
			 (id, tenant_id, definition_id, version_id, status, started_by, input_json, variables_json, error)
			 VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9)
			 RETURNING started_at, updated_at`,
			run.ID, run.TenantID, run.DefinitionID, run.VersionID, run.Status, run.StartedBy,
			run.InputJSON, run.VariablesJSON, run.Error).Scan(&run.StartedAt, &run.UpdatedAt); err != nil {
			return fmt.Errorf("pg create workflow run: %w", err)
		}
		for _, step := range steps {
			if step.ID == "" {
				step.ID = uuid.New().String()
			}
			step.RunID = run.ID
			step.TenantID = run.TenantID
			if err := tx.QueryRow(ctx,
				`INSERT INTO workflow_step_runs
				 (id, tenant_id, run_id, node_id, node_type, status, attempt, assignee_user_id, assignee_role,
				  input_json, output_json, outcome, error)
				 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11::jsonb,$12,$13)
				 RETURNING created_at, updated_at`,
				step.ID, step.TenantID, step.RunID, step.NodeID, step.NodeType, step.Status, step.Attempt,
				step.AssigneeUserID, step.AssigneeRole, jsonOrEmpty(step.InputJSON), jsonOrEmpty(step.OutputJSON), step.Outcome, step.Error,
			).Scan(&step.CreatedAt, &step.UpdatedAt); err != nil {
				return fmt.Errorf("pg create workflow step: %w", err)
			}
		}
		return nil
	})
}

func (s *pgWorkflowStore) GetRun(ctx context.Context, tenantID, runID string) (*store.WorkflowRun, error) {
	run := &store.WorkflowRun{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, definition_id, version_id, status, started_by,
		        input_json::text, variables_json::text, error, started_at, completed_at, updated_at
		 FROM workflow_runs WHERE tenant_id=$1 AND id=$2`, tenantID, runID).
		Scan(&run.ID, &run.TenantID, &run.DefinitionID, &run.VersionID, &run.Status, &run.StartedBy,
			&run.InputJSON, &run.VariablesJSON, &run.Error, &run.StartedAt, &run.CompletedAt, &run.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pg get workflow run: %w", err)
	}
	return run, nil
}

func (s *pgWorkflowStore) ListRuns(ctx context.Context, tenantID string, opts store.WorkflowRunListOptions) ([]*store.WorkflowRun, int, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	where := []string{"tenant_id=$1"}
	args := []any{tenantID}
	next := 2
	if opts.DefinitionID != "" {
		where = append(where, fmt.Sprintf("definition_id=$%d", next))
		args = append(args, opts.DefinitionID)
		next++
	}
	if opts.Status != "" {
		where = append(where, fmt.Sprintf("status=$%d", next))
		args = append(args, opts.Status)
		next++
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM workflow_runs WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("pg count workflow runs: %w", err)
	}
	queryArgs := append(append([]any{}, args...), limit, opts.Offset)
	rows, err := s.pool.Query(ctx,
		fmt.Sprintf(`SELECT id, tenant_id, definition_id, version_id, status, started_by,
		        input_json::text, variables_json::text, error, started_at, completed_at, updated_at
		 FROM workflow_runs WHERE %s ORDER BY started_at DESC LIMIT $%d OFFSET $%d`, whereSQL, next, next+1),
		queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("pg list workflow runs: %w", err)
	}
	defer rows.Close()
	var runs []*store.WorkflowRun
	for rows.Next() {
		run := &store.WorkflowRun{}
		if err := rows.Scan(&run.ID, &run.TenantID, &run.DefinitionID, &run.VersionID, &run.Status, &run.StartedBy,
			&run.InputJSON, &run.VariablesJSON, &run.Error, &run.StartedAt, &run.CompletedAt, &run.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("pg scan workflow run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, total, rows.Err()
}

func (s *pgWorkflowStore) UpdateRun(ctx context.Context, run *store.WorkflowRun) error {
	return withTenantTx(ctx, s.pool, run.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE workflow_runs
			 SET status=$3, variables_json=$4::jsonb, error=$5, completed_at=$6, updated_at=now()
			 WHERE tenant_id=$1 AND id=$2`,
			run.TenantID, run.ID, run.Status, jsonOrEmpty(run.VariablesJSON), run.Error, run.CompletedAt)
		if err != nil {
			return fmt.Errorf("pg update workflow run: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return store.ErrNotFound
		}
		return tx.QueryRow(ctx, `SELECT updated_at FROM workflow_runs WHERE tenant_id=$1 AND id=$2`,
			run.TenantID, run.ID).Scan(&run.UpdatedAt)
	})
}

func (s *pgWorkflowStore) GetStepRun(ctx context.Context, tenantID, stepRunID string) (*store.WorkflowStepRun, error) {
	step := &store.WorkflowStepRun{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, run_id, node_id, node_type, status, attempt, assignee_user_id, assignee_role,
		        input_json::text, output_json::text, outcome, error, started_at, completed_at, created_at, updated_at
		 FROM workflow_step_runs WHERE tenant_id=$1 AND id=$2`, tenantID, stepRunID).
		Scan(&step.ID, &step.TenantID, &step.RunID, &step.NodeID, &step.NodeType, &step.Status, &step.Attempt,
			&step.AssigneeUserID, &step.AssigneeRole, &step.InputJSON, &step.OutputJSON, &step.Outcome, &step.Error,
			&step.StartedAt, &step.CompletedAt, &step.CreatedAt, &step.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pg get workflow step: %w", err)
	}
	return step, nil
}

func (s *pgWorkflowStore) ListStepRuns(ctx context.Context, tenantID, runID string) ([]*store.WorkflowStepRun, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, run_id, node_id, node_type, status, attempt, assignee_user_id, assignee_role,
		        input_json::text, output_json::text, outcome, error, started_at, completed_at, created_at, updated_at
		 FROM workflow_step_runs WHERE tenant_id=$1 AND run_id=$2 ORDER BY created_at ASC`, tenantID, runID)
	if err != nil {
		return nil, fmt.Errorf("pg list workflow steps: %w", err)
	}
	defer rows.Close()
	var steps []*store.WorkflowStepRun
	for rows.Next() {
		step := &store.WorkflowStepRun{}
		if err := rows.Scan(&step.ID, &step.TenantID, &step.RunID, &step.NodeID, &step.NodeType, &step.Status, &step.Attempt,
			&step.AssigneeUserID, &step.AssigneeRole, &step.InputJSON, &step.OutputJSON, &step.Outcome, &step.Error,
			&step.StartedAt, &step.CompletedAt, &step.CreatedAt, &step.UpdatedAt); err != nil {
			return nil, fmt.Errorf("pg scan workflow step: %w", err)
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (s *pgWorkflowStore) UpdateStepRun(ctx context.Context, step *store.WorkflowStepRun) error {
	return withTenantTx(ctx, s.pool, step.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE workflow_step_runs
			 SET status=$3, attempt=$4, input_json=$5::jsonb, output_json=$6::jsonb, outcome=$7,
			     error=$8, started_at=$9, completed_at=$10, updated_at=now()
			 WHERE tenant_id=$1 AND id=$2`,
			step.TenantID, step.ID, step.Status, step.Attempt, jsonOrEmpty(step.InputJSON), jsonOrEmpty(step.OutputJSON),
			step.Outcome, step.Error, step.StartedAt, step.CompletedAt)
		if err != nil {
			return fmt.Errorf("pg update workflow step: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return store.ErrNotFound
		}
		return tx.QueryRow(ctx, `SELECT updated_at FROM workflow_step_runs WHERE tenant_id=$1 AND id=$2`,
			step.TenantID, step.ID).Scan(&step.UpdatedAt)
	})
}

func (s *pgWorkflowStore) ListPendingHumanTasks(ctx context.Context, tenantID, userID string, roles []string) ([]*store.WorkflowStepRun, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, run_id, node_id, node_type, status, attempt, assignee_user_id, assignee_role,
		        input_json::text, output_json::text, outcome, error, started_at, completed_at, created_at, updated_at
		 FROM workflow_step_runs
		 WHERE tenant_id=$1 AND status='waiting_human'
		   AND (assignee_user_id=$2 OR assignee_role = ANY($3))
		 ORDER BY created_at ASC`,
		tenantID, userID, roles)
	if err != nil {
		return nil, fmt.Errorf("pg list pending human tasks: %w", err)
	}
	defer rows.Close()
	var steps []*store.WorkflowStepRun
	for rows.Next() {
		step := &store.WorkflowStepRun{}
		if err := rows.Scan(&step.ID, &step.TenantID, &step.RunID, &step.NodeID, &step.NodeType, &step.Status, &step.Attempt,
			&step.AssigneeUserID, &step.AssigneeRole, &step.InputJSON, &step.OutputJSON, &step.Outcome, &step.Error,
			&step.StartedAt, &step.CompletedAt, &step.CreatedAt, &step.UpdatedAt); err != nil {
			return nil, fmt.Errorf("pg scan pending human task: %w", err)
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (s *pgWorkflowStore) DeleteAllByTenant(ctx context.Context, tenantID string) (int64, error) {
	var total int64
	err := withTenantTx(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		for _, table := range []string{"workflow_step_runs", "workflow_runs", "workflow_versions", "workflow_definitions"} {
			tag, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE tenant_id=$1`, table), tenantID)
			if err != nil {
				return fmt.Errorf("pg delete %s: %w", table, err)
			}
			total += tag.RowsAffected()
		}
		return nil
	})
	return total, err
}

func jsonOrEmpty(v string) string {
	if strings.TrimSpace(v) == "" {
		return "{}"
	}
	return v
}

var _ store.WorkflowStore = (*pgWorkflowStore)(nil)
