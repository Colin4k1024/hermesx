package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errMock = errors.New("store mock error")

// ── Mock sub-stores ─────────────────────────────────────────────────────────

type mockSessionStore struct{ err error }

func (m *mockSessionStore) Create(_ context.Context, _ string, _ *Session) error { return m.err }
func (m *mockSessionStore) Get(_ context.Context, _, _ string) (*Session, error) {
	return nil, m.err
}
func (m *mockSessionStore) End(_ context.Context, _, _, _ string) error { return m.err }
func (m *mockSessionStore) List(_ context.Context, _ string, _ ListOptions) ([]*Session, int, error) {
	return nil, 0, m.err
}
func (m *mockSessionStore) Delete(_ context.Context, _, _ string) error { return m.err }
func (m *mockSessionStore) UpdateTokens(_ context.Context, _, _ string, _ TokenDelta) error {
	return m.err
}
func (m *mockSessionStore) SetTitle(_ context.Context, _, _, _ string) error { return m.err }

type mockMessageStore struct{ err error }

func (m *mockMessageStore) Append(_ context.Context, _, _ string, _ *Message) (int64, error) {
	return 0, m.err
}
func (m *mockMessageStore) List(_ context.Context, _, _ string, _, _ int) ([]*Message, error) {
	return nil, m.err
}
func (m *mockMessageStore) Search(_ context.Context, _, _ string, _ int) ([]*SearchResult, error) {
	return nil, m.err
}
func (m *mockMessageStore) CountBySession(_ context.Context, _, _ string) (int, error) {
	return 0, m.err
}

type mockUserStore struct{ err error }

func (m *mockUserStore) GetOrCreate(_ context.Context, _, _, _ string) (*User, error) {
	return nil, m.err
}
func (m *mockUserStore) IsApproved(_ context.Context, _, _, _ string) (bool, error) {
	return false, m.err
}
func (m *mockUserStore) Approve(_ context.Context, _, _, _ string) error { return m.err }
func (m *mockUserStore) Revoke(_ context.Context, _, _, _ string) error  { return m.err }
func (m *mockUserStore) ListApproved(_ context.Context, _, _ string) ([]string, error) {
	return nil, m.err
}
func (m *mockUserStore) CreateWithPassword(_ context.Context, _ *User, _ string) error {
	return m.err
}
func (m *mockUserStore) GetByUsername(_ context.Context, _, _ string) (*User, string, error) {
	return nil, "", m.err
}
func (m *mockUserStore) GetByID(_ context.Context, _, _ string) (*User, error) { return nil, m.err }

type mockTenantStore struct{ err error }

func (m *mockTenantStore) Create(_ context.Context, _ *Tenant) error { return m.err }
func (m *mockTenantStore) Get(_ context.Context, _ string) (*Tenant, error) {
	return nil, m.err
}
func (m *mockTenantStore) Update(_ context.Context, _ *Tenant) error { return m.err }
func (m *mockTenantStore) Delete(_ context.Context, _ string) error  { return m.err }
func (m *mockTenantStore) List(_ context.Context, _ ListOptions) ([]*Tenant, int, error) {
	return nil, 0, m.err
}
func (m *mockTenantStore) ListDeleted(_ context.Context, _ time.Time) ([]*Tenant, error) {
	return nil, m.err
}
func (m *mockTenantStore) HardDelete(_ context.Context, _ string) error { return m.err }
func (m *mockTenantStore) Restore(_ context.Context, _ string) error    { return m.err }

type mockAuditLogStore struct{ err error }

func (m *mockAuditLogStore) Append(_ context.Context, _ *AuditLog) error { return m.err }
func (m *mockAuditLogStore) List(_ context.Context, _ string, _ AuditListOptions) ([]*AuditLog, int, error) {
	return nil, 0, m.err
}
func (m *mockAuditLogStore) DeleteByTenant(_ context.Context, _ string) (int64, error) {
	return 0, m.err
}
func (m *mockAuditLogStore) ArchiveOlderThan(_ context.Context, _ time.Time, _ int) ([]*AuditLog, error) {
	return nil, m.err
}
func (m *mockAuditLogStore) ArchiveCount(_ context.Context, _ time.Time) (int64, error) {
	return 0, m.err
}

type mockAPIKeyStore struct{ err error }

func (m *mockAPIKeyStore) Create(_ context.Context, _ *APIKey) error { return m.err }
func (m *mockAPIKeyStore) GetByHash(_ context.Context, _ string) (*APIKey, error) {
	return nil, m.err
}
func (m *mockAPIKeyStore) GetByID(_ context.Context, _, _ string) (*APIKey, error) {
	return nil, m.err
}
func (m *mockAPIKeyStore) List(_ context.Context, _ string) ([]*APIKey, error) {
	return nil, m.err
}
func (m *mockAPIKeyStore) Revoke(_ context.Context, _, _ string) error { return m.err }

type mockMemoryStore struct{ err error }

func (m *mockMemoryStore) Get(_ context.Context, _, _, _ string) (string, error) {
	return "", m.err
}
func (m *mockMemoryStore) List(_ context.Context, _, _ string) ([]MemoryEntry, error) {
	return nil, m.err
}
func (m *mockMemoryStore) Upsert(_ context.Context, _, _, _, _ string) error { return m.err }
func (m *mockMemoryStore) Delete(_ context.Context, _, _, _ string) error    { return m.err }
func (m *mockMemoryStore) DeleteAllByUser(_ context.Context, _, _ string) (int64, error) {
	return 0, m.err
}
func (m *mockMemoryStore) DeleteAllByTenant(_ context.Context, _ string) (int64, error) {
	return 0, m.err
}

type mockUserProfileStore struct{ err error }

func (m *mockUserProfileStore) Get(_ context.Context, _, _ string) (string, error) {
	return "", m.err
}
func (m *mockUserProfileStore) Upsert(_ context.Context, _, _, _ string) error { return m.err }
func (m *mockUserProfileStore) Delete(_ context.Context, _, _ string) error    { return m.err }
func (m *mockUserProfileStore) DeleteAllByTenant(_ context.Context, _ string) (int64, error) {
	return 0, m.err
}

type mockCronJobStore struct{ err error }

