package store

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("hermesx-store")

// TracedStore wraps a Store and adds OpenTelemetry spans to key operations.
type TracedStore struct{ inner Store }

// NewTracedStore returns a Store that emits OTel spans for every sub-store operation.
// When no OTLP endpoint is configured the underlying tracer is a no-op (zero overhead).
func NewTracedStore(s Store) Store { return &TracedStore{inner: s} }

func (t *TracedStore) Sessions() SessionStore   { return &tracedSessions{t.inner.Sessions()} }
func (t *TracedStore) Messages() MessageStore   { return &tracedMessages{t.inner.Messages()} }
func (t *TracedStore) Users() UserStore         { return &tracedUsers{t.inner.Users()} }
func (t *TracedStore) Tenants() TenantStore     { return &tracedTenants{t.inner.Tenants()} }
func (t *TracedStore) AuditLogs() AuditLogStore { return &tracedAuditLogs{t.inner.AuditLogs()} }
func (t *TracedStore) APIKeys() APIKeyStore     { return &tracedAPIKeys{t.inner.APIKeys()} }
func (t *TracedStore) Memories() MemoryStore    { return &tracedMemories{t.inner.Memories()} }
func (t *TracedStore) UserProfiles() UserProfileStore {
	return &tracedUserProfiles{t.inner.UserProfiles()}
}
func (t *TracedStore) CronJobs() CronJobStore { return &tracedCronJobs{t.inner.CronJobs()} }
func (t *TracedStore) Roles() RoleStore       { return &tracedRoles{t.inner.Roles()} }
func (t *TracedStore) PricingRules() PricingRuleStore {
	return &tracedPricingRules{t.inner.PricingRules()}
}
func (t *TracedStore) ExecutionReceipts() ExecutionReceiptStore {
	return &tracedExecutionReceipts{t.inner.ExecutionReceipts()}
}
func (t *TracedStore) Close() error                      { return t.inner.Close() }
func (t *TracedStore) Migrate(ctx context.Context) error { return t.inner.Migrate(ctx) }

// ── Sessions ────────────────────────────────────────────────────────────────

type tracedSessions struct{ inner SessionStore }

func (s *tracedSessions) Create(ctx context.Context, tenantID string, sess *Session) error {
	ctx, span := tracer.Start(ctx, "store.Sessions.Create")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	if err := s.inner.Create(ctx, tenantID, sess); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (s *tracedSessions) Get(ctx context.Context, tenantID, sessionID string) (*Session, error) {
	ctx, span := tracer.Start(ctx, "store.Sessions.Get")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID), attribute.String("session_id", sessionID))
	v, err := s.inner.Get(ctx, tenantID, sessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (s *tracedSessions) End(ctx context.Context, tenantID, sessionID, reason string) error {
	ctx, span := tracer.Start(ctx, "store.Sessions.End")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID), attribute.String("session_id", sessionID))
	if err := s.inner.End(ctx, tenantID, sessionID, reason); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (s *tracedSessions) List(ctx context.Context, tenantID string, opts ListOptions) ([]*Session, int, error) {
	ctx, span := tracer.Start(ctx, "store.Sessions.List")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, n, err := s.inner.List(ctx, tenantID, opts)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, n, err
}

