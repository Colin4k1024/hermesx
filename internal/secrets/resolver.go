package secrets

import (
	"context"
	"fmt"
	"regexp"
	"sync"
)

type SecretResolver interface {
	Resolve(ctx context.Context, name string) (string, error)
	RegisterPattern(name string, pattern *regexp.Regexp)
	ListRegistered() []string
	ResolvedValues() map[string]string
}

type EnvSecretResolver struct {
	store    SecretStore
	mu       sync.RWMutex
	resolved map[string]string
	patterns map[string]*regexp.Regexp
}

func NewEnvSecretResolver(store SecretStore) *EnvSecretResolver {
	return &EnvSecretResolver{
		store:    store,
		resolved: make(map[string]string),
		patterns: make(map[string]*regexp.Regexp),
	}
}

func (r *EnvSecretResolver) Resolve(ctx context.Context, name string) (string, error) {
	val, err := r.store.Get(ctx, name)
	if err != nil {
		return "", fmt.Errorf("resolve secret %q: %w", name, err)
	}

	r.mu.Lock()
	r.resolved[name] = val
	r.mu.Unlock()

	return val, nil
}

func (r *EnvSecretResolver) RegisterPattern(name string, pattern *regexp.Regexp) {
	r.mu.Lock()
	r.patterns[name] = pattern
	r.mu.Unlock()
}

func (r *EnvSecretResolver) ListRegistered() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.patterns))
	for name := range r.patterns {
		names = append(names, name)
	}
	return names
}

func (r *EnvSecretResolver) ResolvedValues() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make(map[string]string, len(r.resolved))
	for k, v := range r.resolved {
		out[k] = v
	}
	return out
}