func (m *mockCronJobStore) Create(_ context.Context, _ *CronJob) error { return m.err }
func (m *mockCronJobStore) Get(_ context.Context, _, _ string) (*CronJob, error) {
	return nil, m.err
}
func (m *mockCronJobStore) Update(_ context.Context, _ *CronJob) error  { return m.err }
func (m *mockCronJobStore) Delete(_ context.Context, _, _ string) error { return m.err }
func (m *mockCronJobStore) List(_ context.Context, _ string) ([]*CronJob, error) {
	return nil, m.err
}
func (m *mockCronJobStore) ListDue(_ context.Context, _ time.Time) ([]*CronJob, error) {
	return nil, m.err
}
func (m *mockCronJobStore) ListAllEnabled(_ context.Context) ([]*CronJob, error) {
	return nil, m.err
}
func (m *mockCronJobStore) ListRuns(_ context.Context, _, _ string, _ int) ([]*CronJobRun, error) {
	return nil, m.err
}

type mockRoleStore struct{ err error }

func (m *mockRoleStore) Create(_ context.Context, _ *Role) error { return m.err }
func (m *mockRoleStore) Get(_ context.Context, _, _ string) (*Role, error) {
	return nil, m.err
}
func (m *mockRoleStore) GetByName(_ context.Context, _, _ string) (*Role, error) {
	return nil, m.err
}
func (m *mockRoleStore) List(_ context.Context, _ string) ([]*Role, error) {
	return nil, m.err
}
func (m *mockRoleStore) Delete(_ context.Context, _, _ string) error                 { return m.err }
func (m *mockRoleStore) AddPermission(_ context.Context, _, _, _, _ string) error    { return m.err }
func (m *mockRoleStore) RemovePermission(_ context.Context, _, _, _, _ string) error { return m.err }
func (m *mockRoleStore) ListPermissions(_ context.Context, _, _ string) ([]*RolePermission, error) {
	return nil, m.err
}
func (m *mockRoleStore) HasPermission(_ context.Context, _ []string, _, _, _ string) (bool, error) {
	return false, m.err
}

type mockPricingRuleStore struct{ err error }

func (m *mockPricingRuleStore) List(_ context.Context) ([]PricingRule, error) {
	return nil, m.err
}
func (m *mockPricingRuleStore) Get(_ context.Context, _ string) (*PricingRule, error) {
	return nil, m.err
}
func (m *mockPricingRuleStore) Upsert(_ context.Context, _ *PricingRule) error { return m.err }
func (m *mockPricingRuleStore) Delete(_ context.Context, _ string) error       { return m.err }

type mockExecutionReceiptStore struct{ err error }

func (m *mockExecutionReceiptStore) Create(_ context.Context, _ *ExecutionReceipt) error {
	return m.err
}
func (m *mockExecutionReceiptStore) Get(_ context.Context, _, _ string) (*ExecutionReceipt, error) {
	return nil, m.err
}
func (m *mockExecutionReceiptStore) List(_ context.Context, _ string, _ ReceiptListOptions) ([]*ExecutionReceipt, int, error) {
	return nil, 0, m.err
}
func (m *mockExecutionReceiptStore) GetByIdempotencyID(_ context.Context, _, _ string) (*ExecutionReceipt, error) {
	return nil, m.err
}

type mockFileEntryStore struct{ err error }

func (m *mockFileEntryStore) List(_ context.Context, _, _ string) ([]*FileEntry, error) {
	return nil, m.err
}
func (m *mockFileEntryStore) Get(_ context.Context, _, _, _ string) (*FileEntry, error) {
	return nil, m.err
}
func (m *mockFileEntryStore) Create(_ context.Context, _ *FileEntry) error   { return m.err }
func (m *mockFileEntryStore) Delete(_ context.Context, _, _, _ string) error { return m.err }
func (m *mockFileEntryStore) GetUserStorageUsage(_ context.Context, _, _ string) (int64, error) {
	return 0, m.err
}

type mockWorkflowStore struct{ err error }

func (m *mockWorkflowStore) CreateDefinition(_ context.Context, _ *WorkflowDefinition) error {
	return m.err
}
func (m *mockWorkflowStore) UpdateDefinition(_ context.Context, _ *WorkflowDefinition) error {
	return m.err
}
func (m *mockWorkflowStore) GetDefinition(_ context.Context, _, _ string) (*WorkflowDefinition, error) {
	return nil, m.err
}
func (m *mockWorkflowStore) ListDefinitions(_ context.Context, _ string) ([]*WorkflowDefinition, error) {
	return nil, m.err
}
func (m *mockWorkflowStore) CreateVersion(_ context.Context, _ *WorkflowVersion) error {
	return m.err
}
func (m *mockWorkflowStore) GetVersion(_ context.Context, _, _ string) (*WorkflowVersion, error) {
	return nil, m.err
}
func (m *mockWorkflowStore) GetLatestVersion(_ context.Context, _, _ string) (*WorkflowVersion, error) {
	return nil, m.err
}
func (m *mockWorkflowStore) CreateRun(_ context.Context, _ *WorkflowRun, _ []*WorkflowStepRun) error {
	return m.err
}
func (m *mockWorkflowStore) GetRun(_ context.Context, _, _ string) (*WorkflowRun, error) {
	return nil, m.err
}
func (m *mockWorkflowStore) ListRuns(_ context.Context, _ string, _ WorkflowRunListOptions) ([]*WorkflowRun, int, error) {
	return nil, 0, m.err
}
func (m *mockWorkflowStore) UpdateRun(_ context.Context, _ *WorkflowRun) error { return m.err }
func (m *mockWorkflowStore) GetStepRun(_ context.Context, _, _ string) (*WorkflowStepRun, error) {
	return nil, m.err
}
func (m *mockWorkflowStore) ListStepRuns(_ context.Context, _, _ string) ([]*WorkflowStepRun, error) {
	return nil, m.err
}
func (m *mockWorkflowStore) UpdateStepRun(_ context.Context, _ *WorkflowStepRun) error {
	return m.err
}
func (m *mockWorkflowStore) ListPendingHumanTasks(_ context.Context, _, _ string, _ []string) ([]*WorkflowStepRun, error) {
	return nil, m.err
}
func (m *mockWorkflowStore) DeleteAllByTenant(_ context.Context, _ string) (int64, error) {
	return 0, m.err
}

