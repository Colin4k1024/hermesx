// Package cli provides CLI utilities including OAuth 2.0 Device Authorization Grant (RFC 8628).
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Colin4k1024/hermesx/internal/config"
)

const (
	// defaultDeviceAuthURL is the Anthropic Console device authorization endpoint.
	defaultDeviceAuthURL = "https://console.anthropic.com/v1/oauth/device/code"
	// defaultTokenURL is the Anthropic Console token endpoint.
	defaultTokenURL = "https://console.anthropic.com/v1/oauth/token"
	// deviceCodeGrantType is the RFC 8628 grant type for device code polling.
	deviceCodeGrantType = "urn:ietf:params:oauth:grant-type:device_code"
	// refreshTokenGrantType is the OAuth 2.0 grant type for token refresh.
	refreshTokenGrantType = "refresh_token"
	// pollInterval is the default RFC 8628 polling interval in seconds.
	pollInterval = 5 * time.Second
	// tokenFileName is the filename for the stored Anthropic OAuth token.
	tokenFileName = "anthropic.json"
)

// OAuthToken represents the stored OAuth token data.
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// IsExpired returns true if the token is expired or will expire within 30 seconds.
func (t *OAuthToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt.Add(-30 * time.Second))
}

// deviceAuthResponse is the response from the device authorization endpoint.
type deviceAuthResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// tokenResponse is the response from the token endpoint.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// DeviceAuthFlow implements RFC 8628 OAuth 2.0 Device Authorization Grant.
type DeviceAuthFlow struct {
	// ClientID is the OAuth client ID. Falls back to ANTHROPIC_CLIENT_ID env var.
	ClientID string
	// DeviceAuthURL overrides the default device authorization endpoint (for testing).
	DeviceAuthURL string
	// TokenURL overrides the default token endpoint (for testing).
	TokenURL string
	// HTTPClient is the HTTP client to use. Defaults to http.DefaultClient.
	HTTPClient *http.Client
	// PollInterval overrides the RFC 8628 polling interval. Zero uses pollInterval default.
	PollInterval time.Duration
}

// newDeviceAuthFlow creates a DeviceAuthFlow with defaults resolved from env/config.
func newDeviceAuthFlow() *DeviceAuthFlow {
	clientID := os.Getenv("ANTHROPIC_CLIENT_ID")
	if clientID == "" {
		// Fall back to a well-known public client ID for the Anthropic Console.
		clientID = "anthropic-console"
	}
	return &DeviceAuthFlow{
		ClientID:      clientID,
		DeviceAuthURL: defaultDeviceAuthURL,
		TokenURL:      defaultTokenURL,
		HTTPClient:    http.DefaultClient,
	}
}

// deviceAuthURL returns the device authorization URL to use.
func (f *DeviceAuthFlow) deviceAuthURL() string {
	if f.DeviceAuthURL != "" {
		return f.DeviceAuthURL
	}
	return defaultDeviceAuthURL
}

// tokenURL returns the token URL to use.
func (f *DeviceAuthFlow) tokenURL() string {
	if f.TokenURL != "" {
		return f.TokenURL
	}
	return defaultTokenURL
}

// StartDeviceAuth initiates the device authorization flow.
// Returns device_code, user_code, and verification_uri.
func (f *DeviceAuthFlow) StartDeviceAuth(ctx context.Context) (*deviceAuthResponse, error) {
	form := url.Values{}
	form.Set("client_id", f.ClientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.deviceAuthURL(),
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create device auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := f.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device auth request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read device auth response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device auth failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var dar deviceAuthResponse
	if err := json.Unmarshal(body, &dar); err != nil {
		return nil, fmt.Errorf("parse device auth response: %w", err)
	}

	slog.Debug("Device auth started", "verification_uri", dar.VerificationURI, "user_code", dar.UserCode)
	return &dar, nil
}

// PollForToken polls the token endpoint until the user completes browser auth,
// the device code expires, or the context is cancelled.
func (f *DeviceAuthFlow) PollForToken(ctx context.Context, deviceCode string) (*OAuthToken, error) {
	interval := f.PollInterval
	if interval <= 0 {
		interval = pollInterval
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		tok, err := f.requestToken(ctx, deviceCode)
		if err != nil {
			return nil, err
		}
		if tok != nil {
			return tok, nil
		}
		// tok == nil means authorization_pending — keep polling.
	}
}

// requestToken makes a single token request. Returns (nil, nil) when the
// server responds with authorization_pending so the caller can continue polling.
func (f *DeviceAuthFlow) requestToken(ctx context.Context, deviceCode string) (*OAuthToken, error) {
	form := url.Values{}
	form.Set("grant_type", deviceCodeGrantType)
	form.Set("device_code", deviceCode)
	form.Set("client_id", f.ClientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.tokenURL(),
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := f.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	if tr.Error != "" {
		switch tr.Error {
		case "authorization_pending":
			slog.Debug("Authorization pending, continuing to poll")
			return nil, nil
		case "slow_down":
			// RFC 8628 §3.5: increase interval by 5 seconds.
			slog.Debug("Slow down requested by server")
			return nil, nil
		case "expired_token":
			return nil, errors.New("device code expired: please run login again")
		case "access_denied":
			return nil, errors.New("access denied: user declined the authorization request")
		default:
			return nil, fmt.Errorf("token error %q: %s", tr.Error, tr.ErrorDesc)
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	expiresIn := tr.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600 // default 1 hour
	}

	tok := &OAuthToken{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(expiresIn) * time.Second),
		TokenType:    tr.TokenType,
	}
	if tok.TokenType == "" {
		tok.TokenType = "bearer"
	}

	slog.Debug("OAuth token obtained", "expires_at", tok.ExpiresAt)
	return tok, nil
}

// RefreshToken exchanges a refresh token for a new access token.
func (f *DeviceAuthFlow) RefreshToken(ctx context.Context, refreshToken string) (*OAuthToken, error) {
	form := url.Values{}
	form.Set("grant_type", refreshTokenGrantType)
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", f.ClientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.tokenURL(),
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := f.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("parse refresh response: %w", err)
	}

	if tr.Error != "" {
		return nil, fmt.Errorf("refresh error %q: %s", tr.Error, tr.ErrorDesc)
	}

	expiresIn := tr.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}

	tok := &OAuthToken{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(expiresIn) * time.Second),
		TokenType:    tr.TokenType,
	}
	if tok.TokenType == "" {
		tok.TokenType = "bearer"
	}
	// If the server didn't return a new refresh token, keep the old one.
	if tok.RefreshToken == "" {
		tok.RefreshToken = refreshToken
	}

	slog.Debug("OAuth token refreshed", "expires_at", tok.ExpiresAt)
	return tok, nil
}

