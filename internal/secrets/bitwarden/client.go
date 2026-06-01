// Package bitwarden provides a secrets.SecretSource backed by
// Bitwarden Secrets Manager (https://bitwarden.com/products/secrets-manager/).
//
// # Design
//
// BitwardenClient is a narrow interface so the provider can be tested with a
// mock without an HTTP server. The default implementation (httpClient) calls
// the Bitwarden Secrets Manager REST API using a machine-account access token.
//
// # Usage
//
//	p := bitwarden.New(accessToken, orgID)      // production — uses HTTP client
//	chain := secrets.NewChain(p)
//	val, err := chain.Get(ctx, "my-secret-name")
package bitwarden

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.bitwarden.com/secrets/v1"

// SecretMeta holds the metadata of a single Bitwarden secret.
type SecretMeta struct {
	// ID is the Bitwarden UUID of the secret.
	ID string
	// Key is the human-readable name of the secret.
	Key string
}

// BitwardenClient is the interface the Provider uses to talk to the API.
// Swapping it with a test double avoids live HTTP calls in unit tests.
type BitwardenClient interface {
	// GetSecret retrieves the plaintext value of the secret identified by id.
	GetSecret(ctx context.Context, id string) (value string, err error)
	// ListSecrets returns all secrets visible to the configured org.
	ListSecrets(ctx context.Context, orgID string) ([]SecretMeta, error)
}

// httpClient is the default BitwardenClient backed by net/http.
type httpClient struct {
	baseURL     string
	accessToken string
	orgID       string
	http        *http.Client
}

// NewHTTPClient returns a production BitwardenClient.
func NewHTTPClient(accessToken, orgID string) BitwardenClient {
	return &httpClient{
		baseURL:     defaultBaseURL,
		accessToken: accessToken,
		orgID:       orgID,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetSecret fetches a single secret value by UUID.
func (c *httpClient) GetSecret(ctx context.Context, id string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/"+id, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("bitwarden: HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("bitwarden: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("bitwarden: failed to decode response: %w", err)
	}
	return payload.Value, nil
}

// ListSecrets returns metadata for all secrets in the configured organisation.
func (c *httpClient) ListSecrets(ctx context.Context, orgID string) ([]SecretMeta, error) {
	url := fmt.Sprintf("%s/list/secrets/%s", c.baseURL, orgID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitwarden: HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("bitwarden: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Data []struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("bitwarden: failed to decode list response: %w", err)
	}

	result := make([]SecretMeta, 0, len(payload.Data))
	for _, item := range payload.Data {
		result = append(result, SecretMeta{ID: item.ID, Key: item.Key})
	}
	return result, nil
}
