// Package secrets provides an abstraction layer for retrieving secrets from
// various backends (environment variables, Bitwarden Secrets Manager, Vault…).
//
// # Quick start
//
//	chain := secrets.NewChain(
//	    env.New("APP_"),
//	    bitwarden.New(accessToken, orgID),
//	)
//	val, err := chain.Get(ctx, "DB_PASSWORD")
package secrets

import (
	"context"
	"errors"
	"fmt"
)

// ErrNotFound is returned by Get when no source in the chain holds the key.
var ErrNotFound = errors.New("secrets: key not found")

// SecretSource is the core interface for a single secret backend.
type SecretSource interface {
	// Name returns a human-readable label used in logs and error messages.
	Name() string

	// Get retrieves the plaintext value of key from this source.
	// Returns ErrNotFound when the key does not exist in this source.
	Get(ctx context.Context, key string) (string, error)

	// List returns all key names available in this source.
	List(ctx context.Context) ([]string, error)
}

// Chain resolves secrets through an ordered list of SecretSource backends
// using a first-match strategy.
type Chain struct {
	sources []SecretSource
}

// NewChain creates a Chain that queries each source in order.
// At least one source is required.
func NewChain(sources ...SecretSource) *Chain {
	if len(sources) == 0 {
		panic("secrets.NewChain: at least one source is required")
	}
	return &Chain{sources: sources}
}

// Name implements SecretSource.
func (c *Chain) Name() string { return "chain" }

// Get returns the first value found across all sources.
// If no source holds the key, ErrNotFound is returned.
// Non-ErrNotFound errors from a source are returned immediately.
func (c *Chain) Get(ctx context.Context, key string) (string, error) {
	for _, s := range c.sources {
		val, err := s.Get(ctx, key)
		if err == nil {
			return val, nil
		}
		if errors.Is(err, ErrNotFound) {
			continue
		}
		// Propagate unexpected errors immediately to avoid silent fallthrough.
		return "", fmt.Errorf("secrets: source %q error for key %q: %w", s.Name(), key, err)
	}
	return "", fmt.Errorf("%w: %q", ErrNotFound, key)
}

// List returns the deduplicated union of all keys available across all sources.
func (c *Chain) List(ctx context.Context) ([]string, error) {
	seen := map[string]struct{}{}
	var result []string
	for _, s := range c.sources {
		keys, err := s.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("secrets: source %q list error: %w", s.Name(), err)
		}
		for _, k := range keys {
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				result = append(result, k)
			}
		}
	}
	return result, nil
}
