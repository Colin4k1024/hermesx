package cli

import (
	"log/slog"
	"sync"
)

// FastMode manages the /fast toggle for priority processing.
type FastMode struct {
	mu      sync.RWMutex
	enabled bool
}

var globalFastMode = &FastMode{}

// GlobalFastMode returns the global fast mode instance.
func GlobalFastMode() *FastMode { return globalFastMode }

// Toggle switches fast mode on/off and returns the new state.
func (f *FastMode) Toggle() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enabled = !f.enabled
	slog.Info("Fast mode toggled", "enabled", f.enabled)
	return f.enabled
}

// IsEnabled returns whether fast mode is active.
func (f *FastMode) IsEnabled() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.enabled
}

// Set explicitly sets the fast mode state.
func (f *FastMode) Set(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enabled = enabled
}
