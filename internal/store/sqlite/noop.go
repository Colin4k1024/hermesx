package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

var errSQLiteUnsupported = fmt.Errorf("not supported in SQLite mode — use PostgreSQL for SaaS features")

// --- TenantStore no-op ---

type noopTenantStore struct{}

func (n *noopTenantStore) Create(_ context.Context, _ *store.Tenant) error {
	return errSQLiteUnsupported
}
func (n *noopTenantStore) Get(_ context.Context, _ string) (*store.Tenant, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopTenantStore) Update(_ context.Context, _ *store.Tenant) error {
	return errSQLiteUnsupported
}
func (n *noopTenantStore) Delete(_ context.Context, _ string) error { return errSQLiteUnsupported }
func (n *noopTenantStore) List(_ context.Context, _ store.ListOptions) ([]*store.Tenant, int, error) {
	return nil, 0, errSQLiteUnsupported
}
func (n *noopTenantStore) ListDeleted(_ context.Context, _ time.Time) ([]*store.Tenant, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopTenantStore) HardDelete(_ context.Context, _ string) error {
	return errSQLiteUnsupported
}
func (n *noopTenantStore) Restore(_ context.Context, _ string) error { return errSQLiteUnsupported }

var _ store.TenantStore = (*noopTenantStore)(nil)

// --- AuditLogStore no-op ---

type noopAuditLogStore struct{}

func (n *noopAuditLogStore) Append(_ context.Context, _ *store.AuditLog) error {
	return errSQLiteUnsupported
}
func (n *noopAuditLogStore) List(_ context.Context, _ string, _ store.AuditListOptions) ([]*store.AuditLog, int, error) {
	return nil, 0, errSQLiteUnsupported
}
func (n *noopAuditLogStore) DeleteByTenant(_ context.Context, _ string) (int64, error) {
	return 0, errSQLiteUnsupported
}
func (n *noopAuditLogStore) ArchiveOlderThan(_ context.Context, _ time.Time, _ int) ([]*store.AuditLog, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopAuditLogStore) ArchiveCount(_ context.Context, _ time.Time) (int64, error) {
	return 0, errSQLiteUnsupported
}

var _ store.AuditLogStore = (*noopAuditLogStore)(nil)

// --- APIKeyStore no-op ---

type noopAPIKeyStore struct{}

func (n *noopAPIKeyStore) Create(_ context.Context, _ *store.APIKey) error {
	return errSQLiteUnsupported
}
func (n *noopAPIKeyStore) GetByHash(_ context.Context, _ string) (*store.APIKey, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopAPIKeyStore) GetByID(_ context.Context, _, _ string) (*store.APIKey, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopAPIKeyStore) List(_ context.Context, _ string) ([]*store.APIKey, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopAPIKeyStore) Revoke(_ context.Context, _, _ string) error { return errSQLiteUnsupported }

var _ store.APIKeyStore = (*noopAPIKeyStore)(nil)

// --- MemoryStore no-op ---

type noopMemoryStore struct{}

func (n *noopMemoryStore) Get(_ context.Context, _, _, _ string) (string, error) {
	return "", errSQLiteUnsupported
}
func (n *noopMemoryStore) List(_ context.Context, _, _ string) ([]store.MemoryEntry, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopMemoryStore) Upsert(_ context.Context, _, _, _, _ string) error {
	return errSQLiteUnsupported
}
func (n *noopMemoryStore) Delete(_ context.Context, _, _, _ string) error {
	return errSQLiteUnsupported
}
func (n *noopMemoryStore) DeleteAllByUser(_ context.Context, _, _ string) (int64, error) {
	return 0, errSQLiteUnsupported
}
func (n *noopMemoryStore) DeleteAllByTenant(_ context.Context, _ string) (int64, error) {
	return 0, errSQLiteUnsupported
}

var _ store.MemoryStore = (*noopMemoryStore)(nil)

// --- UserProfileStore no-op ---

type noopUserProfileStore struct{}

func (n *noopUserProfileStore) Get(_ context.Context, _, _ string) (string, error) {
	return "", errSQLiteUnsupported
}
func (n *noopUserProfileStore) Upsert(_ context.Context, _, _, _ string) error {
	return errSQLiteUnsupported
}
func (n *noopUserProfileStore) Delete(_ context.Context, _, _ string) error {
	return errSQLiteUnsupported
}
func (n *noopUserProfileStore) DeleteAllByTenant(_ context.Context, _ string) (int64, error) {
	return 0, errSQLiteUnsupported
}

var _ store.UserProfileStore = (*noopUserProfileStore)(nil)

// --- CronJobStore no-op ---

type noopCronJobStore struct{}

func (n *noopCronJobStore) Create(_ context.Context, _ *store.CronJob) error {
	return errSQLiteUnsupported
}
func (n *noopCronJobStore) Get(_ context.Context, _, _ string) (*store.CronJob, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopCronJobStore) Update(_ context.Context, _ *store.CronJob) error {
	return errSQLiteUnsupported
}
func (n *noopCronJobStore) Delete(_ context.Context, _, _ string) error {
	return errSQLiteUnsupported
}
func (n *noopCronJobStore) List(_ context.Context, _ string) ([]*store.CronJob, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopCronJobStore) ListDue(_ context.Context, _ time.Time) ([]*store.CronJob, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopCronJobStore) ListAllEnabled(_ context.Context) ([]*store.CronJob, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopCronJobStore) ListRuns(_ context.Context, _, _ string, _ int) ([]*store.CronJobRun, error) {
	return nil, errSQLiteUnsupported
}

