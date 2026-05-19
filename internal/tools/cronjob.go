package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/config"
	"github.com/Colin4k1024/hermesx/internal/store"
	"github.com/google/uuid"
)

// localCronJob is the file-backed model used by CLI/standalone mode.
type localCronJob struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Schedule  string     `json:"schedule"`
	Command   string     `json:"command"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	LastRun   *time.Time `json:"last_run,omitempty"`
	RunCount  int        `json:"run_count"`
}

func init() {
	Register(&ToolEntry{
		Name:    "cronjob",
		Toolset: "cronjob",
		Schema: map[string]any{
			"name":        "cronjob",
			"description": "Manage scheduled tasks (cron jobs). Create, list, update, or delete recurring jobs.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type":        "string",
						"description": "Action to perform",
						"enum":        []string{"create", "list", "get", "update", "delete", "enable", "disable", "list_runs"},
					},
					"id": map[string]any{
						"type":        "string",
						"description": "Job ID (for get, update, delete, enable, disable)",
					},
					"name": map[string]any{
						"type":        "string",
						"description": "Human-readable job name",
					},
					"schedule": map[string]any{
						"type":        "string",
						"description": "Cron expression (e.g., '0 9 * * *' for daily at 9am, '*/5 * * * *' for every 5 min)",
					},
					"command": map[string]any{
						"type":        "string",
						"description": "Command or task to execute on schedule",
					},
				},
				"required": []string{"action"},
			},
		},
		Handler: handleCronjob,
		Emoji:   "⏰",
	})
}

func handleCronjob(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	action, _ := args["action"].(string)

	// SaaS mode: delegate to PG-backed store.
	if tctx.CronJobStore != nil {
		return handleCronjobSaaS(ctx, action, args, tctx)
	}

	// CLI mode: original file-system implementation.
	switch action {
	case "create":
		return createCronJob(args)
	case "list":
		return listCronJobsAction()
	case "get":
		return getCronJobAction(args)
	case "update":
		return updateCronJob(args)
	case "delete":
		return deleteCronJob(args)
	case "enable":
		return toggleCronJob(args, true)
	case "disable":
		return toggleCronJob(args, false)
	default:
		return `{"error":"Invalid action. Use: create, list, get, update, delete, enable, disable"}`
	}
}

// ── SaaS path ────────────────────────────────────────────────────────────────

func handleCronjobSaaS(ctx context.Context, action string, args map[string]any, tctx *ToolContext) string {
	switch action {
	case "create":
		return saasCreateCronJob(ctx, args, tctx)
	case "list":
		return saasListCronJobs(ctx, tctx)
	case "get":
		return saasGetCronJob(ctx, args, tctx)
	case "update":
		return saasUpdateCronJob(ctx, args, tctx)
	case "delete":
		return saasDeleteCronJob(ctx, args, tctx)
	case "enable":
		return saasToggleCronJob(ctx, args, tctx, true)
	case "disable":
		return saasToggleCronJob(ctx, args, tctx, false)
	case "list_runs":
		return saasListRuns(ctx, args, tctx)
	default:
		return `{"error":"Invalid action. Use: create, list, get, update, delete, enable, disable, list_runs"}`
	}
}

func saasCreateCronJob(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	name, _ := args["name"].(string)
	schedule, _ := args["schedule"].(string)
	command, _ := args["command"].(string)

	if name == "" || schedule == "" || command == "" {
		return `{"error":"name, schedule, and command are required for create"}`
	}

	deliver := "local"
	if tctx.Platform != "" && tctx.Platform != "cron" {
		deliver = tctx.Platform
	}

	// Capture source chat ID from Extra context (set by gateway adapters).
	sourceChatID := ""
	if tctx.Extra != nil {
		if cid, ok := tctx.Extra["chat_id"].(string); ok {
			sourceChatID = cid
		}
	}

	now := time.Now()
	job := &store.CronJob{
		ID:             uuid.New().String(),
		TenantID:       tctx.TenantID,
		Name:           name,
		Prompt:         command,
		Schedule:       schedule,
		Deliver:        deliver,
		Enabled:        true,
		CreatedAt:      now,
		NextRunAt:      &now,
		SourcePlatform: tctx.Platform,
		SourceChatID:   sourceChatID,
	}

	if err := tctx.CronJobStore.Create(ctx, job); err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("failed to create job: %v", err)})
	}

	return toJSON(map[string]any{
		"success": true,
		"job":     cronJobToMap(job),
		"message": fmt.Sprintf("Cron job '%s' created with ID %s. Results will be delivered to %s.", name, job.ID, deliver),
	})
}

func saasListCronJobs(ctx context.Context, tctx *ToolContext) string {
	jobs, err := tctx.CronJobStore.List(ctx, tctx.TenantID)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("failed to list jobs: %v", err)})
	}

	result := make([]map[string]any, 0, len(jobs))
	for _, j := range jobs {
		result = append(result, cronJobToMap(j))
	}

	return toJSON(map[string]any{"jobs": result, "count": len(result)})
}

func saasGetCronJob(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	id, _ := args["id"].(string)
	if id == "" {
		return `{"error":"id is required for get"}`
	}

	job, err := tctx.CronJobStore.Get(ctx, tctx.TenantID, id)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("job not found: %s", id)})
	}

	return toJSON(map[string]any{"job": cronJobToMap(job)})
}

func saasUpdateCronJob(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	id, _ := args["id"].(string)
	if id == "" {
		return `{"error":"id is required for update"}`
	}

	job, err := tctx.CronJobStore.Get(ctx, tctx.TenantID, id)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("job not found: %s", id)})
	}

	if name, ok := args["name"].(string); ok && name != "" {
		job.Name = name
	}
	if schedule, ok := args["schedule"].(string); ok && schedule != "" {
		job.Schedule = schedule
	}
	if command, ok := args["command"].(string); ok && command != "" {
		job.Prompt = command
	}

	if err := tctx.CronJobStore.Update(ctx, job); err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("failed to update job: %v", err)})
	}

	return toJSON(map[string]any{
		"success": true,
		"job":     cronJobToMap(job),
		"message": fmt.Sprintf("Job '%s' updated", job.Name),
	})
}

func saasDeleteCronJob(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	id, _ := args["id"].(string)
	if id == "" {
		return `{"error":"id is required for delete"}`
	}

	if err := tctx.CronJobStore.Delete(ctx, tctx.TenantID, id); err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("failed to delete job: %v", err)})
	}

	return toJSON(map[string]any{
		"success": true,
		"id":      id,
		"message": fmt.Sprintf("Job %s deleted", id),
	})
}

func saasToggleCronJob(ctx context.Context, args map[string]any, tctx *ToolContext, enabled bool) string {
	id, _ := args["id"].(string)
	if id == "" {
		return `{"error":"id is required"}`
	}

	job, err := tctx.CronJobStore.Get(ctx, tctx.TenantID, id)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("job not found: %s", id)})
	}

	job.Enabled = enabled
	if err := tctx.CronJobStore.Update(ctx, job); err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("failed to update job: %v", err)})
	}

	status := "disabled"
	if enabled {
		status = "enabled"
	}

	return toJSON(map[string]any{
		"success": true,
		"job":     cronJobToMap(job),
		"message": fmt.Sprintf("Job '%s' %s", job.Name, status),
	})
}

func saasListRuns(ctx context.Context, args map[string]any, tctx *ToolContext) string {
	id, _ := args["id"].(string)
	if id == "" {
		return `{"error":"id is required for list_runs"}`
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	runs, err := tctx.CronJobStore.ListRuns(ctx, tctx.TenantID, id, limit)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("failed to list runs: %v", err)})
	}

	result := make([]map[string]any, 0, len(runs))
	for _, r := range runs {
		m := map[string]any{
			"id":           r.ID,
			"status":       r.Status,
			"scheduled_at": r.ScheduledAt,
			"started_at":   r.StartedAt,
			"duration_ms":  r.DurationMs,
		}
		if r.FinishedAt != nil {
			m["finished_at"] = r.FinishedAt
		}
		if r.Result != "" {
			m["result"] = r.Result
		}
		if r.Error != "" {
			m["error"] = r.Error
		}
		result = append(result, m)
	}

	return toJSON(map[string]any{"runs": result, "count": len(result), "job_id": id})
}

func cronJobToMap(j *store.CronJob) map[string]any {
	m := map[string]any{
		"id":         j.ID,
		"name":       j.Name,
		"schedule":   j.Schedule,
		"command":    j.Prompt,
		"enabled":    j.Enabled,
		"created_at": j.CreatedAt,
		"run_count":  j.RunCount,
	}
	if j.LastRunAt != nil {
		m["last_run"] = j.LastRunAt
	}
	if j.NextRunAt != nil {
		m["next_run_at"] = j.NextRunAt
	}
	return m
}

// ── CLI / file-system path (unchanged) ───────────────────────────────────────

func getCronDir() string {
	return filepath.Join(config.HermesHome(), "cron")
}

func getCronJobPath(id string) string {
	return filepath.Join(getCronDir(), id+".json")
}

func loadCronJob(id string) (*localCronJob, error) {
	data, err := os.ReadFile(getCronJobPath(id))
	if err != nil {
		return nil, err
	}
	var job localCronJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

func saveCronJob(job *localCronJob) error {
	cronDir := getCronDir()
	os.MkdirAll(cronDir, 0755)

	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getCronJobPath(job.ID), data, 0644)
}

func listCronJobs() ([]localCronJob, error) {
	cronDir := getCronDir()
	os.MkdirAll(cronDir, 0755)

	entries, err := os.ReadDir(cronDir)
	if err != nil {
		return nil, err
	}

	var jobs []localCronJob
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(cronDir, entry.Name()))
		if err != nil {
			continue
		}
		var job localCronJob
		if err := json.Unmarshal(data, &job); err != nil {
			continue
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func createCronJob(args map[string]any) string {
	name, _ := args["name"].(string)
	schedule, _ := args["schedule"].(string)
	command, _ := args["command"].(string)

	if name == "" || schedule == "" || command == "" {
		return `{"error":"name, schedule, and command are required for create"}`
	}

	id := fmt.Sprintf("job_%d", time.Now().UnixMilli())
	now := time.Now()

	job := &localCronJob{
		ID:        id,
		Name:      name,
		Schedule:  schedule,
		Command:   command,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := saveCronJob(job); err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Failed to save job: %v", err)})
	}

	return toJSON(map[string]any{
		"success": true,
		"job":     job,
		"message": fmt.Sprintf("Cron job '%s' created with ID %s", name, id),
	})
}

func listCronJobsAction() string {
	jobs, err := listCronJobs()
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Failed to list jobs: %v", err)})
	}

	if jobs == nil {
		jobs = []localCronJob{}
	}

	return toJSON(map[string]any{
		"jobs":  jobs,
		"count": len(jobs),
	})
}

func getCronJobAction(args map[string]any) string {
	id, _ := args["id"].(string)
	if id == "" {
		return `{"error":"id is required for get"}`
	}

	job, err := loadCronJob(id)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Job not found: %s", id)})
	}

	return toJSON(map[string]any{"job": job})
}

func updateCronJob(args map[string]any) string {
	id, _ := args["id"].(string)
	if id == "" {
		return `{"error":"id is required for update"}`
	}

	job, err := loadCronJob(id)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Job not found: %s", id)})
	}

	if name, ok := args["name"].(string); ok && name != "" {
		job.Name = name
	}
	if schedule, ok := args["schedule"].(string); ok && schedule != "" {
		job.Schedule = schedule
	}
	if command, ok := args["command"].(string); ok && command != "" {
		job.Command = command
	}
	job.UpdatedAt = time.Now()

	if err := saveCronJob(job); err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Failed to update job: %v", err)})
	}

	return toJSON(map[string]any{
		"success": true,
		"job":     job,
		"message": fmt.Sprintf("Job '%s' updated", job.Name),
	})
}

func deleteCronJob(args map[string]any) string {
	id, _ := args["id"].(string)
	if id == "" {
		return `{"error":"id is required for delete"}`
	}

	path := getCronJobPath(id)
	if !fileExists(path) {
		return toJSON(map[string]any{"error": fmt.Sprintf("Job not found: %s", id)})
	}

	if err := os.Remove(path); err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Failed to delete job: %v", err)})
	}

	return toJSON(map[string]any{
		"success": true,
		"id":      id,
		"message": fmt.Sprintf("Job %s deleted", id),
	})
}

func toggleCronJob(args map[string]any, enabled bool) string {
	id, _ := args["id"].(string)
	if id == "" {
		return `{"error":"id is required"}`
	}

	job, err := loadCronJob(id)
	if err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Job not found: %s", id)})
	}

	job.Enabled = enabled
	job.UpdatedAt = time.Now()

	if err := saveCronJob(job); err != nil {
		return toJSON(map[string]any{"error": fmt.Sprintf("Failed to update job: %v", err)})
	}

	status := "disabled"
	if enabled {
		status = "enabled"
	}

	return toJSON(map[string]any{
		"success": true,
		"job":     job,
		"message": fmt.Sprintf("Job '%s' %s", job.Name, status),
	})
}