type mockAgentProfileStore struct{ err error }

func (m *mockAgentProfileStore) Create(_ context.Context, _ *AgentProfile) error { return m.err }
func (m *mockAgentProfileStore) Get(_ context.Context, _, _, _ string) (*AgentProfile, error) {
	return nil, m.err
}
func (m *mockAgentProfileStore) List(_ context.Context, _, _ string) ([]*AgentProfile, error) {
	return nil, m.err
}
func (m *mockAgentProfileStore) Update(_ context.Context, _ *AgentProfile) error { return m.err }
func (m *mockAgentProfileStore) Delete(_ context.Context, _, _, _ string) error  { return m.err }
func (m *mockAgentProfileStore) GetDefault(_ context.Context, _, _ string) (*AgentProfile, error) {
	return nil, m.err
}
func (m *mockAgentProfileStore) SetDefault(_ context.Context, _, _, _ string) error { return m.err }

// ── Mock composite Store ────────────────────────────────────────────────────

type mockStore struct {
	sessions          *mockSessionStore
	messages          *mockMessageStore
	users             *mockUserStore
	tenants           *mockTenantStore
	auditLogs         *mockAuditLogStore
	apiKeys           *mockAPIKeyStore
	memories          *mockMemoryStore
	userProfiles      *mockUserProfileStore
	cronJobs          *mockCronJobStore
	roles             *mockRoleStore
	pricingRules      *mockPricingRuleStore
	executionReceipts *mockExecutionReceiptStore
	fileEntries       *mockFileEntryStore
	workflows         *mockWorkflowStore
	agentProfiles     *mockAgentProfileStore
}

func (m *mockStore) Sessions() SessionStore                   { return m.sessions }
func (m *mockStore) Messages() MessageStore                   { return m.messages }
func (m *mockStore) Users() UserStore                         { return m.users }
func (m *mockStore) Tenants() TenantStore                     { return m.tenants }
func (m *mockStore) AuditLogs() AuditLogStore                 { return m.auditLogs }
func (m *mockStore) APIKeys() APIKeyStore                     { return m.apiKeys }
func (m *mockStore) Memories() MemoryStore                    { return m.memories }
func (m *mockStore) UserProfiles() UserProfileStore           { return m.userProfiles }
func (m *mockStore) CronJobs() CronJobStore                   { return m.cronJobs }
func (m *mockStore) Roles() RoleStore                         { return m.roles }
func (m *mockStore) PricingRules() PricingRuleStore           { return m.pricingRules }
func (m *mockStore) ExecutionReceipts() ExecutionReceiptStore { return m.executionReceipts }
func (m *mockStore) FileEntries() FileEntryStore              { return m.fileEntries }
func (m *mockStore) Workflows() WorkflowStore                 { return m.workflows }
func (m *mockStore) AgentProfiles() AgentProfileStore         { return m.agentProfiles }
func (m *mockStore) Close() error                             { return nil }
func (m *mockStore) Migrate(_ context.Context) error          { return nil }

func newFullMockStore() *mockStore {
	return &mockStore{
		sessions:          &mockSessionStore{},
		messages:          &mockMessageStore{},
		users:             &mockUserStore{},
		tenants:           &mockTenantStore{},
		auditLogs:         &mockAuditLogStore{},
		apiKeys:           &mockAPIKeyStore{},
		memories:          &mockMemoryStore{},
		userProfiles:      &mockUserProfileStore{},
		cronJobs:          &mockCronJobStore{},
		roles:             &mockRoleStore{},
		pricingRules:      &mockPricingRuleStore{},
		executionReceipts: &mockExecutionReceiptStore{},
		fileEntries:       &mockFileEntryStore{},
		workflows:         &mockWorkflowStore{},
		agentProfiles:     &mockAgentProfileStore{},
	}
}

// ── Tests ───────────────────────────────────────────────────────────────────

func TestNewTracedStore(t *testing.T) {
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	if ts == nil {
		t.Fatal("NewTracedStore returned nil")
	}
}

func TestTracedStore_Delegation(t *testing.T) {
	inner := newFullMockStore()
	ts := NewTracedStore(inner)

	if ts.Sessions() == nil {
		t.Fatal("Sessions() returned nil")
	}
	if ts.Messages() == nil {
		t.Fatal("Messages() returned nil")
	}
	if ts.Users() == nil {
		t.Fatal("Users() returned nil")
	}
	if ts.Tenants() == nil {
		t.Fatal("Tenants() returned nil")
	}
	if ts.AuditLogs() == nil {
		t.Fatal("AuditLogs() returned nil")
	}
	if ts.APIKeys() == nil {
		t.Fatal("APIKeys() returned nil")
	}
	if ts.Memories() == nil {
		t.Fatal("Memories() returned nil")
	}
	if ts.UserProfiles() == nil {
		t.Fatal("UserProfiles() returned nil")
	}
	if ts.CronJobs() == nil {
		t.Fatal("CronJobs() returned nil")
	}
	if ts.Roles() == nil {
		t.Fatal("Roles() returned nil")
	}
	if ts.PricingRules() == nil {
		t.Fatal("PricingRules() returned nil")
	}
	if ts.ExecutionReceipts() == nil {
		t.Fatal("ExecutionReceipts() returned nil")
	}
	if ts.FileEntries() == nil {
		t.Fatal("FileEntries() returned nil")
	}
	if ts.Workflows() == nil {
		t.Fatal("Workflows() returned nil")
	}
	if ts.AgentProfiles() == nil {
		t.Fatal("AgentProfiles() returned nil")
	}
}

func TestTracedStore_CloseAndMigrate(t *testing.T) {
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	ctx := context.Background()

	if err := ts.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := ts.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
}