var _ store.CronJobStore = (*noopCronJobStore)(nil)

// --- RoleStore no-op ---

type noopRoleStore struct{}

func (n *noopRoleStore) Create(_ context.Context, _ *store.Role) error { return errSQLiteUnsupported }
func (n *noopRoleStore) Get(_ context.Context, _, _ string) (*store.Role, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopRoleStore) GetByName(_ context.Context, _, _ string) (*store.Role, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopRoleStore) List(_ context.Context, _ string) ([]*store.Role, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopRoleStore) Delete(_ context.Context, _, _ string) error { return errSQLiteUnsupported }
func (n *noopRoleStore) AddPermission(_ context.Context, _, _, _, _ string) error {
	return errSQLiteUnsupported
}
func (n *noopRoleStore) RemovePermission(_ context.Context, _, _, _, _ string) error {
	return errSQLiteUnsupported
}
func (n *noopRoleStore) ListPermissions(_ context.Context, _, _ string) ([]*store.RolePermission, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopRoleStore) HasPermission(_ context.Context, _ []string, _, _, _ string) (bool, error) {
	return false, errSQLiteUnsupported
}

var _ store.RoleStore = (*noopRoleStore)(nil)

type noopPricingRuleStore struct{}

func (n *noopPricingRuleStore) List(_ context.Context) ([]store.PricingRule, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopPricingRuleStore) Get(_ context.Context, _ string) (*store.PricingRule, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopPricingRuleStore) Upsert(_ context.Context, _ *store.PricingRule) error {
	return errSQLiteUnsupported
}
func (n *noopPricingRuleStore) Delete(_ context.Context, _ string) error {
	return errSQLiteUnsupported
}

var _ store.PricingRuleStore = (*noopPricingRuleStore)(nil)

// --- ExecutionReceipt noop ---

type noopExecutionReceiptStore struct{}

func (n *noopExecutionReceiptStore) Create(_ context.Context, _ *store.ExecutionReceipt) error {
	return errSQLiteUnsupported
}
func (n *noopExecutionReceiptStore) Get(_ context.Context, _, _ string) (*store.ExecutionReceipt, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopExecutionReceiptStore) List(_ context.Context, _ string, _ store.ReceiptListOptions) ([]*store.ExecutionReceipt, int, error) {
	return nil, 0, errSQLiteUnsupported
}
func (n *noopExecutionReceiptStore) GetByIdempotencyID(_ context.Context, _, _ string) (*store.ExecutionReceipt, error) {
	return nil, errSQLiteUnsupported
}

var _ store.ExecutionReceiptStore = (*noopExecutionReceiptStore)(nil)

// --- WorkflowStore noop ---

type noopWorkflowStore struct{}

func (n *noopWorkflowStore) CreateDefinition(_ context.Context, _ *store.WorkflowDefinition) error {
	return errSQLiteUnsupported
}
func (n *noopWorkflowStore) UpdateDefinition(_ context.Context, _ *store.WorkflowDefinition) error {
	return errSQLiteUnsupported
}
func (n *noopWorkflowStore) GetDefinition(_ context.Context, _, _ string) (*store.WorkflowDefinition, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopWorkflowStore) ListDefinitions(_ context.Context, _ string) ([]*store.WorkflowDefinition, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopWorkflowStore) CreateVersion(_ context.Context, _ *store.WorkflowVersion) error {
	return errSQLiteUnsupported
}
func (n *noopWorkflowStore) GetVersion(_ context.Context, _, _ string) (*store.WorkflowVersion, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopWorkflowStore) GetLatestVersion(_ context.Context, _, _ string) (*store.WorkflowVersion, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopWorkflowStore) CreateRun(_ context.Context, _ *store.WorkflowRun, _ []*store.WorkflowStepRun) error {
	return errSQLiteUnsupported
}
func (n *noopWorkflowStore) GetRun(_ context.Context, _, _ string) (*store.WorkflowRun, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopWorkflowStore) ListRuns(_ context.Context, _ string, _ store.WorkflowRunListOptions) ([]*store.WorkflowRun, int, error) {
	return nil, 0, errSQLiteUnsupported
}
func (n *noopWorkflowStore) UpdateRun(_ context.Context, _ *store.WorkflowRun) error {
	return errSQLiteUnsupported
}
func (n *noopWorkflowStore) GetStepRun(_ context.Context, _, _ string) (*store.WorkflowStepRun, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopWorkflowStore) ListStepRuns(_ context.Context, _, _ string) ([]*store.WorkflowStepRun, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopWorkflowStore) UpdateStepRun(_ context.Context, _ *store.WorkflowStepRun) error {
	return errSQLiteUnsupported
}
func (n *noopWorkflowStore) ListPendingHumanTasks(_ context.Context, _, _ string, _ []string) ([]*store.WorkflowStepRun, error) {
	return nil, errSQLiteUnsupported
}
func (n *noopWorkflowStore) DeleteAllByTenant(_ context.Context, _ string) (int64, error) {
	return 0, errSQLiteUnsupported
}

var _ store.WorkflowStore = (*noopWorkflowStore)(nil)
