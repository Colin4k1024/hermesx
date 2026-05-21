package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound indicates the requested resource does not exist.
var ErrNotFound = errors.New("not found")

// Store is the unified state persistence interface.
// Implementations: pg.PGStore (production), sqlite.SQLiteStore (local dev).
type Store interface {
	Sessions() SessionStore
	Messages() MessageStore
	Users() UserStore
	Tenants() TenantStore
	AuditLogs() AuditLogStore
	APIKeys() APIKeyStore
	Memories() MemoryStore
	UserProfiles() UserProfileStore
	CronJobs() CronJobStore
	Roles() RoleStore
	PricingRules() PricingRuleStore
	ExecutionReceipts() ExecutionReceiptStore
	Workflows() WorkflowStore
	Close() error
	Migrate(ctx context.Context) error
}

// SessionStore manages conversation sessions.
type SessionStore interface {
	Create(ctx context.Context, tenantID string, s *Session) error
	Get(ctx context.Context, tenantID, sessionID string) (*Session, error)
	End(ctx context.Context, tenantID, sessionID, reason string) error
	List(ctx context.Context, tenantID string, opts ListOptions) ([]*Session, int, error)
	Delete(ctx context.Context, tenantID, sessionID string) error
	UpdateTokens(ctx context.Context, tenantID, sessionID string, delta TokenDelta) error
	SetTitle(ctx context.Context, tenantID, sessionID, title string) error
}

// MessageStore manages conversation messages.
type MessageStore interface {
	Append(ctx context.Context, tenantID, sessionID string, msg *Message) (int64, error)
	List(ctx context.Context, tenantID, sessionID string, limit, offset int) ([]*Message, error)
	Search(ctx context.Context, tenantID, query string, limit int) ([]*SearchResult, error)
	CountBySession(ctx context.Context, tenantID, sessionID string) (int, error)
}

// AgentCheckpointStore manages Eino ADK checkpoint payloads.
type AgentCheckpointStore interface {
	Get(ctx context.Context, tenantID, sessionID, checkpointID string) (*AgentCheckpoint, error)
	Set(ctx context.Context, checkpoint *AgentCheckpoint) error
	Delete(ctx context.Context, tenantID, sessionID, checkpointID string) error
}

// UserStore manages user accounts and permissions.
type UserStore interface {
	GetOrCreate(ctx context.Context, tenantID, externalID, username string) (*User, error)
	IsApproved(ctx context.Context, tenantID, platform, userID string) (bool, error)
	Approve(ctx context.Context, tenantID, platform, userID string) error
	Revoke(ctx context.Context, tenantID, platform, userID string) error
	ListApproved(ctx context.Context, tenantID, platform string) ([]string, error)
}

// TenantStore manages tenant CRUD operations.
type TenantStore interface {
	Create(ctx context.Context, t *Tenant) error
	Get(ctx context.Context, id string) (*Tenant, error)
	Update(ctx context.Context, t *Tenant) error
	Delete(ctx context.Context, id string) error // soft delete (sets deleted_at)
	List(ctx context.Context, opts ListOptions) ([]*Tenant, int, error)
	ListDeleted(ctx context.Context, olderThan time.Time) ([]*Tenant, error)
	HardDelete(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) error
}

// AuditLogStore manages append-only audit trail.
type AuditLogStore interface {
	Append(ctx context.Context, log *AuditLog) error
	List(ctx context.Context, tenantID string, opts AuditListOptions) ([]*AuditLog, int, error)
	DeleteByTenant(ctx context.Context, tenantID string) (int64, error)
}

// AuditListOptions controls pagination and filtering for audit log queries.
type AuditListOptions struct {
	Action string
	From   *time.Time // inclusive lower bound on created_at
	To     *time.Time // exclusive upper bound on created_at
	Limit  int
	Offset int
}

// APIKeyStore manages hashed API key lifecycle.
type APIKeyStore interface {
	Create(ctx context.Context, key *APIKey) error
	GetByHash(ctx context.Context, hash string) (*APIKey, error)
	GetByID(ctx context.Context, tenantID, id string) (*APIKey, error)
	List(ctx context.Context, tenantID string) ([]*APIKey, error)
	Revoke(ctx context.Context, tenantID, id string) error
}

// BootstrapAdminKeyCreator is an optional extension for stores that can create
// the first platform admin key atomically across multiple API replicas.
type BootstrapAdminKeyCreator interface {
	CreateBootstrapAdminKey(ctx context.Context, key *APIKey) (created bool, err error)
}

// MemoryStore manages per-user memory key-value pairs.
type MemoryStore interface {
	Get(ctx context.Context, tenantID, userID, key string) (string, error)
	List(ctx context.Context, tenantID, userID string) ([]MemoryEntry, error)
	Upsert(ctx context.Context, tenantID, userID, key, content string) error
	Delete(ctx context.Context, tenantID, userID, key string) error
	DeleteAllByUser(ctx context.Context, tenantID, userID string) (int64, error)
	DeleteAllByTenant(ctx context.Context, tenantID string) (int64, error)
}