func TestTracedSessions(t *testing.T) {
	ctx := context.Background()
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	sess := ts.Sessions()

	// ── success path ──
	if err := sess.Create(ctx, "t1", &Session{}); err != nil {
		t.Fatalf("Create success: %v", err)
	}
	if _, err := sess.Get(ctx, "t1", "s1"); err != nil {
		t.Fatalf("Get success: %v", err)
	}
	if err := sess.End(ctx, "t1", "s1", "done"); err != nil {
		t.Fatalf("End success: %v", err)
	}
	if _, _, err := sess.List(ctx, "t1", ListOptions{}); err != nil {
		t.Fatalf("List success: %v", err)
	}
	if err := sess.Delete(ctx, "t1", "s1"); err != nil {
		t.Fatalf("Delete success: %v", err)
	}
	if err := sess.UpdateTokens(ctx, "t1", "s1", TokenDelta{}); err != nil {
		t.Fatalf("UpdateTokens success: %v", err)
	}
	if err := sess.SetTitle(ctx, "t1", "s1", "title"); err != nil {
		t.Fatalf("SetTitle success: %v", err)
	}

	// ── error path ──
	inner.sessions.err = errMock
	if err := sess.Create(ctx, "t1", &Session{}); err == nil {
		t.Fatal("Create should return error")
	}
	if _, err := sess.Get(ctx, "t1", "s1"); err == nil {
		t.Fatal("Get should return error")
	}
	if err := sess.End(ctx, "t1", "s1", "done"); err == nil {
		t.Fatal("End should return error")
	}
	if _, _, err := sess.List(ctx, "t1", ListOptions{}); err == nil {
		t.Fatal("List should return error")
	}
	if err := sess.Delete(ctx, "t1", "s1"); err == nil {
		t.Fatal("Delete should return error")
	}
	if err := sess.UpdateTokens(ctx, "t1", "s1", TokenDelta{}); err == nil {
		t.Fatal("UpdateTokens should return error")
	}
	if err := sess.SetTitle(ctx, "t1", "s1", "title"); err == nil {
		t.Fatal("SetTitle should return error")
	}
}

func TestTracedMessages(t *testing.T) {
	ctx := context.Background()
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	msgs := ts.Messages()

	// ── success path ──
	if _, err := msgs.Append(ctx, "t1", "s1", &Message{Role: "user"}); err != nil {
		t.Fatalf("Append success: %v", err)
	}
	if _, err := msgs.List(ctx, "t1", "s1", 10, 0); err != nil {
		t.Fatalf("List success: %v", err)
	}
	if _, err := msgs.Search(ctx, "t1", "hello", 5); err != nil {
		t.Fatalf("Search success: %v", err)
	}
	if _, err := msgs.CountBySession(ctx, "t1", "s1"); err != nil {
		t.Fatalf("CountBySession success: %v", err)
	}

	// ── error path ──
	inner.messages.err = errMock
	if _, err := msgs.Append(ctx, "t1", "s1", &Message{Role: "user"}); err == nil {
		t.Fatal("Append should return error")
	}
	if _, err := msgs.List(ctx, "t1", "s1", 10, 0); err == nil {
		t.Fatal("List should return error")
	}
	if _, err := msgs.Search(ctx, "t1", "hello", 5); err == nil {
		t.Fatal("Search should return error")
	}
	if _, err := msgs.CountBySession(ctx, "t1", "s1"); err == nil {
		t.Fatal("CountBySession should return error")
	}
}

func TestTracedUsers(t *testing.T) {
	ctx := context.Background()
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	users := ts.Users()

	// ── success path ──
	if _, err := users.GetOrCreate(ctx, "t1", "ext1", "alice"); err != nil {
		t.Fatalf("GetOrCreate success: %v", err)
	}
	if _, err := users.IsApproved(ctx, "t1", "slack", "u1"); err != nil {
		t.Fatalf("IsApproved success: %v", err)
	}
	if err := users.Approve(ctx, "t1", "slack", "u1"); err != nil {
		t.Fatalf("Approve success: %v", err)
	}
	if err := users.Revoke(ctx, "t1", "slack", "u1"); err != nil {
		t.Fatalf("Revoke success: %v", err)
	}
	if _, err := users.ListApproved(ctx, "t1", "slack"); err != nil {
		t.Fatalf("ListApproved success: %v", err)
	}
	if err := users.CreateWithPassword(ctx, &User{TenantID: "t1"}, "hash"); err != nil {
		t.Fatalf("CreateWithPassword success: %v", err)
	}
	if _, _, err := users.GetByUsername(ctx, "t1", "alice"); err != nil {
		t.Fatalf("GetByUsername success: %v", err)
	}
	if _, err := users.GetByID(ctx, "t1", "u1"); err != nil {
		t.Fatalf("GetByID success: %v", err)
	}

	// ── error path ──
	inner.users.err = errMock
	if _, err := users.GetOrCreate(ctx, "t1", "ext1", "alice"); err == nil {
		t.Fatal("GetOrCreate should return error")
	}
	if _, err := users.IsApproved(ctx, "t1", "slack", "u1"); err == nil {
		t.Fatal("IsApproved should return error")
	}
	if err := users.Approve(ctx, "t1", "slack", "u1"); err == nil {
		t.Fatal("Approve should return error")
	}
	if err := users.Revoke(ctx, "t1", "slack", "u1"); err == nil {
		t.Fatal("Revoke should return error")
	}
	if _, err := users.ListApproved(ctx, "t1", "slack"); err == nil {
		t.Fatal("ListApproved should return error")
	}
	if err := users.CreateWithPassword(ctx, &User{TenantID: "t1"}, "hash"); err == nil {
		t.Fatal("CreateWithPassword should return error")
	}
	if _, _, err := users.GetByUsername(ctx, "t1", "alice"); err == nil {
		t.Fatal("GetByUsername should return error")
	}
	if _, err := users.GetByID(ctx, "t1", "u1"); err == nil {
		t.Fatal("GetByID should return error")
	}
}

