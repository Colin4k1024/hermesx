package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetSandboxPolicy_MissingTenantID(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest("POST", "/admin/v1/tenants//sandbox-policy", nil)
	w := httptest.NewRecorder()

	h.setSandboxPolicy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestSetSandboxPolicy_InvalidBody(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest("POST", "/admin/v1/tenants/test-tenant/sandbox-policy",
		bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	h.setSandboxPolicy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGetSandboxPolicy_MissingTenantID(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest("GET", "/admin/v1/tenants//sandbox-policy", nil)
	w := httptest.NewRecorder()

	h.getSandboxPolicy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestDeleteSandboxPolicy_MissingTenantID(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest("DELETE", "/admin/v1/tenants//sandbox-policy", nil)
	w := httptest.NewRecorder()

	h.deleteSandboxPolicy(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestListAPIKeys_MissingTenantID(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest("GET", "/admin/v1/tenants//api-keys", nil)
	w := httptest.NewRecorder()

	h.listAPIKeys(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateAPIKey_MissingTenantID(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest("POST", "/admin/v1/tenants//api-keys", nil)
	w := httptest.NewRecorder()

	h.createAPIKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateAPIKey_InvalidBody(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest("POST", "/admin/v1/tenants/test-tenant/api-keys",
		bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	h.createAPIKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateAPIKey_MissingName(t *testing.T) {
	h := &AdminHandler{}
	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest("POST", "/admin/v1/tenants/test-tenant/api-keys",
		bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	h.createAPIKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRevokeAPIKey_MissingTenantID(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest("DELETE", "/admin/v1/tenants//api-keys/test-key", nil)
	w := httptest.NewRecorder()

	h.revokeAPIKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRevokeAPIKey_MissingKeyID(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest("DELETE", "/admin/v1/tenants/test-tenant/api-keys/", nil)
	w := httptest.NewRecorder()

	h.revokeAPIKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRotateAPIKey_MissingTenantID(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest("POST", "/admin/v1/tenants//api-keys/test-key/rotate", nil)
	w := httptest.NewRecorder()

	h.rotateAPIKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestRotateAPIKey_MissingKeyID(t *testing.T) {
	h := &AdminHandler{}
	req := httptest.NewRequest("POST", "/admin/v1/tenants/test-tenant/api-keys//rotate", nil)
	w := httptest.NewRecorder()

	h.rotateAPIKey(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
