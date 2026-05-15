package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- helpers ---

// newMockOAuthServer sets up a mock server that handles the device auth flow.
// authState controls responses:
//   - "pending"     → authorization_pending on first poll, then success
//   - "denied"      → access_denied on poll
//   - "expired"     → expired_token on poll
//   - "success"     → immediate success on first poll
func newMockOAuthServer(t *testing.T, authState string) *httptest.Server {
	t.Helper()

	pollCount := 0
	mux := http.NewServeMux()

	mux.HandleFunc("/device/code", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deviceAuthResponse{
			DeviceCode:      "test-device-code",
			UserCode:        "ABCD-1234",
			VerificationURI: "https://example.com/activate",
			ExpiresIn:       300,
			Interval:        5,
		})
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		grantType := r.FormValue("grant_type")

		// Handle refresh token grant.
		if grantType == refreshTokenGrantType {
			rt := r.FormValue("refresh_token")
			if rt == "" {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(tokenResponse{Error: "invalid_request", ErrorDesc: "missing refresh_token"})
				return
			}
			json.NewEncoder(w).Encode(tokenResponse{
				AccessToken:  "refreshed-access-token",
				RefreshToken: "new-refresh-token",
				ExpiresIn:    3600,
				TokenType:    "bearer",
			})
			return
		}

		// Handle device_code grant.
		pollCount++
		switch authState {
		case "pending":
			if pollCount == 1 {
				json.NewEncoder(w).Encode(tokenResponse{Error: "authorization_pending"})
				return
			}
			json.NewEncoder(w).Encode(tokenResponse{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
				ExpiresIn:    3600,
				TokenType:    "bearer",
			})
		case "denied":
			json.NewEncoder(w).Encode(tokenResponse{Error: "access_denied", ErrorDesc: "user denied"})
		case "expired":
			json.NewEncoder(w).Encode(tokenResponse{Error: "expired_token"})
		case "success":
			json.NewEncoder(w).Encode(tokenResponse{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
				ExpiresIn:    3600,
				TokenType:    "bearer",
			})
		}
	})

	return httptest.NewServer(mux)
}

// newTestFlow returns a DeviceAuthFlow pointing to the mock server with zero poll interval.
func newTestFlow(srv *httptest.Server) *DeviceAuthFlow {
	return &DeviceAuthFlow{
		ClientID:      "test-client-id",
		DeviceAuthURL: srv.URL + "/device/code",
		TokenURL:      srv.URL + "/token",
		HTTPClient:    srv.Client(),
		PollInterval:  time.Millisecond, // near-zero for fast tests
	}
}

// --- StartDeviceAuth ---

func TestStartDeviceAuth_Success(t *testing.T) {
	srv := newMockOAuthServer(t, "success")
	defer srv.Close()
	flow := newTestFlow(srv)

	dar, err := flow.StartDeviceAuth(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dar.DeviceCode != "test-device-code" {
		t.Errorf("device_code = %q, want %q", dar.DeviceCode, "test-device-code")
	}
	if dar.UserCode != "ABCD-1234" {
		t.Errorf("user_code = %q, want %q", dar.UserCode, "ABCD-1234")
	}
	if dar.VerificationURI != "https://example.com/activate" {
		t.Errorf("verification_uri = %q", dar.VerificationURI)
	}
}

func TestStartDeviceAuth_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	flow := &DeviceAuthFlow{
		ClientID:      "client",
		DeviceAuthURL: srv.URL,
		HTTPClient:    srv.Client(),
	}
	_, err := flow.StartDeviceAuth(context.Background())
	if err == nil {
		t.Error("expected error from server 500")
	}
}

func TestStartDeviceAuth_ContextCancelled(t *testing.T) {
	// Use a pre-cancelled context to avoid blocking the test server connection.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	srv := newMockOAuthServer(t, "success")
	defer srv.Close()
	flow := newTestFlow(srv)

	_, err := flow.StartDeviceAuth(ctx)
	if err == nil {
		t.Error("expected error when context is already cancelled")
	}
}

// --- PollForToken ---

func TestPollForToken_ImmediateSuccess(t *testing.T) {
	srv := newMockOAuthServer(t, "success")
	defer srv.Close()
	flow := newTestFlow(srv)

	tok, err := flow.PollForToken(context.Background(), "test-device-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.AccessToken != "test-access-token" {
		t.Errorf("access_token = %q", tok.AccessToken)
	}
	if tok.RefreshToken != "test-refresh-token" {
		t.Errorf("refresh_token = %q", tok.RefreshToken)
	}
	if tok.TokenType != "bearer" {
		t.Errorf("token_type = %q", tok.TokenType)
	}
	if tok.ExpiresAt.IsZero() {
		t.Error("expires_at should not be zero")
	}
}