func TestTracedTenants(t *testing.T) {
	ctx := context.Background()
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	tenants := ts.Tenants()

	// ── success path ──
	if err := tenants.Create(ctx, &Tenant{}); err != nil {
		t.Fatalf("Create success: %v", err)
	}
	if _, err := tenants.Get(ctx, "t1"); err != nil {
		t.Fatalf("Get success: %v", err)
	}
	if err := tenants.Update(ctx, &Tenant{}); err != nil {
		t.Fatalf("Update success: %v", err)
	}
	if err := tenants.Delete(ctx, "t1"); err != nil {
		t.Fatalf("Delete success: %v", err)
	}
	if _, _, err := tenants.List(ctx, ListOptions{}); err != nil {
		t.Fatalf("List success: %v", err)
	}
	if _, err := tenants.ListDeleted(ctx, time.Now()); err != nil {
		t.Fatalf("ListDeleted success: %v", err)
	}
	if err := tenants.HardDelete(ctx, "t1"); err != nil {
		t.Fatalf("HardDelete success: %v", err)
	}
	if err := tenants.Restore(ctx, "t1"); err != nil {
		t.Fatalf("Restore success: %v", err)
	}

	// ── error path ──
	inner.tenants.err = errMock
	if err := tenants.Create(ctx, &Tenant{}); err == nil {
		t.Fatal("Create should return error")
	}
	if _, err := tenants.Get(ctx, "t1"); err == nil {
		t.Fatal("Get should return error")
	}
	if err := tenants.Update(ctx, &Tenant{}); err == nil {
		t.Fatal("Update should return error")
	}
	if err := tenants.Delete(ctx, "t1"); err == nil {
		t.Fatal("Delete should return error")
	}
	if _, _, err := tenants.List(ctx, ListOptions{}); err == nil {
		t.Fatal("List should return error")
	}
	if _, err := tenants.ListDeleted(ctx, time.Now()); err == nil {
		t.Fatal("ListDeleted should return error")
	}
	if err := tenants.HardDelete(ctx, "t1"); err == nil {
		t.Fatal("HardDelete should return error")
	}
	if err := tenants.Restore(ctx, "t1"); err == nil {
		t.Fatal("Restore should return error")
	}
}

func TestTracedAuditLogs(t *testing.T) {
	ctx := context.Background()
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	audit := ts.AuditLogs()

	// ── success path ──
	if err := audit.Append(ctx, &AuditLog{}); err != nil {
		t.Fatalf("Append success: %v", err)
	}
	if _, _, err := audit.List(ctx, "t1", AuditListOptions{}); err != nil {
		t.Fatalf("List success: %v", err)
	}
	if _, err := audit.DeleteByTenant(ctx, "t1"); err != nil {
		t.Fatalf("DeleteByTenant success: %v", err)
	}
	if _, err := audit.ArchiveOlderThan(ctx, time.Now(), 100); err != nil {
		t.Fatalf("ArchiveOlderThan success: %v", err)
	}
	if _, err := audit.ArchiveCount(ctx, time.Now()); err != nil {
		t.Fatalf("ArchiveCount success: %v", err)
	}

	// ── error path ──
	inner.auditLogs.err = errMock
	if err := audit.Append(ctx, &AuditLog{}); err == nil {
		t.Fatal("Append should return error")
	}
	if _, _, err := audit.List(ctx, "t1", AuditListOptions{}); err == nil {
		t.Fatal("List should return error")
	}
	if _, err := audit.DeleteByTenant(ctx, "t1"); err == nil {
		t.Fatal("DeleteByTenant should return error")
	}
	if _, err := audit.ArchiveOlderThan(ctx, time.Now(), 100); err == nil {
		t.Fatal("ArchiveOlderThan should return error")
	}
	if _, err := audit.ArchiveCount(ctx, time.Now()); err == nil {
		t.Fatal("ArchiveCount should return error")
	}
}

func TestTracedAPIKeys(t *testing.T) {
	ctx := context.Background()
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	keys := ts.APIKeys()

	// ── success path ──
	if err := keys.Create(ctx, &APIKey{TenantID: "t1"}); err != nil {
		t.Fatalf("Create success: %v", err)
	}
	if _, err := keys.GetByHash(ctx, "hash1"); err != nil {
		t.Fatalf("GetByHash success: %v", err)
	}
	if _, err := keys.GetByID(ctx, "t1", "k1"); err != nil {
		t.Fatalf("GetByID success: %v", err)
	}
	if _, err := keys.List(ctx, "t1"); err != nil {
		t.Fatalf("List success: %v", err)
	}
	if err := keys.Revoke(ctx, "t1", "k1"); err != nil {
		t.Fatalf("Revoke success: %v", err)
	}

	// ── error path ──
	inner.apiKeys.err = errMock
	if err := keys.Create(ctx, &APIKey{TenantID: "t1"}); err == nil {
		t.Fatal("Create should return error")
	}
	if _, err := keys.GetByHash(ctx, "hash1"); err == nil {
		t.Fatal("GetByHash should return error")
	}
	if _, err := keys.GetByID(ctx, "t1", "k1"); err == nil {
		t.Fatal("GetByID should return error")
	}
	if _, err := keys.List(ctx, "t1"); err == nil {
		t.Fatal("List should return error")
	}
	if err := keys.Revoke(ctx, "t1", "k1"); err == nil {
		t.Fatal("Revoke should return error")
	}
}

func TestTracedMemories(t *testing.T) {
	ctx := context.Background()
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	mem := ts.Memories()

	// ── success path ──
	if _, err := mem.Get(ctx, "t1", "u1", "key1"); err != nil {
		t.Fatalf("Get success: %v", err)
	}
	if _, err := mem.List(ctx, "t1", "u1"); err != nil {
		t.Fatalf("List success: %v", err)
	}
	if err := mem.Upsert(ctx, "t1", "u1", "key1", "val"); err != nil {
		t.Fatalf("Upsert success: %v", err)
	}
	if err := mem.Delete(ctx, "t1", "u1", "key1"); err != nil {
		t.Fatalf("Delete success: %v", err)
	}
	if _, err := mem.DeleteAllByUser(ctx, "t1", "u1"); err != nil {
		t.Fatalf("DeleteAllByUser success: %v", err)
	}
	if _, err := mem.DeleteAllByTenant(ctx, "t1"); err != nil {
		t.Fatalf("DeleteAllByTenant success: %v", err)
	}

	// ── error path ──
	inner.memories.err = errMock
	if _, err := mem.Get(ctx, "t1", "u1", "key1"); err == nil {
		t.Fatal("Get should return error")
	}
	if _, err := mem.List(ctx, "t1", "u1"); err == nil {
		t.Fatal("List should return error")
	}
	if err := mem.Upsert(ctx, "t1", "u1", "key1", "val"); err == nil {
		t.Fatal("Upsert should return error")
	}
	if err := mem.Delete(ctx, "t1", "u1", "key1"); err == nil {
		t.Fatal("Delete should return error")
	}
	if _, err := mem.DeleteAllByUser(ctx, "t1", "u1"); err == nil {
		t.Fatal("DeleteAllByUser should return error")
	}
	if _, err := mem.DeleteAllByTenant(ctx, "t1"); err == nil {
		t.Fatal("DeleteAllByTenant should return error")
	}
}

