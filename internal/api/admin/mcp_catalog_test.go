package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/mcpcatalog"
)

func TestAdminMCPCatalog_UpsertListAndTenantPolicy(t *testing.T) {
	catalog := mcpcatalog.NewMemoryStore()
	audit := &governanceAuditStore{}
	h := NewAdminHandler(&governanceStore{audit: audit}, nil, WithMCPCatalog(catalog))

	body := `{
		"name":"n8n",
		"source_url":"https://github.com/n8n-io/n8n",
		"trust_tier":"trusted",
		"review_status":"approved",
		"transport":"stdio",
		"command":"npx",
		"args":["-y","@n8n/mcp"],
		"required_credentials":["N8N_API_KEY"],
		"scopes":["workflow:read"],
		"reason":"seed trusted catalog item"
	}`
	req := adminReq(http.MethodPut, "/admin/v1/mcp-catalog/n8n", body, "security:write")
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = adminReq(http.MethodGet, "/admin/v1/mcp-catalog", "", "security:read")
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Items []mcpcatalog.Item `json:"items"`
		Count int               `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listResp.Count != 1 || listResp.Items[0].ID != "n8n" {
		t.Fatalf("list response = %+v", listResp)
	}

	req = adminReq(http.MethodPut, "/admin/v1/mcp-catalog/tenants/tenant-a/items/n8n", `{"enabled":true,"reason":"approved for tenant"}`, "tenant:write")
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tenant policy status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = adminReq(http.MethodGet, "/admin/v1/mcp-catalog/tenants/tenant-a", "", "tenant:read")
	rec = httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tenant list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"enabled":true`)) {
		t.Fatalf("tenant policy body = %s", rec.Body.String())
	}

	if len(audit.logs) != 2 {
		t.Fatalf("audit logs = %#v", audit.logs)
	}
}

func TestAdminMCPCatalog_InvalidItemRejected(t *testing.T) {
	h := NewAdminHandler(&governanceStore{audit: &governanceAuditStore{}}, nil, WithMCPCatalog(mcpcatalog.NewMemoryStore()))

	req := adminReq(http.MethodPut, "/admin/v1/mcp-catalog/bad", `{"name":"bad","source_url":"https://example.com","trust_tier":"trusted","review_status":"approved","transport":"stdio"}`, "security:write")
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminMCPCatalog_RouteRequiresConfiguredCatalog(t *testing.T) {
	h := NewAdminHandler(&governanceStore{audit: &governanceAuditStore{}}, nil)

	req := adminReq(http.MethodGet, "/admin/v1/mcp-catalog", "", "security:read")
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
