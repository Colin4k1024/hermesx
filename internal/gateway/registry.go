package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// PlatformFactory creates a PlatformAdapter from configuration.
type PlatformFactory func(cfg *PlatformConfig) (PlatformAdapter, error)

// PlatformCapabilities declares what a platform supports.
type PlatformCapabilities struct {
	SupportsImages    bool
	SupportsVideo     bool
	SupportsVoice     bool
	SupportsDocuments bool
	SupportsStickers  bool
	SupportsThreads   bool
	SupportsReactions bool
	SupportsEdits     bool
	MaxMessageLength  int
	MaxImages         int // 0 = unlimited
}

// PlatformRegistration holds metadata for a registered platform.
type PlatformRegistration struct {
	Platform     Platform
	DisplayName  string
	Factory      PlatformFactory
	Capabilities PlatformCapabilities
	EnvVars      []string // required env vars to auto-detect
}

// Registry holds all registered platform factories.
type Registry struct {
	mu            sync.RWMutex
	registrations map[Platform]*PlatformRegistration
}

var (
	globalRegistry     *Registry
	globalRegistryOnce sync.Once
)

// GlobalRegistry returns the singleton platform registry.
func GlobalRegistry() *Registry {
	globalRegistryOnce.Do(func() {
		globalRegistry = &Registry{
			registrations: make(map[Platform]*PlatformRegistration),
		}
	})
	return globalRegistry
}

// Register adds a platform to the registry. Typically called from init().
func (r *Registry) Register(reg *PlatformRegistration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.registrations[reg.Platform]; exists {
		slog.Warn("Platform already registered, overwriting", "platform", reg.Platform)
	}
	r.registrations[reg.Platform] = reg
}

// Get returns a platform registration by identifier.
func (r *Registry) Get(p Platform) (*PlatformRegistration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.registrations[p]
	return reg, ok
}

// List returns all registered platforms.
func (r *Registry) List() []*PlatformRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*PlatformRegistration, 0, len(r.registrations))
	for _, reg := range r.registrations {
		result = append(result, reg)
	}
	return result
}

// Instantiate creates a PlatformAdapter for the given platform using its factory.
func (r *Registry) Instantiate(p Platform, cfg *PlatformConfig) (PlatformAdapter, error) {
	r.mu.RLock()
	reg, ok := r.registrations[p]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("platform %q not registered", p)
	}
	return reg.Factory(cfg)
}

// Capabilities returns the capabilities for a platform.
func (r *Registry) Capabilities(p Platform) (PlatformCapabilities, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.registrations[p]
	if !ok {
		return PlatformCapabilities{}, false
	}
	return reg.Capabilities, true
}

// RegisterPlatform is a convenience function for package-level registration.
func RegisterPlatform(reg *PlatformRegistration) {
	GlobalRegistry().Register(reg)
}

// AutoDiscover creates adapters for all platforms that have required env vars set.
func (r *Registry) AutoDiscover(gwCfg *GatewayConfig) []PlatformAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var adapters []PlatformAdapter
	for platform, reg := range r.registrations {
		pcfg, configured := gwCfg.Platforms[platform]
		if !configured || !pcfg.Enabled {
			continue
		}
		adapter, err := reg.Factory(pcfg)
		if err != nil {
			slog.Warn("Failed to create platform adapter", "platform", platform, "error", err)
			continue
		}
		adapters = append(adapters, adapter)
		slog.Info("Auto-discovered platform", "platform", platform, "name", reg.DisplayName)
	}
	return adapters
}

// DiscoverAndRegister creates adapters from config and registers them with the runner.
func DiscoverAndRegister(ctx context.Context, runner *Runner, gwCfg *GatewayConfig) int {
	adapters := GlobalRegistry().AutoDiscover(gwCfg)
	for _, adapter := range adapters {
		runner.RegisterAdapter(adapter)
	}
	return len(adapters)
}