func TestTracedUserProfiles(t *testing.T) {
	ctx := context.Background()
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	profiles := ts.UserProfiles()

	// ── success path ──
	if _, err := profiles.Get(ctx, "t1", "u1"); err != nil {
		t.Fatalf("Get success: %v", err)
	}
	if err := profiles.Upsert(ctx, "t1", "u1", "content"); err != nil {
		t.Fatalf("Upsert success: %v", err)
	}
	if err := profiles.Delete(ctx, "t1", "u1"); err != nil {
		t.Fatalf("Delete success: %v", err)
	}
	if _, err := profiles.DeleteAllByTenant(ctx, "t1"); err != nil {
		t.Fatalf("DeleteAllByTenant success: %v", err)
	}

	// ── error path ──
	inner.userProfiles.err = errMock
	if _, err := profiles.Get(ctx, "t1", "u1"); err == nil {
		t.Fatal("Get should return error")
	}
	if err := profiles.Upsert(ctx, "t1", "u1", "content"); err == nil {
		t.Fatal("Upsert should return error")
	}
	if err := profiles.Delete(ctx, "t1", "u1"); err == nil {
		t.Fatal("Delete should return error")
	}
	if _, err := profiles.DeleteAllByTenant(ctx, "t1"); err == nil {
		t.Fatal("DeleteAllByTenant should return error")
	}
}

func TestTracedCronJobs(t *testing.T) {
	ctx := context.Background()
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	cron := ts.CronJobs()

	// ── success path ──
	if err := cron.Create(ctx, &CronJob{TenantID: "t1"}); err != nil {
		t.Fatalf("Create success: %v", err)
	}
	if _, err := cron.Get(ctx, "t1", "j1"); err != nil {
		t.Fatalf("Get success: %v", err)
	}
	if err := cron.Update(ctx, &CronJob{TenantID: "t1"}); err != nil {
		t.Fatalf("Update success: %v", err)
	}
	if err := cron.Delete(ctx, "t1", "j1"); err != nil {
		t.Fatalf("Delete success: %v", err)
	}
	if _, err := cron.List(ctx, "t1"); err != nil {
		t.Fatalf("List success: %v", err)
	}
	if _, err := cron.ListDue(ctx, time.Now()); err != nil {
		t.Fatalf("ListDue success: %v", err)
	}
	if _, err := cron.ListAllEnabled(ctx); err != nil {
		t.Fatalf("ListAllEnabled success: %v", err)
	}
	if _, err := cron.ListRuns(ctx, "t1", "j1", 10); err != nil {
		t.Fatalf("ListRuns success: %v", err)
	}

	// ── error path ──
	inner.cronJobs.err = errMock
	if err := cron.Create(ctx, &CronJob{TenantID: "t1"}); err == nil {
		t.Fatal("Create should return error")
	}
	if _, err := cron.Get(ctx, "t1", "j1"); err == nil {
		t.Fatal("Get should return error")
	}
	if err := cron.Update(ctx, &CronJob{TenantID: "t1"}); err == nil {
		t.Fatal("Update should return error")
	}
	if err := cron.Delete(ctx, "t1", "j1"); err == nil {
		t.Fatal("Delete should return error")
	}
	if _, err := cron.List(ctx, "t1"); err == nil {
		t.Fatal("List should return error")
	}
	if _, err := cron.ListDue(ctx, time.Now()); err == nil {
		t.Fatal("ListDue should return error")
	}
	if _, err := cron.ListAllEnabled(ctx); err == nil {
		t.Fatal("ListAllEnabled should return error")
	}
	if _, err := cron.ListRuns(ctx, "t1", "j1", 10); err == nil {
		t.Fatal("ListRuns should return error")
	}
}

func TestTracedRoles(t *testing.T) {
	ctx := context.Background()
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	roles := ts.Roles()

	// ── success path ──
	if err := roles.Create(ctx, &Role{TenantID: "t1"}); err != nil {
		t.Fatalf("Create success: %v", err)
	}
	if _, err := roles.Get(ctx, "t1", "r1"); err != nil {
		t.Fatalf("Get success: %v", err)
	}
	if _, err := roles.GetByName(ctx, "t1", "admin"); err != nil {
		t.Fatalf("GetByName success: %v", err)
	}
	if _, err := roles.List(ctx, "t1"); err != nil {
		t.Fatalf("List success: %v", err)
	}
	if err := roles.Delete(ctx, "t1", "r1"); err != nil {
		t.Fatalf("Delete success: %v", err)
	}
	if err := roles.AddPermission(ctx, "t1", "admin", "sessions", "read"); err != nil {
		t.Fatalf("AddPermission success: %v", err)
	}
	if err := roles.RemovePermission(ctx, "t1", "admin", "sessions", "read"); err != nil {
		t.Fatalf("RemovePermission success: %v", err)
	}
	if _, err := roles.ListPermissions(ctx, "t1", "admin"); err != nil {
		t.Fatalf("ListPermissions success: %v", err)
	}
	if _, err := roles.HasPermission(ctx, []string{"admin"}, "t1", "sessions", "read"); err != nil {
		t.Fatalf("HasPermission success: %v", err)
	}

	// ── error path ──
	inner.roles.err = errMock
	if err := roles.Create(ctx, &Role{TenantID: "t1"}); err == nil {
		t.Fatal("Create should return error")
	}
	if _, err := roles.Get(ctx, "t1", "r1"); err == nil {
		t.Fatal("Get should return error")
	}
	if _, err := roles.GetByName(ctx, "t1", "admin"); err == nil {
		t.Fatal("GetByName should return error")
	}
	if _, err := roles.List(ctx, "t1"); err == nil {
		t.Fatal("List should return error")
	}
	if err := roles.Delete(ctx, "t1", "r1"); err == nil {
		t.Fatal("Delete should return error")
	}
	if err := roles.AddPermission(ctx, "t1", "admin", "sessions", "read"); err == nil {
		t.Fatal("AddPermission should return error")
	}
	if err := roles.RemovePermission(ctx, "t1", "admin", "sessions", "read"); err == nil {
		t.Fatal("RemovePermission should return error")
	}
	if _, err := roles.ListPermissions(ctx, "t1", "admin"); err == nil {
		t.Fatal("ListPermissions should return error")
	}
	if _, err := roles.HasPermission(ctx, []string{"admin"}, "t1", "sessions", "read"); err == nil {
		t.Fatal("HasPermission should return error")
	}
}