// tokenFilePath returns the path to the stored token file.
func tokenFilePath() string {
	return filepath.Join(config.HermesHome(), "tokens", tokenFileName)
}

// SaveToken writes the token to ~/.hermes/tokens/anthropic.json.
func SaveToken(tok *OAuthToken) error {
	dir := filepath.Join(config.HermesHome(), "tokens")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create token directory: %w", err)
	}

	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}

	path := tokenFilePath()
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write token file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename token file: %w", err)
	}

	slog.Debug("Token saved", "path", path)
	return nil
}

// LoadSavedToken loads and returns the stored token, or an error if none exists
// or the file is malformed.
func LoadSavedToken() (*OAuthToken, error) {
	data, err := os.ReadFile(tokenFilePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read token file: %w", err)
	}

	var tok OAuthToken
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, fmt.Errorf("parse token file: %w", err)
	}

	if tok.AccessToken == "" {
		return nil, errors.New("token file has no access_token")
	}

	return &tok, nil
}

// DeleteToken removes the stored token file.
func DeleteToken() error {
	path := tokenFilePath()
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// ResolveAnthropicAPIKey returns the Anthropic API key to use by checking,
// in order: config/env API key, saved OAuth token (refreshed if needed).
// Returns an empty string if no credentials are found.
func ResolveAnthropicAPIKey() string {
	// 1. Check env var / config (existing behavior, highest priority).
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	cfg := config.Load()
	if cfg.APIKey != "" {
		return cfg.APIKey
	}

	// 2. Try saved OAuth token.
	tok, err := LoadSavedToken()
	if err != nil {
		slog.Debug("Could not load saved token", "error", err)
		return ""
	}
	if tok == nil {
		return ""
	}

	// 3. Refresh if expired.
	if tok.IsExpired() && tok.RefreshToken != "" {
		slog.Debug("Token expired, attempting refresh")
		flow := newDeviceAuthFlow()
		newTok, err := flow.RefreshToken(context.Background(), tok.RefreshToken)
		if err != nil {
			slog.Debug("Token refresh failed", "error", err)
			return ""
		}
		if err := SaveToken(newTok); err != nil {
			slog.Debug("Failed to save refreshed token", "error", err)
		}
		return newTok.AccessToken
	}

	if tok.IsExpired() {
		slog.Debug("Token is expired and has no refresh token")
		return ""
	}

	return tok.AccessToken
}

// Login runs the full RFC 8628 device authorization flow interactively.
// It prints instructions, waits for the user to complete browser auth, then saves the token.
func Login(ctx context.Context) error {
	flow := newDeviceAuthFlow()

	dar, err := flow.StartDeviceAuth(ctx)
	if err != nil {
		return fmt.Errorf("start device authorization: %w", err)
	}

	fmt.Printf("\nTo authorize, visit:\n\n  %s\n\nAnd enter code: %s\n\nWaiting for authorization...\n",
		dar.VerificationURI, dar.UserCode)

	tok, err := flow.PollForToken(ctx, dar.DeviceCode)
	if err != nil {
		return fmt.Errorf("authorization failed: %w", err)
	}

	if err := SaveToken(tok); err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	fmt.Println("\nAuthorization successful. Token saved.")
	return nil
}

// Logout removes the stored Anthropic OAuth token.
func Logout() error {
	if err := DeleteToken(); err != nil {
		return fmt.Errorf("remove token: %w", err)
	}
	fmt.Println("Logged out. Anthropic OAuth token removed.")
	return nil
}