func TestPollForToken_PendingThenSuccess(t *testing.T) {
	srv := newMockOAuthServer(t, "pending")
	defer srv.Close()
	flow := newTestFlow(srv)

	tok, err := flow.PollForToken(context.Background(), "test-device-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.AccessToken != "test-access-token" {
		t.Errorf("access_token = %q", tok.AccessToken)
	}
}

func TestPollForToken_AccessDenied(t *testing.T) {
	srv := newMockOAuthServer(t, "denied")
	defer srv.Close()
	flow := newTestFlow(srv)

	_, err := flow.PollForToken(context.Background(), "test-device-code")
	if err == nil {
		t.Error("expected error on access_denied")
	}
}

func TestPollForToken_ExpiredToken(t *testing.T) {
	srv := newMockOAuthServer(t, "expired")
	defer srv.Close()
	flow := newTestFlow(srv)

	_, err := flow.PollForToken(context.Background(), "test-device-code")
	if err == nil {
		t.Error("expected error on expired_token")
	}
}

func TestPollForToken_ContextCancelled(t *testing.T) {
	// Server always returns pending.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tokenResponse{Error: "authorization_pending"})
	}))
	defer srv.Close()

	flow := &DeviceAuthFlow{
		ClientID:     "client",
		TokenURL:     srv.URL,
		HTTPClient:   srv.Client(),
		PollInterval: time.Millisecond, // fast polling for test
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := flow.PollForToken(ctx, "dc")
	if err == nil {
		t.Error("expected error when context is cancelled")
	}
}

// --- RefreshToken ---

func TestRefreshToken_Success(t *testing.T) {
	srv := newMockOAuthServer(t, "success")
	defer srv.Close()
	flow := newTestFlow(srv)

	tok, err := flow.RefreshToken(context.Background(), "old-refresh-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.AccessToken != "refreshed-access-token" {
		t.Errorf("access_token = %q", tok.AccessToken)
	}
	if tok.RefreshToken != "new-refresh-token" {
		t.Errorf("refresh_token = %q", tok.RefreshToken)
	}
}

func TestRefreshToken_KeepsOldRefreshTokenWhenNoneReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// No refresh_token in response.
		json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: "new-access",
			ExpiresIn:   3600,
			TokenType:   "bearer",
		})
	}))
	defer srv.Close()

	flow := &DeviceAuthFlow{
		ClientID:   "client",
		TokenURL:   srv.URL,
		HTTPClient: srv.Client(),
	}

	tok, err := flow.RefreshToken(context.Background(), "my-refresh-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.RefreshToken != "my-refresh-token" {
		t.Errorf("expected old refresh_token to be kept, got %q", tok.RefreshToken)
	}
}

func TestRefreshToken_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	flow := &DeviceAuthFlow{
		ClientID:   "client",
		TokenURL:   srv.URL,
		HTTPClient: srv.Client(),
	}

	_, err := flow.RefreshToken(context.Background(), "bad-token")
	if err == nil {
		t.Error("expected error on HTTP 401")
	}
}

// --- Token storage ---

func TestSaveAndLoadToken(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	tok := &OAuthToken{
		AccessToken:  "acc-123",
		RefreshToken: "ref-456",
		ExpiresAt:    time.Now().UTC().Add(time.Hour).Truncate(time.Second),
		TokenType:    "bearer",
	}

	if err := SaveToken(tok); err != nil {
		t.Fatalf("save: %v", err)
	}

	// File should exist with 0600 perms.
	info, err := os.Stat(filepath.Join(tmpDir, "tokens", tokenFileName))
	if err != nil {
		t.Fatalf("stat token file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0600 {
		t.Errorf("token file mode = %o, want 0600", mode)
	}

	loaded, err := LoadSavedToken()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil token")
	}
	if loaded.AccessToken != tok.AccessToken {
		t.Errorf("access_token = %q, want %q", loaded.AccessToken, tok.AccessToken)
	}
	if loaded.RefreshToken != tok.RefreshToken {
		t.Errorf("refresh_token = %q, want %q", loaded.RefreshToken, tok.RefreshToken)
	}
	if !loaded.ExpiresAt.Equal(tok.ExpiresAt) {
		t.Errorf("expires_at = %v, want %v", loaded.ExpiresAt, tok.ExpiresAt)
	}
}

func TestLoadSavedToken_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	tok, err := LoadSavedToken()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if tok != nil {
		t.Errorf("expected nil token when file does not exist")
	}
}

func TestLoadSavedToken_MalformedFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	dir := filepath.Join(tmpDir, "tokens")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, tokenFileName), []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadSavedToken()
	if err == nil {
		t.Error("expected error for malformed token file")
	}
}

