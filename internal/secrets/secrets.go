package secrets

import (
	"context"
	"fmt"
	"os"
)

// SecretStore abstracts secret retrieval from various backends.
type SecretStore interface {
	Get(ctx context.Context, key string) (string, error)
}

// EnvSecretStore reads secrets from environment variables (default backend).
type EnvSecretStore struct {
	prefix string
}

func NewEnvSecretStore(prefix string) *EnvSecretStore {
	return &EnvSecretStore{prefix: prefix}
}

func (s *EnvSecretStore) Get(_ context.Context, key string) (string, error) {
	fullKey := s.prefix + key
	val := os.Getenv(fullKey)
	if val == "" {
		return "", fmt.Errorf("secret %q not found in environment", fullKey)
	}
	return val, nil
}
