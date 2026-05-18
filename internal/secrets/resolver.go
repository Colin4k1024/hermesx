package secrets

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"sync"
)

// ErrKeyNotAllowed is returned when a tool handler attempts to resolve a secret
// key that is not in the allowed set configured for the current ToolContext.
var ErrKeyNotAllowed = errors.New("secret key not in allowed set")

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
	maps.Copy(out, r.resolved)
	return out
}

// WithAllowedKeys returns a SecretResolver that delegates to r but only allows
// resolving the given keys. If keys is nil or empty the original resolver is
// returned unchanged (backward-compatible, unrestricted) — a warning is logged
// because callers almost certainly want an explicit allowlist (M-6).
func WithAllowedKeys(r SecretResolver, keys []string) SecretResolver {
	if len(keys) == 0 {
		slog.Warn("secrets: WithAllowedKeys called with empty key list — resolver is unrestricted; consider passing an explicit allowlist")
		return r
	}
	allowed := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		allowed[k] = struct{}{}
	}
	return &restrictedResolver{inner: r, allowed: allowed}
}

// restrictedResolver wraps a SecretResolver and enforces an allowlist of key
// names. All other SecretResolver methods are forwarded verbatim so the
// wrapper is transparent to callers that only use registration / listing APIs.
type restrictedResolver struct {
	inner   SecretResolver
	allowed map[string]struct{}
}

func (r *restrictedResolver) Resolve(ctx context.Context, name string) (string, error) {
	if _, ok := r.allowed[name]; !ok {
		return "", fmt.Errorf("%w: %q", ErrKeyNotAllowed, name)
	}
	return r.inner.Resolve(ctx, name)
}

func (r *restrictedResolver) RegisterPattern(name string, pattern *regexp.Regexp) {
	r.inner.RegisterPattern(name, pattern)
}

func (r *restrictedResolver) ListRegistered() []string {
	return r.inner.ListRegistered()
}

// ResolvedValues returns only the keys that fall within the allowed set (B-3).
func (r *restrictedResolver) ResolvedValues() map[string]string {
	all := r.inner.ResolvedValues()
	out := make(map[string]string, len(r.allowed))
	for k := range r.allowed {
		if v, ok := all[k]; ok {
			out[k] = v
		}
	}
	return out
}