func TestDeleteToken(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	tok := &OAuthToken{AccessToken: "x", ExpiresAt: time.Now().Add(time.Hour), TokenType: "bearer"}
	if err := SaveToken(tok); err != nil {
		t.Fatal(err)
	}

	if err := DeleteToken(); err != nil {
		t.Errorf("delete: %v", err)
	}

	// Deleting again should not error.
	if err := DeleteToken(); err != nil {
		t.Errorf("second delete: %v", err)
	}

	loaded, err := LoadSavedToken()
	if err != nil {
		t.Errorf("unexpected load error: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil token after delete")
	}
}

// --- OAuthToken.IsExpired ---

func TestOAuthToken_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{"future", time.Now().Add(2 * time.Minute), false},
		{"near expiry within 30s", time.Now().Add(10 * time.Second), true},
		{"past", time.Now().Add(-1 * time.Minute), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tok := &OAuthToken{ExpiresAt: tc.expiresAt}
			if got := tok.IsExpired(); got != tc.want {
				t.Errorf("IsExpired() = %v, want %v", got, tc.want)
			}
		})
	}
}

// --- ResolveAnthropicAPIKey ---

func TestResolveAnthropicAPIKey_EnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	os.Setenv("ANTHROPIC_API_KEY", "sk-env-key")
	defer func() {
		os.Unsetenv("HERMES_HOME")
		os.Unsetenv("ANTHROPIC_API_KEY")
	}()

	key := ResolveAnthropicAPIKey()
	if key != "sk-env-key" {
		t.Errorf("key = %q, want %q", key, "sk-env-key")
	}
}

func TestResolveAnthropicAPIKey_SavedToken(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer func() {
		os.Unsetenv("HERMES_HOME")
	}()

	tok := &OAuthToken{
		AccessToken:  "oauth-access-token",
		RefreshToken: "oauth-refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour),
		TokenType:    "bearer",
	}
	if err := SaveToken(tok); err != nil {
		t.Fatal(err)
	}

	key := ResolveAnthropicAPIKey()
	if key != "oauth-access-token" {
		t.Errorf("key = %q, want %q", key, "oauth-access-token")
	}
}

func TestResolveAnthropicAPIKey_NoCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer func() {
		os.Unsetenv("HERMES_HOME")
	}()

	key := ResolveAnthropicAPIKey()
	if key != "" {
		t.Errorf("expected empty key, got %q", key)
	}
}

func TestResolveAnthropicAPIKey_ExpiredTokenNoRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer func() {
		os.Unsetenv("HERMES_HOME")
	}()

	tok := &OAuthToken{
		AccessToken: "expired-token",
		ExpiresAt:   time.Now().Add(-time.Hour),
		TokenType:   "bearer",
		// No RefreshToken.
	}
	if err := SaveToken(tok); err != nil {
		t.Fatal(err)
	}

	key := ResolveAnthropicAPIKey()
	if key != "" {
		t.Errorf("expected empty key for expired token with no refresh, got %q", key)
	}
}

// --- Login / Logout integration ---

// TestLogin_FlowIntegration verifies the full device-auth → poll → save cycle
// using a single token request (no poll wait).
func TestLogin_FlowIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	srv := newMockOAuthServer(t, "success")
	defer srv.Close()

	flow := newTestFlow(srv)

	// StartDeviceAuth.
	dar, err := flow.StartDeviceAuth(context.Background())
	if err != nil {
		t.Fatalf("start device auth: %v", err)
	}
	if dar.DeviceCode == "" {
		t.Fatal("expected non-empty device code")
	}

	// One direct token request (bypasses the poll interval timer).
	tok, err := flow.requestToken(context.Background(), dar.DeviceCode)
	if err != nil {
		t.Fatalf("request token: %v", err)
	}
	if tok == nil {
		t.Fatal("expected non-nil token from request")
	}

	if err := SaveToken(tok); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadSavedToken()
	if err != nil || loaded == nil {
		t.Fatalf("expected saved token, err=%v", err)
	}
	if loaded.AccessToken != "test-access-token" {
		t.Errorf("access_token = %q", loaded.AccessToken)
	}
}

func TestLogout_RemovesToken(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HERMES_HOME", tmpDir)
	defer os.Unsetenv("HERMES_HOME")

	tok := &OAuthToken{AccessToken: "x", ExpiresAt: time.Now().Add(time.Hour), TokenType: "bearer"}
	if err := SaveToken(tok); err != nil {
		t.Fatal(err)
	}

	if err := Logout(); err != nil {
		t.Fatalf("logout: %v", err)
	}

	loaded, err := LoadSavedToken()
	if err != nil {
		t.Fatalf("unexpected load error after logout: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil token after logout")
	}
}
