package environments

import "sync"

// Environment defines the interface for command execution environments.
// Implementations include local execution, Docker containers, and SSH remote hosts.
type Environment interface {
	// Execute runs a command in the environment and returns its output.
	// timeout is in seconds. Returns stdout, stderr, exit code, and any error.
	Execute(command string, timeout int) (stdout, stderr string, exitCode int, err error)

	// IsAvailable checks if this environment is ready for use.
	IsAvailable() bool

	// Name returns the human-readable name of this environment.
	Name() string
}

// EnvironmentFactory creates an Environment from configuration parameters.
type EnvironmentFactory func(params map[string]string) (Environment, error)

// registryMu protects the environment factory registry.
// Registration happens in init() (single-goroutine) and reads happen at runtime.
var registryMu sync.RWMutex

// registry holds all registered environment factories.
var registry = map[string]EnvironmentFactory{}

// RegisterEnvironment registers an environment factory under a name.
// Safe to call from init() and at runtime.
func RegisterEnvironment(name string, factory EnvironmentFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = factory
}

// GetEnvironment creates an environment by name with the given parameters.
func GetEnvironment(name string, params map[string]string) (Environment, error) {
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		// Default to local
		return NewLocalEnvironment(), nil
	}
	return factory(params)
}

// ListEnvironments returns the names of all registered environments.
func ListEnvironments() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
