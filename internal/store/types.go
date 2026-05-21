package store

import (
	"time"
)

// Session represents a conversation session.
type Session struct {
	ID               string         `json:"id" db:"id"`
	TenantID         string         `json:"tenant_id" db:"tenant_id"`
	Platform         string         `json:"platform" db:"platform"`
	UserID           string         `json:"user_id" db:"user_id"`
	Model            string         `json:"model" db:"model"`
	SystemPrompt     string         `json:"system_prompt,omitempty" db:"system_prompt"`
	ParentSessionID  string         `json:"parent_session_id,omitempty" db:"parent_session_id"`
	Title            string         `json:"title,omitempty" db:"title"`
	StartedAt        time.Time      `json:"started_at" db:"started_at"`
	EndedAt          *time.Time     `json:"ended_at,omitempty" db:"ended_at"`
	EndReason        string         `json:"end_reason,omitempty" db:"end_reason"`
	MessageCount     int            `json:"message_count" db:"message_count"`
	ToolCallCount    int            `json:"tool_call_count" db:"tool_call_count"`
	InputTokens      int            `json:"input_tokens" db:"input_tokens"`
	OutputTokens     int            `json:"output_tokens" db:"output_tokens"`
	CacheReadTokens  int            `json:"cache_read_tokens" db:"cache_read_tokens"`
	CacheWriteTokens int            `json:"cache_write_tokens" db:"cache_write_tokens"`
	EstimatedCostUSD float64        `json:"estimated_cost_usd" db:"estimated_cost_usd"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// Message represents a conversation message.
type Message struct {
	ID            int64     `json:"id" db:"id"`
	TenantID      string    `json:"tenant_id" db:"tenant_id"`
	SessionID     string    `json:"session_id" db:"session_id"`
	Role          string    `json:"role" db:"role"`
	Content       string    `json:"content,omitempty" db:"content"`
	ToolCallID    string    `json:"tool_call_id,omitempty" db:"tool_call_id"`
	ToolCalls     string    `json:"tool_calls,omitempty" db:"tool_calls"` // JSON
	ToolName      string    `json:"tool_name,omitempty" db:"tool_name"`
	Reasoning     string    `json:"reasoning,omitempty" db:"reasoning"`
	Timestamp     time.Time `json:"timestamp" db:"timestamp"`
	TokenCount    int       `json:"token_count,omitempty" db:"token_count"`
	FinishReason  string    `json:"finish_reason,omitempty" db:"finish_reason"`
	AgenticBlocks string    `json:"agentic_blocks,omitempty" db:"agentic_blocks"` // JSON
}

// AgentCheckpoint stores Eino ADK checkpoint payloads for tenant-scoped sessions.
type AgentCheckpoint struct {
	TenantID     string    `json:"tenant_id" db:"tenant_id"`
	SessionID    string    `json:"session_id" db:"session_id"`
	CheckpointID string    `json:"checkpoint_id" db:"checkpoint_id"`
	Payload      []byte    `json:"payload" db:"payload"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// PricingRule defines per-model token pricing stored in the database.
type PricingRule struct {
	ModelKey       string    `json:"model_key" db:"model_key"`
	InputPer1K     float64   `json:"input_per_1k" db:"input_per_1k"`
	OutputPer1K    float64   `json:"output_per_1k" db:"output_per_1k"`
	CacheReadPer1K float64   `json:"cache_read_per_1k" db:"cache_read_per_1k"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// User represents a platform user.
type User struct {
	ID          string         `json:"id" db:"id"`
	TenantID    string         `json:"tenant_id" db:"tenant_id"`
	ExternalID  string         `json:"external_id" db:"external_id"` // platform:user_id
	Username    string         `json:"username,omitempty" db:"username"`
	DisplayName string         `json:"display_name,omitempty" db:"display_name"`
	Role        string         `json:"role" db:"role"` // user / admin
	ApprovedAt  *time.Time     `json:"approved_at,omitempty" db:"approved_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// TokenDelta represents incremental token count updates.
type TokenDelta struct {
	Input      int
	Output     int
	CacheRead  int
	CacheWrite int
	Reasoning  int
}

// SearchResult represents a full-text search match.
type SearchResult struct {
	SessionID string  `json:"session_id"`
	MessageID int64   `json:"message_id"`
	Content   string  `json:"content"`
	Snippet   string  `json:"snippet"`
	Rank      float64 `json:"rank"`
}

// Tenant represents a SaaS tenant.
// SandboxPolicy defines per-tenant sandbox execution constraints.
type SandboxPolicy struct {
	Enabled         bool     `json:"enabled"`
	MaxTimeout      int      `json:"max_timeout_seconds"`
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	AllowDocker     bool     `json:"allow_docker"`
	RestrictNetwork bool     `json:"restrict_network"`
	MaxStdoutKB     int      `json:"max_stdout_kb"`
}

type Tenant struct {
	ID            string         `json:"id" db:"id"`
	Name          string         `json:"name" db:"name"`
	Plan          string         `json:"plan" db:"plan"` // free / pro / enterprise
	RateLimitRPM  int            `json:"rate_limit_rpm" db:"rate_limit_rpm"`
	MaxSessions   int            `json:"max_sessions" db:"max_sessions"`
	SandboxPolicy *SandboxPolicy `json:"sandbox_policy,omitempty" db:"sandbox_policy"`
	CreatedAt     time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at" db:"updated_at"`
	DeletedAt     *time.Time     `json:"deleted_at,omitempty" db:"deleted_at"`
}

// AuditLog represents an immutable audit trail entry.
type AuditLog struct {
	ID         int64     `json:"id" db:"id"`
	TenantID   string    `json:"tenant_id" db:"tenant_id"`
	UserID     string    `json:"user_id,omitempty" db:"user_id"`
	SessionID  string    `json:"session_id,omitempty" db:"session_id"`
	Action     string    `json:"action" db:"action"`
	Detail     string    `json:"detail,omitempty" db:"detail"`
	RequestID  string    `json:"request_id,omitempty" db:"request_id"`
	StatusCode int       `json:"status_code,omitempty" db:"status_code"`
	LatencyMs  int       `json:"latency_ms,omitempty" db:"latency_ms"`
	SourceIP   string    `json:"source_ip,omitempty" db:"source_ip"`
	ErrorCode  string    `json:"error_code,omitempty" db:"error_code"`
	UserAgent  string    `json:"user_agent,omitempty" db:"user_agent"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// MemoryEntry represents a per-user memory key-value pair.
type MemoryEntry struct {
	TenantID  string    `json:"tenant_id" db:"tenant_id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Key       string    `json:"key" db:"key"`
	Content   string    `json:"content" db:"content"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// UserProfile represents per-user profile content.
type UserProfile struct {
	TenantID  string    `json:"tenant_id" db:"tenant_id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Content   string    `json:"content" db:"content"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CronJob represents a scheduled job.
type CronJob struct {
	ID             string     `json:"id" db:"id"`
	TenantID       string     `json:"tenant_id" db:"tenant_id"`
	Name           string     `json:"name" db:"name"`
	Prompt         string     `json:"prompt" db:"prompt"`
	Schedule       string     `json:"schedule" db:"schedule"`
	Deliver        string     `json:"deliver,omitempty" db:"deliver"`
	Enabled        bool       `json:"enabled" db:"enabled"`
	Model          string     `json:"model,omitempty" db:"model"`
	NextRunAt      *time.Time `json:"next_run_at,omitempty" db:"next_run_at"`
	LastRunAt      *time.Time `json:"last_run_at,omitempty" db:"last_run_at"`
	RunCount       int        `json:"run_count" db:"run_count"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	Metadata       string     `json:"metadata,omitempty" db:"metadata"`
	SourcePlatform string     `json:"source_platform,omitempty" db:"source_platform"`
	SourceChatID   string     `json:"source_chat_id,omitempty" db:"source_chat_id"`
}

// CronJobRun represents a single execution record.
type CronJobRun struct {
	ID          string     `json:"id"`
	CronJobID   string     `json:"cron_job_id"`
	TenantID    string     `json:"tenant_id"`
	Status      string     `json:"status"`
	ScheduledAt time.Time  `json:"scheduled_at"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	DurationMs  *int       `json:"duration_ms,omitempty"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	PodID       string     `json:"pod_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Role represents a named role within a tenant.
type Role struct {
	ID          string    `json:"id" db:"id"`
	TenantID    string    `json:"tenant_id" db:"tenant_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description,omitempty" db:"description"`
	IsSystem    bool      `json:"is_system" db:"is_system"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// RolePermission represents a resource+action grant on a role.
type RolePermission struct {
	ID        string    `json:"id" db:"id"`
	RoleID    string    `json:"role_id" db:"role_id"`
	Resource  string    `json:"resource" db:"resource"`
	Action    string    `json:"action" db:"action"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ExecutionReceipt represents an auditable record of a single tool execution.
type ExecutionReceipt struct {
	ID            string    `json:"id" db:"id"`
	TenantID      string    `json:"tenant_id" db:"tenant_id"`
	SessionID     string    `json:"session_id" db:"session_id"`
	UserID        string    `json:"user_id" db:"user_id"`
	ToolName      string    `json:"tool_name" db:"tool_name"`
	Input         string    `json:"input" db:"input"`
	Output        string    `json:"output" db:"output"`
	Status        string    `json:"status" db:"status"` // "success", "error", "timeout"
	DurationMs    int       `json:"duration_ms" db:"duration_ms"`
	IdempotencyID string    `json:"idempotency_id,omitempty" db:"idempotency_id"`
	TraceID       string    `json:"trace_id,omitempty" db:"trace_id"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

const (
	WorkflowDefinitionDraft     = "draft"
	WorkflowDefinitionPublished = "published"
	WorkflowDefinitionArchived  = "archived"

	WorkflowNodeStart       = "start"
	WorkflowNodeHumanTask   = "human_task"
	WorkflowNodeServiceTask = "service_task"
	WorkflowNodeAgentTask   = "agent_task"
	WorkflowNodeEnd         = "end"

	WorkflowRunPending   = "pending"
	WorkflowRunRunning   = "running"
	WorkflowRunWaiting   = "waiting"
	WorkflowRunPaused    = "paused"
	WorkflowRunCompleted = "completed"
	WorkflowRunCancelled = "cancelled"

	WorkflowStepPending      = "pending"
	WorkflowStepReady        = "ready"
	WorkflowStepRunning      = "running"
	WorkflowStepWaitingHuman = "waiting_human"
	WorkflowStepSucceeded    = "succeeded"
	WorkflowStepFailed       = "failed"
	WorkflowStepSkipped      = "skipped"
)

// WorkflowDefinition is the mutable authoring record for a fixed SOP.
// Published versions snapshot GraphJSON so running instances never drift.
type WorkflowDefinition struct {
	ID              string    `json:"id" db:"id"`
	TenantID        string    `json:"tenant_id" db:"tenant_id"`
	Name            string    `json:"name" db:"name"`
	Description     string    `json:"description,omitempty" db:"description"`
	Status          string    `json:"status" db:"status"`
	GraphJSON       string    `json:"graph_json" db:"graph_json"`
	LatestVersionID string    `json:"latest_version_id,omitempty" db:"latest_version_id"`
	CreatedBy       string    `json:"created_by,omitempty" db:"created_by"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// WorkflowVersion is an immutable published snapshot of a workflow definition.
type WorkflowVersion struct {
	ID           string    `json:"id" db:"id"`
	TenantID     string    `json:"tenant_id" db:"tenant_id"`
	DefinitionID string    `json:"definition_id" db:"definition_id"`
	Version      int       `json:"version" db:"version"`
	GraphJSON    string    `json:"graph_json" db:"graph_json"`
	PublishedBy  string    `json:"published_by,omitempty" db:"published_by"`
	PublishedAt  time.Time `json:"published_at" db:"published_at"`
}

// WorkflowNode is one vertex in the workflow DAG.
type WorkflowNode struct {
	ID             string         `json:"id"`
	Type           string         `json:"type"`
	Name           string         `json:"name,omitempty"`
	AssigneeUserID string         `json:"assignee_user_id,omitempty"`
	AssigneeRole   string         `json:"assignee_role,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
}

// WorkflowCondition is a deterministic predicate over explicit workflow data.
type WorkflowCondition struct {
	Path  string `json:"path,omitempty"`
	Op    string `json:"op,omitempty"`
	Value any    `json:"value,omitempty"`
}

// WorkflowEdge connects two nodes. Outcome is most useful for human tasks;
// Condition is evaluated against input, variables, and prior step outputs.
type WorkflowEdge struct {
	From      string             `json:"from"`
	To        string             `json:"to"`
	Outcome   string             `json:"outcome,omitempty"`
	Condition *WorkflowCondition `json:"condition,omitempty"`
}

// WorkflowGraph is the JSON authoring contract for a workflow definition.
type WorkflowGraph struct {
	Nodes []WorkflowNode `json:"nodes"`
	Edges []WorkflowEdge `json:"edges"`
}

// WorkflowRun is a tenant-scoped runtime instance pinned to one immutable version.
type WorkflowRun struct {
	ID            string     `json:"id" db:"id"`
	TenantID      string     `json:"tenant_id" db:"tenant_id"`
	DefinitionID  string     `json:"definition_id" db:"definition_id"`
	VersionID     string     `json:"version_id" db:"version_id"`
	Status        string     `json:"status" db:"status"`
	StartedBy     string     `json:"started_by,omitempty" db:"started_by"`
	InputJSON     string     `json:"input_json" db:"input_json"`
	VariablesJSON string     `json:"variables_json" db:"variables_json"`
	Error         string     `json:"error,omitempty" db:"error"`
	StartedAt     time.Time  `json:"started_at" db:"started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// WorkflowStepRun is the per-node runtime state for one workflow instance.
type WorkflowStepRun struct {
	ID             string     `json:"id" db:"id"`
	TenantID       string     `json:"tenant_id" db:"tenant_id"`
	RunID          string     `json:"run_id" db:"run_id"`
	NodeID         string     `json:"node_id" db:"node_id"`
	NodeType       string     `json:"node_type" db:"node_type"`
	Status         string     `json:"status" db:"status"`
	Attempt        int        `json:"attempt" db:"attempt"`
	AssigneeUserID string     `json:"assignee_user_id,omitempty" db:"assignee_user_id"`
	AssigneeRole   string     `json:"assignee_role,omitempty" db:"assignee_role"`
	InputJSON      string     `json:"input_json,omitempty" db:"input_json"`
	OutputJSON     string     `json:"output_json,omitempty" db:"output_json"`
	Outcome        string     `json:"outcome,omitempty" db:"outcome"`
	Error          string     `json:"error,omitempty" db:"error"`
	StartedAt      *time.Time `json:"started_at,omitempty" db:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// HumanTaskOutcome is the request payload used to resolve a waiting human task.
type HumanTaskOutcome struct {
	Outcome   string         `json:"outcome"`
	Output    map[string]any `json:"output,omitempty"`
	Variables map[string]any `json:"variables,omitempty"`
}

// APIKey represents a hashed API key bound to a tenant.
type APIKey struct {
	ID        string     `json:"id" db:"id"`
	TenantID  string     `json:"tenant_id" db:"tenant_id"`
	Name      string     `json:"name" db:"name"`
	KeyHash   string     `json:"-" db:"key_hash"`    // SHA-256 hash, never exposed
	Prefix    string     `json:"prefix" db:"prefix"` // first 8 chars for identification
	Roles     []string   `json:"roles" db:"roles"`
	Scopes    []string   `json:"scopes" db:"scopes"` // fine-grained scopes; empty = legacy (role-only)
	ExpiresAt *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}
