package bitwarden

import (
	"context"
	"fmt"

	"github.com/Colin4k1024/hermesx/internal/secrets"
)

// Provider is a secrets.SecretSource backed by Bitwarden Secrets Manager.
// It resolves secrets by their human-readable Key name within an organisation.
type Provider struct {
	orgID  string
	client BitwardenClient
}

// New creates a Provider using the default HTTP client.
// accessToken is a machine-account access token issued by Bitwarden.
// orgID is the UUID of the Bitwarden organisation.
func New(accessToken, orgID string) *Provider {
	return &Provider{
		orgID:  orgID,
		client: NewHTTPClient(accessToken, orgID),
	}
}

// NewWithClient creates a Provider with a custom BitwardenClient.
// This is the preferred constructor in tests to inject a mock.
func NewWithClient(orgID string, client BitwardenClient) *Provider {
	return &Provider{orgID: orgID, client: client}
}

// Name implements secrets.SecretSource.
func (p *Provider) Name() string { return "bitwarden" }

// Get finds the first secret whose Key matches the requested key and returns
// its plaintext value. Returns secrets.ErrNotFound when no match exists.
func (p *Provider) Get(ctx context.Context, key string) (string, error) {
	metas, err := p.client.ListSecrets(ctx, p.orgID)
	if err != nil {
		return "", fmt.Errorf("bitwarden: list error: %w", err)
	}
	for _, m := range metas {
		if m.Key == key {
			val, err := p.client.GetSecret(ctx, m.ID)
			if err != nil {
				return "", fmt.Errorf("bitwarden: get error for key %q: %w", key, err)
			}
			return val, nil
		}
	}
	return "", fmt.Errorf("%w: %q", secrets.ErrNotFound, key)
}

// List returns the Key names of all secrets in the organisation.
func (p *Provider) List(ctx context.Context) ([]string, error) {
	metas, err := p.client.ListSecrets(ctx, p.orgID)
	if err != nil {
		return nil, fmt.Errorf("bitwarden: list error: %w", err)
	}
	keys := make([]string, 0, len(metas))
	for _, m := range metas {
		keys = append(keys, m.Key)
	}
	return keys, nil
}
