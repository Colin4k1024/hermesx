package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	orisstore "github.com/Colin4k1024/Oris/sdks/go/store"
	"github.com/Colin4k1024/hermesx/internal/auth"
	"github.com/Colin4k1024/hermesx/internal/evolution"
	"github.com/Colin4k1024/hermesx/internal/store"
)

type governanceAuditStore struct {
	logs []*store.AuditLog
}

func (s *governanceAuditStore) Append(_ context.Context, log *store.AuditLog) error {
	s.logs = append(s.logs, log)
	return nil
}

func (s *governanceAuditStore) List(_ context.Context, _ string, _ store.AuditListOptions) ([]*store.AuditLog, int, error) {
	return s.logs, len(s.logs), nil
}

func (s *governanceAuditStore) DeleteByTenant(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (s *governanceAuditStore) ArchiveOlderThan(_ context.Context, _ time.Time, _ int) ([]*store.AuditLog, error) {
	return nil, nil
}
func (s *governanceAuditStore) ArchiveCount(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

type governanceStore struct {
	store.Store
	audit store.AuditLogStore
}

func (s *governanceStore) AuditLogs() store.AuditLogStore { return s.audit }

func openGovernedGeneStore(t *testing.T) *evolution.GeneStore {
	t.Helper()
	gs, err := evolution.Open(evolution.Config{
		StorageMode: "sqlite",
		DBPath:      filepath.Join(t.TempDir(), "evolution.db"),
		SharingMode: evolution.SharingAnonymous,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gs.Close() })
	return gs
}

func adminReq(method, path, body, scope string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(auth.WithContext(req.Context(), &auth.AuthContext{
		Identity: "00000000-0000-0000-0000-000000000042",
		TenantID: "00000000-0000-0000-0000-000000000001",
		Scopes:   []string{scope},
	}))
}

func TestAdminEvolutionSharingPolicy_UpdateAudited(t *testing.T) {
	gs := openGovernedGeneStore(t)
	audit := &governanceAuditStore{}
	h := NewAdminHandler(&governanceStore{audit: audit}, nil, WithEvolutionStore(gs))

	req := adminReq(http.MethodPut, "/admin/v1/evolution/sharing-policy", `{"mode":"trusted","reason":"enable governed contribution"}`, "sharing:write")
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := gs.SharingPolicySnapshot(); got.Mode != evolution.SharingTrusted {
		t.Fatalf("sharing mode = %q, want trusted", got.Mode)
	} else if got.Version != 1 {
		t.Fatalf("sharing version = %d, want 1", got.Version)
	}
	if len(audit.logs) != 1 || audit.logs[0].Action != "admin.evolution.sharing_policy.update" {
		t.Fatalf("audit logs = %#v", audit.logs)
	}
}

func TestAdminEvolutionSharedKnowledge_RevokeBySourceTenant(t *testing.T) {
	gs := openGovernedGeneStore(t)
	now := time.Now().UTC()
	err := gs.Save(context.Background(), "tenant-a", orisstore.Gene{
		GeneID:     "gene-a",
		Name:       "feature pattern",
		TaskClass:  evolution.TaskClassCodingFeature,
		Confidence: 0.9,
		Strategy:   map[string]any{"steps": []string{"test"}},
		Source:     "test",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := gs.SetSharingMode(evolution.SharingTrusted, "enable trusted sharing"); err != nil {
		t.Fatal(err)
	}
	err = gs.Save(context.Background(), "tenant-b", orisstore.Gene{
		GeneID:     "gene-b",
		Name:       "debug pattern",
		TaskClass:  evolution.TaskClassCodingDebug,
		Confidence: 0.9,
		Strategy:   map[string]any{"steps": []string{"debug"}},
		Source:     "test",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatal(err)
	}

	audit := &governanceAuditStore{}
	h := NewAdminHandler(&governanceStore{audit: audit}, nil, WithEvolutionStore(gs))
	req := adminReq(http.MethodPost, "/admin/v1/evolution/shared-knowledge/revoke", `{"source_tenant":"tenant-b","reason":"bad pattern"}`, "sharing:write")
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got, err := gs.QueryTop(context.Background(), "tenant-c", evolution.TaskClassCodingDebug, 0.5, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("debug shared genes after revoke = %d, want 0", len(got))
	}
	if len(audit.logs) != 1 || audit.logs[0].Action != "admin.evolution.shared_knowledge.revoke" {
		t.Fatalf("audit logs = %#v", audit.logs)
	}
}

func TestAdminEvolutionTenantSharingPolicy_OptOut(t *testing.T) {
	gs := openGovernedGeneStore(t)
	audit := &governanceAuditStore{}
	h := NewAdminHandler(&governanceStore{audit: audit}, nil, WithEvolutionStore(gs))

	req := adminReq(http.MethodPut, "/admin/v1/evolution/tenants/tenant-sensitive/sharing-policy", `{"consume_shared":false,"contribution_mode":"disabled","labels":["sensitive"],"reason":"regulated tenant"}`, "sharing:write")
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	now := time.Now().UTC()
	if err := gs.Save(context.Background(), "tenant-a", orisstore.Gene{
		GeneID:     "shared-source",
		Name:       "feature pattern",
		TaskClass:  evolution.TaskClassCodingFeature,
		Confidence: 0.95,
		Strategy:   map[string]any{"steps": []string{"reuse"}},
		Source:     "test",
		CreatedAt:  now,
		UpdatedAt:  now,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := gs.QueryTop(context.Background(), "tenant-sensitive", evolution.TaskClassCodingFeature, 0.5, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("opted-out tenant received shared genes = %d, want 0", len(got))
	}
	if policy := gs.EffectiveTenantSharingPolicy("tenant-sensitive"); policy.Version != 1 {
		t.Fatalf("tenant policy version = %d, want 1", policy.Version)
	}
	if len(audit.logs) != 1 || audit.logs[0].Action != "admin.evolution.tenant_sharing_policy.update" {
		t.Fatalf("audit logs = %#v", audit.logs)
	}
}

func TestAdminEvolutionSharingPolicy_HistoryAndRollback(t *testing.T) {
	gs := openGovernedGeneStore(t)
	audit := &governanceAuditStore{}
	h := NewAdminHandler(&governanceStore{audit: audit}, nil, WithEvolutionStore(gs))

	req := adminReq(http.MethodPut, "/admin/v1/evolution/sharing-policy", `{"mode":"anonymous","reason":"phase-1"}`, "sharing:write")
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("phase 1 status = %d, body = %s", rec.Code, rec.Body.String())
	}
	req = adminReq(http.MethodPut, "/admin/v1/evolution/sharing-policy", `{"mode":"trusted","reason":"phase-2"}`, "sharing:write")
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("phase 2 status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = adminReq(http.MethodGet, "/admin/v1/evolution/sharing-policy/history?limit=10&offset=0", "", "sharing:read")
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("history status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte(`"version":2`)) || !bytes.Contains([]byte(body), []byte(`"mode":"trusted"`)) {
		t.Fatalf("history body = %s", body)
	}

	req = adminReq(http.MethodPost, "/admin/v1/evolution/sharing-policy/rollback", `{"version":1,"reason":"revert"}`, "sharing:write")
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rollback status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if policy := gs.SharingPolicySnapshot(); policy.Mode != evolution.SharingAnonymous || policy.Version != 3 {
		t.Fatalf("post-rollback policy = %+v, want anonymous v3", policy)
	}
	if len(audit.logs) != 3 || audit.logs[2].Action != "admin.evolution.sharing_policy.rollback" {
		t.Fatalf("audit logs = %#v", audit.logs)
	}
}

func TestAdminEvolutionTenantSharingPolicy_HistoryAndRollback(t *testing.T) {
	gs := openGovernedGeneStore(t)
	audit := &governanceAuditStore{}
	h := NewAdminHandler(&governanceStore{audit: audit}, nil, WithEvolutionStore(gs))

	req := adminReq(http.MethodPut, "/admin/v1/evolution/tenants/tenant-sensitive/sharing-policy", `{"consume_shared":false,"contribution_mode":"disabled","labels":["regulated"],"reason":"strict"}`, "sharing:write")
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("strict status = %d, body = %s", rec.Code, rec.Body.String())
	}
	req = adminReq(http.MethodPut, "/admin/v1/evolution/tenants/tenant-sensitive/sharing-policy", `{"consume_shared":true,"contribution_mode":"anonymous","labels":["approved"],"reason":"relaxed"}`, "sharing:write")
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("relaxed status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = adminReq(http.MethodGet, "/admin/v1/evolution/tenants/tenant-sensitive/sharing-policy/history?limit=10&offset=0", "", "sharing:read")
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tenant history status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte(`"version":2`)) || !bytes.Contains([]byte(body), []byte(`"contribution_mode":"anonymous"`)) {
		t.Fatalf("tenant history body = %s", body)
	}

	req = adminReq(http.MethodPost, "/admin/v1/evolution/tenants/tenant-sensitive/sharing-policy/rollback", `{"version":1,"reason":"restore strict"}`, "sharing:write")
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tenant rollback status = %d, body = %s", rec.Code, rec.Body.String())
	}
	policy := gs.EffectiveTenantSharingPolicy("tenant-sensitive")
	if policy.Version != 3 || policy.ConsumeShared || policy.ContributionMode != evolution.SharingDisabled {
		t.Fatalf("post-rollback tenant policy = %+v", policy)
	}
	if len(audit.logs) != 3 || audit.logs[2].Action != "admin.evolution.tenant_sharing_policy.rollback" {
		t.Fatalf("audit logs = %#v", audit.logs)
	}
}

func TestAdminEvolutionSharingPolicy_RollbackMissingVersion(t *testing.T) {
	gs := openGovernedGeneStore(t)
	h := NewAdminHandler(&governanceStore{audit: &governanceAuditStore{}}, nil, WithEvolutionStore(gs))

	req := adminReq(http.MethodPost, "/admin/v1/evolution/sharing-policy/rollback", `{"version":99,"reason":"missing"}`, "sharing:write")
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminEvolutionSharingPolicy_RollbackCurrentVersionRejected(t *testing.T) {
	gs := openGovernedGeneStore(t)
	h := NewAdminHandler(&governanceStore{audit: &governanceAuditStore{}}, nil, WithEvolutionStore(gs))

	req := adminReq(http.MethodPut, "/admin/v1/evolution/sharing-policy", `{"mode":"trusted","reason":"phase-1"}`, "sharing:write")
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = adminReq(http.MethodPost, "/admin/v1/evolution/sharing-policy/rollback", `{"version":1,"reason":"noop"}`, "sharing:write")
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if policy := gs.SharingPolicySnapshot(); policy.Version != 1 {
		t.Fatalf("version after rejected rollback = %d, want 1", policy.Version)
	}
}
