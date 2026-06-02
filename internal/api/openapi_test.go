package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAPISpec(t *testing.T) {
	handler := OpenAPISpec()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/openapi", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var spec map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&spec); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if spec["openapi"] != "3.0.3" {
		t.Errorf("openapi = %v, want 3.0.3", spec["openapi"])
	}

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("expected paths object")
	}
	if _, ok := paths["/v1/usage"]; !ok {
		t.Error("missing /v1/usage path")
	}
}

func TestOpenAPISpec_AllPathsPresent(t *testing.T) {
	handler := OpenAPISpec()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/openapi", nil)
	handler.ServeHTTP(rec, req)

	var spec map[string]any
	json.NewDecoder(rec.Body).Decode(&spec)
	paths := spec["paths"].(map[string]any)

	// All registered paths must be present in the spec.
	// Health/metrics have no /v1 prefix; admin routes are under /admin/v1/.
	wantPaths := []string{
		"/health/live",
		"/health/ready",
		"/metrics",
		"/auth/channel/{platform}/start",
		"/auth/channel/{platform}/callback",
		"/auth/logout",
		"/v1/chat/completions",
		"/v1/agent/chat",
		"/v1/sessions",
		"/v1/tenants",
		"/v1/api-keys",
		"/v1/audit-logs",
		"/v1/execution-receipts",
		"/v1/workflow-definitions",
		"/v1/workflow-runs",
		"/v1/workflow-tasks",
		"/v1/usage",
		"/v1/me",
		"/v1/channel-bindings",
		"/v1/channel-bindings/{id}",
		"/v1/gdpr/cleanup-minio",
		"/admin/v1/bootstrap",
		"/admin/v1/bootstrap/status",
		"/admin/v1/tenants/{id}/api-keys",
		"/admin/v1/tenants/{id}/api-keys/{kid}",
		"/admin/v1/tenants/{id}/api-keys/{kid}/rotate",
		"/admin/v1/tenants/{id}/sandbox-policy",
		"/admin/v1/pricing-rules",
		"/admin/v1/pricing-rules/{model}",
		"/admin/v1/audit-logs",
		"/admin/v1/channel-apps",
		"/admin/v1/channel-apps/{id}",
		"/admin/v1/channel-bindings",
		"/admin/v1/channel-bindings/{id}",
		"/admin/v1/usage",
		"/admin/v1/usage/tenants",
		"/admin/v1/evolution/sharing-policy",
		"/admin/v1/evolution/sharing-policy/history",
		"/admin/v1/evolution/sharing-policy/rollback",
		"/admin/v1/evolution/tenants/{id}/sharing-policy",
		"/admin/v1/evolution/tenants/{id}/sharing-policy/history",
		"/admin/v1/evolution/tenants/{id}/sharing-policy/rollback",
		"/admin/v1/evolution/shared-knowledge/revoke",
		"/admin/v1/secrets/patterns",
		"/admin/v1/secrets/canary-tokens",
		"/admin/v1/secrets/canary-tokens/{id}",
		"/admin/v1/safety/rules",
		"/admin/v1/safety/rules/{id}",
		"/admin/v1/safety/scan",
		"/admin/v1/egress/allowlist",
		"/admin/v1/egress/allowlist/{id}",
		"/admin/v1/egress/blocked-log",
	}

	for _, p := range wantPaths {
		if _, ok := paths[p]; !ok {
			t.Errorf("missing path in spec: %s", p)
		}
	}
}

func TestOpenAPISpec_Structure(t *testing.T) {
	handler := OpenAPISpec()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/openapi", nil))

	var spec map[string]any
	json.NewDecoder(rec.Body).Decode(&spec)

	// Verify top-level structure.
	for _, field := range []string{"openapi", "info", "paths", "components"} {
		if _, ok := spec[field]; !ok {
			t.Errorf("missing top-level field: %s", field)
		}
	}

	// Info block must have title and version.
	info := spec["info"].(map[string]any)
	if info["title"] == "" {
		t.Error("info.title is empty")
	}
	if info["version"] == "" {
		t.Error("info.version is empty")
	}

	// Components must have securitySchemes.
	components := spec["components"].(map[string]any)
	if _, ok := components["securitySchemes"]; !ok {
		t.Error("missing components.securitySchemes")
	}

	// Security field must be set.
	security, ok := spec["security"].([]any)
	if !ok || len(security) == 0 {
		t.Error("spec.security must be a non-empty array")
	}
}

func TestOpenAPISpec_EachPathHasOperation(t *testing.T) {
	handler := OpenAPISpec()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/openapi", nil))

	var spec map[string]any
	json.NewDecoder(rec.Body).Decode(&spec)
	paths := spec["paths"].(map[string]any)

	// Each path entry must contain at least one HTTP method operation.
	for path, v := range paths {
		pathItem, ok := v.(map[string]any)
		if !ok {
			t.Errorf("path %s: expected map, got %T", path, v)
			continue
		}
		hasOp := false
		for _, method := range []string{"get", "post", "put", "delete", "patch"} {
			if _, ok := pathItem[method]; ok {
				hasOp = true
				break
			}
		}
		if !hasOp {
			t.Errorf("path %s: no HTTP operation found", path)
		}
	}
}

func TestOpenAPISpec_PathMethodsHaveResponses(t *testing.T) {
	handler := OpenAPISpec()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/openapi", nil))

	var spec map[string]any
	json.NewDecoder(rec.Body).Decode(&spec)
	paths := spec["paths"].(map[string]any)

	// Each path+method must have a "responses" field.
	for path, v := range paths {
		pathItem := v.(map[string]any)
		for method, op := range pathItem {
			if method == "summary" || method == "description" {
				continue
			}
			opMap := op.(map[string]any)
			if _, ok := opMap["responses"]; !ok {
				t.Errorf("%s (%s): missing responses field", path, method)
			}
		}
	}
}

func TestOpenAPISpec_InfoBranding(t *testing.T) {
	handler := OpenAPISpec()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/openapi", nil))

	var spec map[string]any
	json.NewDecoder(rec.Body).Decode(&spec)
	info := spec["info"].(map[string]any)

	title, _ := info["title"].(string)
	if title == "" {
		t.Fatal("info.title is empty")
	}
	// Title must reference HermesX, not the old "Hermes" branding.
	if title == "Hermes Agent API" {
		t.Errorf("info.title still uses old branding %q; want HermesX", title)
	}

	version, _ := info["version"].(string)
	if version != "2.4.0-dev" {
		t.Errorf("info.version = %q, want 2.4.0-dev", version)
	}
}
