package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockDB struct {
	err error
}

func (m *mockDB) Ping(_ context.Context) error { return m.err }

func TestHealthHandler(t *testing.T) {
	tests := []struct {
		name       string
		handler    func(*HealthHandler) http.HandlerFunc
		db         DBPinger
		wantStatus int
		wantBody   map[string]string
	}{
		{
			name:       "live always returns 200",
			handler:    (*HealthHandler).LiveHandler,
			db:         &mockDB{},
			wantStatus: http.StatusOK,
			wantBody:   map[string]string{"status": "alive"},
		},
		{
			name:       "ready with healthy db",
			handler:    (*HealthHandler).ReadyHandler,
			db:         &mockDB{err: nil},
			wantStatus: http.StatusOK,
			wantBody:   map[string]string{"status": "ready", "database": "ok"},
		},
		{
			name:       "ready with unhealthy db",
			handler:    (*HealthHandler).ReadyHandler,
			db:         &mockDB{err: errors.New("connection refused")},
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   map[string]string{"status": "not_ready"},
		},
		{
			name:       "ready with nil db skips database check",
			handler:    (*HealthHandler).ReadyHandler,
			db:         nil,
			wantStatus: http.StatusOK,
			wantBody:   map[string]string{"status": "ready"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHealthHandler(tt.db)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/health", nil)

			tt.handler(h).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			ct := rec.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var got map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("failed to decode body: %v", err)
			}

			for k, want := range tt.wantBody {
				if got[k] != want {
					t.Errorf("body[%q] = %q, want %q", k, got[k], want)
				}
			}
		})
	}
}
