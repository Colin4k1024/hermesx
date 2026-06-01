package env
// Package env provides a secrets.SecretSource backed by environment variables.
//
// Usage:
//
//	p := env.New("APP_")            // reads os.Getenv("APP_" + key)
//	val, err := p.Get(ctx, "TOKEN")  // → os.Getenv("APP_TOKEN")
package env

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Colin4k1024/hermesx/internal/secrets"
)

// Provider resolves secrets from environment variables with an optional prefix.
type Provider struct {
	prefix string
}

// New creates a Provider that looks up os.Getenv(prefix + key).
// prefix may be empty to read variables without any prefix.
func New(prefix string) *Provider {
	return &Provider{prefix: prefix}
}

// Name implements secrets.SecretSource.
func (p *Provider) Name() string { return "env" }

// Get returns the value of the environment variable "prefix + key".
// Returns secrets.ErrNotFound when the variable is unset or empty.
func (p *Provider) Get(_ context.Context, key string) (string, error) {
	val, ok := os.LookupEnv(p.prefix + key)
	if !ok || val == "" {
		return "", fmt.Errorf("%w: %q", secrets.ErrNotFound, p.prefix+key)
	}
	return val, nil
}

// List returns all keys visible in the current environment that match the
// configured prefix. The returned keys have the prefix stripped so callers
// can use them symmetrically with Get.
func (p *Provider) List(_ context.Context) ([]string, error) {
	var keys []string
	for _, entry := range os.Environ() {
		k, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if p.prefix == "" || strings.HasPrefix(k, p.prefix) {
			keys = append(keys, strings.TrimPrefix(k, p.prefix))
		}
	}
	return keys, nil
}
