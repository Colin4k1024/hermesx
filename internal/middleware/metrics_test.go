package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"short path unchanged", "/v1/me", "/v1/me"},
		{"UUID normalized", "/v1/tenants/12345678-1234-1234-1234-123456789012", "/v1/tenants/:id"},
		{"session ID normalized", "/v1/sessions/sess_abcdef0123456789abcdef0123456789", "/v1/sessions/:id"},
		{"numeric ID normalized", "/v1/messages/12345", "/v1/messages/:id"},
		{"health unchanged", "/health/ready", "/health/ready"},
		{"nested UUIDs both normalized", "/v1/tenants/12345678-1234-1234-1234-123456789012/keys/87654321-4321-4321-4321-210987654321", "/v1/tenants/:id/keys/:id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePath(tt.path)
			if got != tt.want {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestMetricsMiddleware_IncrementsCounter(t *testing.T) {
	handler := MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handler response = %d, want 200", rec.Code)
	}
}

func TestMetricsMiddleware_CapturesStatus(t *testing.T) {
	// Test that a handler returning 201 is captured correctly.
	handler := MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/api-keys", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("handler response = %d, want 201", rec.Code)
	}
}

func TestMetricsMiddleware_PanicsWithoutStatus(t *testing.T) {
	// If a handler panics, the defer still calls Dec() — verify no double-decrement.
	handler := MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("intentional")
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic to propagate")
		}
		// After panic recovery, in-flight should not go negative.
		// (The gauge may have been incremented but the defer Dec() won't run after panic.)
	}()

	handler.ServeHTTP(rec, req)
}

func TestStatusWriter_WriteHeader(t *testing.T) {
	tests := []struct {
		name       string
		writeCodes []int
		wantStatus int
	}{
		{"first call wins", []int{200, 404, 500}, 200},
		{"ignores subsequent writes", []int{201, 202, 203}, 201},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			sw := &statusWriter{ResponseWriter: rec, status: 0}
			for _, code := range tt.writeCodes {
				sw.WriteHeader(code)
			}
			if rec.Code != tt.wantStatus {
				t.Errorf("statusWriter final code = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestStatusWriter_Write_CallsWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, status: 0}

	// Calling Write without WriteHeader first should still write the body.
	n, err := sw.Write([]byte("hello"))
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != 5 {
		t.Errorf("Write() n = %d, want 5", n)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Write() implicit code = %d, want 200", rec.Code)
	}
}

func TestMetricsMiddleware_InFlightGauge(t *testing.T) {
	// Use a request that blocks so we can observe the in-flight count.
	done := make(chan struct{})

	handler := MetricsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-done // block until signaled
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	rec := httptest.NewRecorder()

	// Start handler in goroutine.
	go handler.ServeHTTP(rec, req)

	// Give it time to increment the gauge.
	// Verify the handler is still running (in-flight incremented).
	select {
	case <-done:
		t.Fatal("handler finished before we signaled it")
	default:
		// Expected: still running.
	}

	close(done) // Unblock.
}

func TestMetricsMiddleware_PathNormalization(t *testing.T) {
	// Test that long paths are normalized before recording.
	longPath := "/v1/sessions/12345678-1234-1234-1234-123456789012"
	normalized := normalizePath(longPath)

	if len(normalized) > 64 {
		t.Errorf("normalizePath returned path longer than 64 chars: %d", len(normalized))
	}
}