func (s *tracedSessions) Delete(ctx context.Context, tenantID, sessionID string) error {
	ctx, span := tracer.Start(ctx, "store.Sessions.Delete")
	defer span.End()
	if err := s.inner.Delete(ctx, tenantID, sessionID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (s *tracedSessions) UpdateTokens(ctx context.Context, tenantID, sessionID string, delta TokenDelta) error {
	ctx, span := tracer.Start(ctx, "store.Sessions.UpdateTokens")
	defer span.End()
	if err := s.inner.UpdateTokens(ctx, tenantID, sessionID, delta); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (s *tracedSessions) SetTitle(ctx context.Context, tenantID, sessionID, title string) error {
	ctx, span := tracer.Start(ctx, "store.Sessions.SetTitle")
	defer span.End()
	if err := s.inner.SetTitle(ctx, tenantID, sessionID, title); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// ── Messages ────────────────────────────────────────────────────────────────

type tracedMessages struct{ inner MessageStore }

func (m *tracedMessages) Append(ctx context.Context, tenantID, sessionID string, msg *Message) (int64, error) {
	ctx, span := tracer.Start(ctx, "store.Messages.Append")
	defer span.End()
	span.SetAttributes(
		attribute.String("tenant_id", tenantID),
		attribute.String("session_id", sessionID),
		attribute.String("role", msg.Role),
	)
	id, err := m.inner.Append(ctx, tenantID, sessionID, msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return id, err
}

func (m *tracedMessages) List(ctx context.Context, tenantID, sessionID string, limit, offset int) ([]*Message, error) {
	ctx, span := tracer.Start(ctx, "store.Messages.List")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID), attribute.String("session_id", sessionID))
	v, err := m.inner.List(ctx, tenantID, sessionID, limit, offset)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (m *tracedMessages) Search(ctx context.Context, tenantID, query string, limit int) ([]*SearchResult, error) {
	ctx, span := tracer.Start(ctx, "store.Messages.Search")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := m.inner.Search(ctx, tenantID, query, limit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (m *tracedMessages) CountBySession(ctx context.Context, tenantID, sessionID string) (int, error) {
	ctx, span := tracer.Start(ctx, "store.Messages.CountBySession")
	defer span.End()
	v, err := m.inner.CountBySession(ctx, tenantID, sessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

// ── Users ────────────────────────────────────────────────────────────────────

type tracedUsers struct{ inner UserStore }

func (u *tracedUsers) GetOrCreate(ctx context.Context, tenantID, externalID, username string) (*User, error) {
	ctx, span := tracer.Start(ctx, "store.Users.GetOrCreate")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := u.inner.GetOrCreate(ctx, tenantID, externalID, username)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (u *tracedUsers) IsApproved(ctx context.Context, tenantID, platform, userID string) (bool, error) {
	ctx, span := tracer.Start(ctx, "store.Users.IsApproved")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := u.inner.IsApproved(ctx, tenantID, platform, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (u *tracedUsers) Approve(ctx context.Context, tenantID, platform, userID string) error {
	ctx, span := tracer.Start(ctx, "store.Users.Approve")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	if err := u.inner.Approve(ctx, tenantID, platform, userID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (u *tracedUsers) Revoke(ctx context.Context, tenantID, platform, userID string) error {
	ctx, span := tracer.Start(ctx, "store.Users.Revoke")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	if err := u.inner.Revoke(ctx, tenantID, platform, userID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (u *tracedUsers) ListApproved(ctx context.Context, tenantID, platform string) ([]string, error) {
	ctx, span := tracer.Start(ctx, "store.Users.ListApproved")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := u.inner.ListApproved(ctx, tenantID, platform)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

// ── Tenants ──────────────────────────────────────────────────────────────────

type tracedTenants struct{ inner TenantStore }

func (t *tracedTenants) Create(ctx context.Context, tenant *Tenant) error {
	ctx, span := tracer.Start(ctx, "store.Tenants.Create")
	defer span.End()
	if err := t.inner.Create(ctx, tenant); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (t *tracedTenants) Get(ctx context.Context, id string) (*Tenant, error) {
	ctx, span := tracer.Start(ctx, "store.Tenants.Get")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", id))
	v, err := t.inner.Get(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (t *tracedTenants) Update(ctx context.Context, tenant *Tenant) error {
	ctx, span := tracer.Start(ctx, "store.Tenants.Update")
	defer span.End()
	if err := t.inner.Update(ctx, tenant); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (t *tracedTenants) Delete(ctx context.Context, id string) error {
	ctx, span := tracer.Start(ctx, "store.Tenants.Delete")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", id))
	if err := t.inner.Delete(ctx, id); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (t *tracedTenants) List(ctx context.Context, opts ListOptions) ([]*Tenant, int, error) {
	ctx, span := tracer.Start(ctx, "store.Tenants.List")
	defer span.End()
	v, n, err := t.inner.List(ctx, opts)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, n, err
}

func (t *tracedTenants) ListDeleted(ctx context.Context, olderThan time.Time) ([]*Tenant, error) {
	ctx, span := tracer.Start(ctx, "store.Tenants.ListDeleted")
	defer span.End()
	v, err := t.inner.ListDeleted(ctx, olderThan)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (t *tracedTenants) HardDelete(ctx context.Context, id string) error {
	ctx, span := tracer.Start(ctx, "store.Tenants.HardDelete")
	defer span.End()
	if err := t.inner.HardDelete(ctx, id); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (t *tracedTenants) Restore(ctx context.Context, id string) error {
	ctx, span := tracer.Start(ctx, "store.Tenants.Restore")
	defer span.End()
	if err := t.inner.Restore(ctx, id); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// ── AuditLogs ────────────────────────────────────────────────────────────────

type tracedAuditLogs struct{ inner AuditLogStore }

func (a *tracedAuditLogs) Append(ctx context.Context, log *AuditLog) error {
	ctx, span := tracer.Start(ctx, "store.AuditLogs.Append")
	defer span.End()
	if err := a.inner.Append(ctx, log); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (a *tracedAuditLogs) List(ctx context.Context, tenantID string, opts AuditListOptions) ([]*AuditLog, int, error) {
	ctx, span := tracer.Start(ctx, "store.AuditLogs.List")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, n, err := a.inner.List(ctx, tenantID, opts)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, n, err
}

func (a *tracedAuditLogs) DeleteByTenant(ctx context.Context, tenantID string) (int64, error) {
	ctx, span := tracer.Start(ctx, "store.AuditLogs.DeleteByTenant")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	n, err := a.inner.DeleteByTenant(ctx, tenantID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return n, err
}

// ── APIKeys ──────────────────────────────────────────────────────────────────

type tracedAPIKeys struct{ inner APIKeyStore }

func (k *tracedAPIKeys) Create(ctx context.Context, key *APIKey) error {
	ctx, span := tracer.Start(ctx, "store.APIKeys.Create")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", key.TenantID))
	if err := k.inner.Create(ctx, key); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (k *tracedAPIKeys) GetByHash(ctx context.Context, hash string) (*APIKey, error) {
	ctx, span := tracer.Start(ctx, "store.APIKeys.GetByHash")
	defer span.End()
	v, err := k.inner.GetByHash(ctx, hash)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (k *tracedAPIKeys) GetByID(ctx context.Context, tenantID, id string) (*APIKey, error) {
	ctx, span := tracer.Start(ctx, "store.APIKeys.GetByID")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := k.inner.GetByID(ctx, tenantID, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (k *tracedAPIKeys) List(ctx context.Context, tenantID string) ([]*APIKey, error) {
	ctx, span := tracer.Start(ctx, "store.APIKeys.List")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := k.inner.List(ctx, tenantID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (k *tracedAPIKeys) Revoke(ctx context.Context, tenantID, id string) error {
	ctx, span := tracer.Start(ctx, "store.APIKeys.Revoke")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	if err := k.inner.Revoke(ctx, tenantID, id); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// ── Memories ─────────────────────────────────────────────────────────────────

type tracedMemories struct{ inner MemoryStore }

func (m *tracedMemories) Get(ctx context.Context, tenantID, userID, key string) (string, error) {
	ctx, span := tracer.Start(ctx, "store.Memories.Get")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := m.inner.Get(ctx, tenantID, userID, key)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (m *tracedMemories) List(ctx context.Context, tenantID, userID string) ([]MemoryEntry, error) {
	ctx, span := tracer.Start(ctx, "store.Memories.List")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := m.inner.List(ctx, tenantID, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (m *tracedMemories) Upsert(ctx context.Context, tenantID, userID, key, content string) error {
	ctx, span := tracer.Start(ctx, "store.Memories.Upsert")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	if err := m.inner.Upsert(ctx, tenantID, userID, key, content); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (m *tracedMemories) Delete(ctx context.Context, tenantID, userID, key string) error {
	ctx, span := tracer.Start(ctx, "store.Memories.Delete")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	if err := m.inner.Delete(ctx, tenantID, userID, key); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (m *tracedMemories) DeleteAllByUser(ctx context.Context, tenantID, userID string) (int64, error) {
	ctx, span := tracer.Start(ctx, "store.Memories.DeleteAllByUser")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	n, err := m.inner.DeleteAllByUser(ctx, tenantID, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return n, err
}

func (m *tracedMemories) DeleteAllByTenant(ctx context.Context, tenantID string) (int64, error) {
	ctx, span := tracer.Start(ctx, "store.Memories.DeleteAllByTenant")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	n, err := m.inner.DeleteAllByTenant(ctx, tenantID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return n, err
}

// ── UserProfiles ─────────────────────────────────────────────────────────────

type tracedUserProfiles struct{ inner UserProfileStore }

func (p *tracedUserProfiles) Get(ctx context.Context, tenantID, userID string) (string, error) {
	ctx, span := tracer.Start(ctx, "store.UserProfiles.Get")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := p.inner.Get(ctx, tenantID, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (p *tracedUserProfiles) Upsert(ctx context.Context, tenantID, userID, content string) error {
	ctx, span := tracer.Start(ctx, "store.UserProfiles.Upsert")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	if err := p.inner.Upsert(ctx, tenantID, userID, content); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (p *tracedUserProfiles) Delete(ctx context.Context, tenantID, userID string) error {
	ctx, span := tracer.Start(ctx, "store.UserProfiles.Delete")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	if err := p.inner.Delete(ctx, tenantID, userID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (p *tracedUserProfiles) DeleteAllByTenant(ctx context.Context, tenantID string) (int64, error) {
	ctx, span := tracer.Start(ctx, "store.UserProfiles.DeleteAllByTenant")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	n, err := p.inner.DeleteAllByTenant(ctx, tenantID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return n, err
}

// ── CronJobs ─────────────────────────────────────────────────────────────────

type tracedCronJobs struct{ inner CronJobStore }

func (c *tracedCronJobs) Create(ctx context.Context, job *CronJob) error {
	ctx, span := tracer.Start(ctx, "store.CronJobs.Create")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", job.TenantID))
	if err := c.inner.Create(ctx, job); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (c *tracedCronJobs) Get(ctx context.Context, tenantID, jobID string) (*CronJob, error) {
	ctx, span := tracer.Start(ctx, "store.CronJobs.Get")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := c.inner.Get(ctx, tenantID, jobID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (c *tracedCronJobs) Update(ctx context.Context, job *CronJob) error {
	ctx, span := tracer.Start(ctx, "store.CronJobs.Update")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", job.TenantID))
	if err := c.inner.Update(ctx, job); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (c *tracedCronJobs) Delete(ctx context.Context, tenantID, jobID string) error {
	ctx, span := tracer.Start(ctx, "store.CronJobs.Delete")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	if err := c.inner.Delete(ctx, tenantID, jobID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (c *tracedCronJobs) List(ctx context.Context, tenantID string) ([]*CronJob, error) {
	ctx, span := tracer.Start(ctx, "store.CronJobs.List")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := c.inner.List(ctx, tenantID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (c *tracedCronJobs) ListDue(ctx context.Context, now time.Time) ([]*CronJob, error) {
	ctx, span := tracer.Start(ctx, "store.CronJobs.ListDue")
	defer span.End()
	v, err := c.inner.ListDue(ctx, now)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

// ── Roles ────────────────────────────────────────────────────────────────────

type tracedRoles struct{ inner RoleStore }

func (r *tracedRoles) Create(ctx context.Context, role *Role) error {
	ctx, span := tracer.Start(ctx, "store.Roles.Create")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", role.TenantID))
	if err := r.inner.Create(ctx, role); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (r *tracedRoles) Get(ctx context.Context, tenantID, roleID string) (*Role, error) {
	ctx, span := tracer.Start(ctx, "store.Roles.Get")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := r.inner.Get(ctx, tenantID, roleID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (r *tracedRoles) GetByName(ctx context.Context, tenantID, name string) (*Role, error) {
	ctx, span := tracer.Start(ctx, "store.Roles.GetByName")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := r.inner.GetByName(ctx, tenantID, name)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (r *tracedRoles) List(ctx context.Context, tenantID string) ([]*Role, error) {
	ctx, span := tracer.Start(ctx, "store.Roles.List")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := r.inner.List(ctx, tenantID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (r *tracedRoles) Delete(ctx context.Context, tenantID, roleID string) error {
	ctx, span := tracer.Start(ctx, "store.Roles.Delete")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	if err := r.inner.Delete(ctx, tenantID, roleID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (r *tracedRoles) AddPermission(ctx context.Context, tenantID, roleName, resource, action string) error {
	ctx, span := tracer.Start(ctx, "store.Roles.AddPermission")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID), attribute.String("role", roleName))
	if err := r.inner.AddPermission(ctx, tenantID, roleName, resource, action); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (r *tracedRoles) RemovePermission(ctx context.Context, tenantID, roleName, resource, action string) error {
	ctx, span := tracer.Start(ctx, "store.Roles.RemovePermission")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID), attribute.String("role", roleName))
	if err := r.inner.RemovePermission(ctx, tenantID, roleName, resource, action); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (r *tracedRoles) ListPermissions(ctx context.Context, tenantID, roleName string) ([]*RolePermission, error) {
	ctx, span := tracer.Start(ctx, "store.Roles.ListPermissions")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID), attribute.String("role", roleName))
	v, err := r.inner.ListPermissions(ctx, tenantID, roleName)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (r *tracedRoles) HasPermission(ctx context.Context, roles []string, tenantID, resource, action string) (bool, error) {
	ctx, span := tracer.Start(ctx, "store.Roles.HasPermission")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := r.inner.HasPermission(ctx, roles, tenantID, resource, action)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

// ── PricingRules ─────────────────────────────────────────────────────────────

type tracedPricingRules struct{ inner PricingRuleStore }

func (p *tracedPricingRules) List(ctx context.Context) ([]PricingRule, error) {
	ctx, span := tracer.Start(ctx, "store.PricingRules.List")
	defer span.End()
	v, err := p.inner.List(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (p *tracedPricingRules) Get(ctx context.Context, modelKey string) (*PricingRule, error) {
	ctx, span := tracer.Start(ctx, "store.PricingRules.Get")
	defer span.End()
	v, err := p.inner.Get(ctx, modelKey)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (p *tracedPricingRules) Upsert(ctx context.Context, rule *PricingRule) error {
	ctx, span := tracer.Start(ctx, "store.PricingRules.Upsert")
	defer span.End()
	if err := p.inner.Upsert(ctx, rule); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (p *tracedPricingRules) Delete(ctx context.Context, modelKey string) error {
	ctx, span := tracer.Start(ctx, "store.PricingRules.Delete")
	defer span.End()
	if err := p.inner.Delete(ctx, modelKey); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// ── ExecutionReceipts ────────────────────────────────────────────────────────

type tracedExecutionReceipts struct{ inner ExecutionReceiptStore }

func (e *tracedExecutionReceipts) Create(ctx context.Context, receipt *ExecutionReceipt) error {
	ctx, span := tracer.Start(ctx, "store.ExecutionReceipts.Create")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", receipt.TenantID))
	if err := e.inner.Create(ctx, receipt); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

func (e *tracedExecutionReceipts) Get(ctx context.Context, tenantID, id string) (*ExecutionReceipt, error) {
	ctx, span := tracer.Start(ctx, "store.ExecutionReceipts.Get")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := e.inner.Get(ctx, tenantID, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}

func (e *tracedExecutionReceipts) List(ctx context.Context, tenantID string, opts ReceiptListOptions) ([]*ExecutionReceipt, int, error) {
	ctx, span := tracer.Start(ctx, "store.ExecutionReceipts.List")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, n, err := e.inner.List(ctx, tenantID, opts)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, n, err
}

func (e *tracedExecutionReceipts) GetByIdempotencyID(ctx context.Context, tenantID, idempotencyID string) (*ExecutionReceipt, error) {
	ctx, span := tracer.Start(ctx, "store.ExecutionReceipts.GetByIdempotencyID")
	defer span.End()
	span.SetAttributes(attribute.String("tenant_id", tenantID))
	v, err := e.inner.GetByIdempotencyID(ctx, tenantID, idempotencyID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return v, err
}
