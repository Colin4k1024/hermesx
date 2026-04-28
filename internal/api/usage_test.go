package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hermes-agent/hermes-agent-go/internal/middleware"
	"github.com/hermes-agent/hermes-agent-go/internal/store"
)

func usageReq(method, path string, tenantID string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := req.Context()
	if tenantID != "" {
		ctx = middleware.WithTenant(ctx, tenantID)
	}
	return req.WithContext(ctx)
}

func TestUsageHandler(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		tenantID   string
		seedData   func(*mockSessionStore)
		wantStatus int
		checkBody  func(t *testing.T, body []byte)
	}{
		{
			name:     "GET with tenant and sessions returns usage",
			method:   http.MethodGet,
			tenantID: "tenant-1",
			seedData: func(ss *mockSessionStore) {
				ss.sessions["tenant-1/s1"] = &store.Session{
					ID: "s1", TenantID: "tenant-1", InputTokens: 100, OutputTokens: 50,
				}
				ss.sessions["tenant-1/s2"] = &store.Session{
					ID: "s2", TenantID: "tenant-1", InputTokens: 200, OutputTokens: 150,
				}
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var resp map[string]any
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if int(resp["total_sessions"].(float64)) != 2 {
					t.Errorf("total_sessions = %v, want 2", resp["total_sessions"])
				}
				if int(resp["total_input_tokens"].(float64)) != 300 {
					t.Errorf("total_input_tokens = %v, want 300", resp["total_input_tokens"])
				}
				if int(resp["total_output_tokens"].(float64)) != 200 {
					t.Errorf("total_output_tokens = %v, want 200", resp["total_output_tokens"])
				}
			},
		},
		{
			name:       "GET without tenant returns 400",
			method:     http.MethodGet,
			tenantID:   "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "POST returns 405",
			method:     http.MethodPost,
			tenantID:   "tenant-1",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "GET with no sessions returns zero tokens",
			method:     http.MethodGet,
			tenantID:   "tenant-1",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var resp map[string]any
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if int(resp["total_input_tokens"].(float64)) != 0 {
					t.Errorf("total_input_tokens = %v, want 0", resp["total_input_tokens"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss := newMockSessionStore()
			ms := newMockMessageStore()
			if tt.seedData != nil {
				tt.seedData(ss)
			}

			handler := NewUsageHandler(ss, ms)
			rec := httptest.NewRecorder()
			req := usageReq(tt.method, "/v1/usage", tt.tenantID)

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.checkBody != nil {
				tt.checkBody(t, rec.Body.Bytes())
			}
		})
	}
}