func TestTracedPricingRules(t *testing.T) {
	ctx := context.Background()
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	pr := ts.PricingRules()

	// ── success path ──
	if _, err := pr.List(ctx); err != nil {
		t.Fatalf("List success: %v", err)
	}
	if _, err := pr.Get(ctx, "gpt-4"); err != nil {
		t.Fatalf("Get success: %v", err)
	}
	if err := pr.Upsert(ctx, &PricingRule{}); err != nil {
		t.Fatalf("Upsert success: %v", err)
	}
	if err := pr.Delete(ctx, "gpt-4"); err != nil {
		t.Fatalf("Delete success: %v", err)
	}

	// ── error path ──
	inner.pricingRules.err = errMock
	if _, err := pr.List(ctx); err == nil {
		t.Fatal("List should return error")
	}
	if _, err := pr.Get(ctx, "gpt-4"); err == nil {
		t.Fatal("Get should return error")
	}
	if err := pr.Upsert(ctx, &PricingRule{}); err == nil {
		t.Fatal("Upsert should return error")
	}
	if err := pr.Delete(ctx, "gpt-4"); err == nil {
		t.Fatal("Delete should return error")
	}
}

func TestTracedExecutionReceipts(t *testing.T) {
	ctx := context.Background()
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	receipts := ts.ExecutionReceipts()

	// ── success path ──
	if err := receipts.Create(ctx, &ExecutionReceipt{TenantID: "t1"}); err != nil {
		t.Fatalf("Create success: %v", err)
	}
	if _, err := receipts.Get(ctx, "t1", "r1"); err != nil {
		t.Fatalf("Get success: %v", err)
	}
	if _, _, err := receipts.List(ctx, "t1", ReceiptListOptions{}); err != nil {
		t.Fatalf("List success: %v", err)
	}
	if _, err := receipts.GetByIdempotencyID(ctx, "t1", "idem1"); err != nil {
		t.Fatalf("GetByIdempotencyID success: %v", err)
	}

	// ── error path ──
	inner.executionReceipts.err = errMock
	if err := receipts.Create(ctx, &ExecutionReceipt{TenantID: "t1"}); err == nil {
		t.Fatal("Create should return error")
	}
	if _, err := receipts.Get(ctx, "t1", "r1"); err == nil {
		t.Fatal("Get should return error")
	}
	if _, _, err := receipts.List(ctx, "t1", ReceiptListOptions{}); err == nil {
		t.Fatal("List should return error")
	}
	if _, err := receipts.GetByIdempotencyID(ctx, "t1", "idem1"); err == nil {
		t.Fatal("GetByIdempotencyID should return error")
	}
}

func TestTracedWorkflows(t *testing.T) {
	ctx := context.Background()
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	wf := ts.Workflows()

	// ── success path ──
	if err := wf.CreateDefinition(ctx, &WorkflowDefinition{TenantID: "t1"}); err != nil {
		t.Fatalf("CreateDefinition success: %v", err)
	}
	if err := wf.UpdateDefinition(ctx, &WorkflowDefinition{TenantID: "t1"}); err != nil {
		t.Fatalf("UpdateDefinition success: %v", err)
	}
	if _, err := wf.GetDefinition(ctx, "t1", "d1"); err != nil {
		t.Fatalf("GetDefinition success: %v", err)
	}
	if _, err := wf.ListDefinitions(ctx, "t1"); err != nil {
		t.Fatalf("ListDefinitions success: %v", err)
	}
	if err := wf.CreateVersion(ctx, &WorkflowVersion{TenantID: "t1"}); err != nil {
		t.Fatalf("CreateVersion success: %v", err)
	}
	if _, err := wf.GetVersion(ctx, "t1", "v1"); err != nil {
		t.Fatalf("GetVersion success: %v", err)
	}
	if _, err := wf.GetLatestVersion(ctx, "t1", "d1"); err != nil {
		t.Fatalf("GetLatestVersion success: %v", err)
	}
	if err := wf.CreateRun(ctx, &WorkflowRun{TenantID: "t1"}, nil); err != nil {
		t.Fatalf("CreateRun success: %v", err)
	}
	if _, err := wf.GetRun(ctx, "t1", "run1"); err != nil {
		t.Fatalf("GetRun success: %v", err)
	}
	if _, _, err := wf.ListRuns(ctx, "t1", WorkflowRunListOptions{}); err != nil {
		t.Fatalf("ListRuns success: %v", err)
	}
	if err := wf.UpdateRun(ctx, &WorkflowRun{TenantID: "t1"}); err != nil {
		t.Fatalf("UpdateRun success: %v", err)
	}
	if _, err := wf.GetStepRun(ctx, "t1", "step1"); err != nil {
		t.Fatalf("GetStepRun success: %v", err)
	}
	if _, err := wf.ListStepRuns(ctx, "t1", "run1"); err != nil {
		t.Fatalf("ListStepRuns success: %v", err)
	}
	if err := wf.UpdateStepRun(ctx, &WorkflowStepRun{TenantID: "t1"}); err != nil {
		t.Fatalf("UpdateStepRun success: %v", err)
	}
	if _, err := wf.ListPendingHumanTasks(ctx, "t1", "u1", []string{"admin"}); err != nil {
		t.Fatalf("ListPendingHumanTasks success: %v", err)
	}
	if _, err := wf.DeleteAllByTenant(ctx, "t1"); err != nil {
		t.Fatalf("DeleteAllByTenant success: %v", err)
	}

	// ── error path ──
	inner.workflows.err = errMock
	if err := wf.CreateDefinition(ctx, &WorkflowDefinition{TenantID: "t1"}); err == nil {
		t.Fatal("CreateDefinition should return error")
	}
	if err := wf.UpdateDefinition(ctx, &WorkflowDefinition{TenantID: "t1"}); err == nil {
		t.Fatal("UpdateDefinition should return error")
	}
	if _, err := wf.GetDefinition(ctx, "t1", "d1"); err == nil {
		t.Fatal("GetDefinition should return error")
	}
	if _, err := wf.ListDefinitions(ctx, "t1"); err == nil {
		t.Fatal("ListDefinitions should return error")
	}
	if err := wf.CreateVersion(ctx, &WorkflowVersion{TenantID: "t1"}); err == nil {
		t.Fatal("CreateVersion should return error")
	}
	if _, err := wf.GetVersion(ctx, "t1", "v1"); err == nil {
		t.Fatal("GetVersion should return error")
	}
	if _, err := wf.GetLatestVersion(ctx, "t1", "d1"); err == nil {
		t.Fatal("GetLatestVersion should return error")
	}
	if err := wf.CreateRun(ctx, &WorkflowRun{TenantID: "t1"}, nil); err == nil {
		t.Fatal("CreateRun should return error")
	}
	if _, err := wf.GetRun(ctx, "t1", "run1"); err == nil {
		t.Fatal("GetRun should return error")
	}
	if _, _, err := wf.ListRuns(ctx, "t1", WorkflowRunListOptions{}); err == nil {
		t.Fatal("ListRuns should return error")
	}
	if err := wf.UpdateRun(ctx, &WorkflowRun{TenantID: "t1"}); err == nil {
		t.Fatal("UpdateRun should return error")
	}
	if _, err := wf.GetStepRun(ctx, "t1", "step1"); err == nil {
		t.Fatal("GetStepRun should return error")
	}
	if _, err := wf.ListStepRuns(ctx, "t1", "run1"); err == nil {
		t.Fatal("ListStepRuns should return error")
	}
	if err := wf.UpdateStepRun(ctx, &WorkflowStepRun{TenantID: "t1"}); err == nil {
		t.Fatal("UpdateStepRun should return error")
	}
	if _, err := wf.ListPendingHumanTasks(ctx, "t1", "u1", []string{"admin"}); err == nil {
		t.Fatal("ListPendingHumanTasks should return error")
	}
	if _, err := wf.DeleteAllByTenant(ctx, "t1"); err == nil {
		t.Fatal("DeleteAllByTenant should return error")
	}
}