// MemorySearcher is an optional extension for vector-based memory retrieval.
// Implementations may use pgvector, Qdrant, or similar backends.
type MemorySearcher interface {
	SearchSimilar(ctx context.Context, tenantID, userID, query string, topK int) ([]MemoryEntry, error)
}

// UserProfileStore manages per-user profile content.
type UserProfileStore interface {
	Get(ctx context.Context, tenantID, userID string) (string, error)
	Upsert(ctx context.Context, tenantID, userID, content string) error
	Delete(ctx context.Context, tenantID, userID string) error
	DeleteAllByTenant(ctx context.Context, tenantID string) (int64, error)
}

// RoleStore manages tenant-scoped roles and permissions.
type RoleStore interface {
	Create(ctx context.Context, role *Role) error
	Get(ctx context.Context, tenantID, roleID string) (*Role, error)
	GetByName(ctx context.Context, tenantID, name string) (*Role, error)
	List(ctx context.Context, tenantID string) ([]*Role, error)
	Delete(ctx context.Context, tenantID, roleID string) error
	AddPermission(ctx context.Context, tenantID, roleName, resource, action string) error
	RemovePermission(ctx context.Context, tenantID, roleName, resource, action string) error
	ListPermissions(ctx context.Context, tenantID, roleName string) ([]*RolePermission, error)
	HasPermission(ctx context.Context, roles []string, tenantID, resource, action string) (bool, error)
}

// CronJobStore manages scheduled job persistence.
type CronJobStore interface {
	Create(ctx context.Context, job *CronJob) error
	Get(ctx context.Context, tenantID, jobID string) (*CronJob, error)
	Update(ctx context.Context, job *CronJob) error
	Delete(ctx context.Context, tenantID, jobID string) error
	List(ctx context.Context, tenantID string) ([]*CronJob, error)
	ListDue(ctx context.Context, now time.Time) ([]*CronJob, error)
	// ListAllEnabled returns all enabled jobs across all tenants (cross-tenant scheduler query).
	ListAllEnabled(ctx context.Context) ([]*CronJob, error)
	// ListRuns returns recent execution history for a job (tenant-scoped).
	ListRuns(ctx context.Context, tenantID, jobID string, limit int) ([]*CronJobRun, error)
}

// PricingRuleStore manages per-model pricing configuration.
type PricingRuleStore interface {
	List(ctx context.Context) ([]PricingRule, error)
	Get(ctx context.Context, modelKey string) (*PricingRule, error)
	Upsert(ctx context.Context, rule *PricingRule) error
	Delete(ctx context.Context, modelKey string) error
}

// ExecutionReceiptStore manages auditable tool execution records.
type ExecutionReceiptStore interface {
	Create(ctx context.Context, receipt *ExecutionReceipt) error
	Get(ctx context.Context, tenantID, id string) (*ExecutionReceipt, error)
	List(ctx context.Context, tenantID string, opts ReceiptListOptions) ([]*ExecutionReceipt, int, error)
	GetByIdempotencyID(ctx context.Context, tenantID, idempotencyID string) (*ExecutionReceipt, error)
}

// WorkflowStore manages fixed SOP workflow definitions, immutable versions,
// runtime instances, and per-node execution state.
type WorkflowStore interface {
	CreateDefinition(ctx context.Context, def *WorkflowDefinition) error
	UpdateDefinition(ctx context.Context, def *WorkflowDefinition) error
	GetDefinition(ctx context.Context, tenantID, definitionID string) (*WorkflowDefinition, error)
	ListDefinitions(ctx context.Context, tenantID string) ([]*WorkflowDefinition, error)

	CreateVersion(ctx context.Context, version *WorkflowVersion) error
	GetVersion(ctx context.Context, tenantID, versionID string) (*WorkflowVersion, error)
	GetLatestVersion(ctx context.Context, tenantID, definitionID string) (*WorkflowVersion, error)

	CreateRun(ctx context.Context, run *WorkflowRun, steps []*WorkflowStepRun) error
	GetRun(ctx context.Context, tenantID, runID string) (*WorkflowRun, error)
	ListRuns(ctx context.Context, tenantID string, opts WorkflowRunListOptions) ([]*WorkflowRun, int, error)
	UpdateRun(ctx context.Context, run *WorkflowRun) error

	GetStepRun(ctx context.Context, tenantID, stepRunID string) (*WorkflowStepRun, error)
	ListStepRuns(ctx context.Context, tenantID, runID string) ([]*WorkflowStepRun, error)
	UpdateStepRun(ctx context.Context, step *WorkflowStepRun) error
	ListPendingHumanTasks(ctx context.Context, tenantID, userID string, roles []string) ([]*WorkflowStepRun, error)

	DeleteAllByTenant(ctx context.Context, tenantID string) (int64, error)
}

// ReceiptListOptions controls pagination and filtering for execution receipt queries.
type ReceiptListOptions struct {
	SessionID string
	ToolName  string
	Status    string
	Limit     int
	Offset    int
}

// WorkflowRunListOptions controls pagination and filtering for workflow runs.
type WorkflowRunListOptions struct {
	DefinitionID string
	Status       string
	Limit        int
	Offset       int
}

// ListOptions controls pagination and filtering for list queries.
type ListOptions struct {
	Platform string
	UserID   string
	Limit    int
	Offset   int
}