func TestTracedAgentProfiles(t *testing.T) {
	ctx := context.Background()
	inner := newFullMockStore()
	ts := NewTracedStore(inner)
	ap := ts.AgentProfiles()

	// ── success path ──
	if err := ap.Create(ctx, &AgentProfile{TenantID: "t1"}); err != nil {
		t.Fatalf("Create success: %v", err)
	}
	if _, err := ap.Get(ctx, "t1", "u1", "p1"); err != nil {
		t.Fatalf("Get success: %v", err)
	}
	if _, err := ap.List(ctx, "t1", "u1"); err != nil {
		t.Fatalf("List success: %v", err)
	}
	if err := ap.Update(ctx, &AgentProfile{TenantID: "t1"}); err != nil {
		t.Fatalf("Update success: %v", err)
	}
	if err := ap.Delete(ctx, "t1", "u1", "p1"); err != nil {
		t.Fatalf("Delete success: %v", err)
	}
	if _, err := ap.GetDefault(ctx, "t1", "u1"); err != nil {
		t.Fatalf("GetDefault success: %v", err)
	}
	if err := ap.SetDefault(ctx, "t1", "u1", "p1"); err != nil {
		t.Fatalf("SetDefault success: %v", err)
	}

	// ── error path ──
	inner.agentProfiles.err = errMock
	if err := ap.Create(ctx, &AgentProfile{TenantID: "t1"}); err == nil {
		t.Fatal("Create should return error")
	}
	if _, err := ap.Get(ctx, "t1", "u1", "p1"); err == nil {
		t.Fatal("Get should return error")
	}
	if _, err := ap.List(ctx, "t1", "u1"); err == nil {
		t.Fatal("List should return error")
	}
	if err := ap.Update(ctx, &AgentProfile{TenantID: "t1"}); err == nil {
		t.Fatal("Update should return error")
	}
	if err := ap.Delete(ctx, "t1", "u1", "p1"); err == nil {
		t.Fatal("Delete should return error")
	}
	if _, err := ap.GetDefault(ctx, "t1", "u1"); err == nil {
		t.Fatal("GetDefault should return error")
	}
	if err := ap.SetDefault(ctx, "t1", "u1", "p1"); err == nil {
		t.Fatal("SetDefault should return error")
	}
}

// TestTracedFileEntries tests the tracedFileEntries wrapper directly since
// TracedStore.FileEntries() delegates to inner without wrapping.
func TestTracedFileEntries(t *testing.T) {
	ctx := context.Background()
	innerFE := &mockFileEntryStore{}
	fe := &tracedFileEntries{inner: innerFE}

	// ── success path ──
	if _, err := fe.List(ctx, "t1", "u1"); err != nil {
		t.Fatalf("List success: %v", err)
	}
	if _, err := fe.Get(ctx, "t1", "u1", "f1"); err != nil {
		t.Fatalf("Get success: %v", err)
	}
	if err := fe.Create(ctx, &FileEntry{TenantID: "t1"}); err != nil {
		t.Fatalf("Create success: %v", err)
	}
	if err := fe.Delete(ctx, "t1", "u1", "f1"); err != nil {
		t.Fatalf("Delete success: %v", err)
	}
	if _, err := fe.GetUserStorageUsage(ctx, "t1", "u1"); err != nil {
		t.Fatalf("GetUserStorageUsage success: %v", err)
	}

	// ── error path ──
	innerFE.err = errMock
	if _, err := fe.List(ctx, "t1", "u1"); err == nil {
		t.Fatal("List should return error")
	}
	if _, err := fe.Get(ctx, "t1", "u1", "f1"); err == nil {
		t.Fatal("Get should return error")
	}
	if err := fe.Create(ctx, &FileEntry{TenantID: "t1"}); err == nil {
		t.Fatal("Create should return error")
	}
	if err := fe.Delete(ctx, "t1", "u1", "f1"); err == nil {
		t.Fatal("Delete should return error")
	}
	if _, err := fe.GetUserStorageUsage(ctx, "t1", "u1"); err == nil {
		t.Fatal("GetUserStorageUsage should return error")
	}
}
