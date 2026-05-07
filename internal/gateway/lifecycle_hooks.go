package gateway

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Lifecycle hook type constants.
const (
	HookOnConnect      = "on_connect"
	HookOnDisconnect   = "on_disconnect"
	HookOnSessionStart = "on_session_start"
	HookOnSessionEnd   = "on_session_end"
	HookOnMediaReceive = "on_media_receive"
	HookOnMediaSend    = "on_media_send"
)

// LifecycleEvent carries context for a lifecycle hook invocation.
type LifecycleEvent struct {
	Type      string
	Platform  Platform
	Timestamp time.Time

	// SessionKey is set for session lifecycle events.
	SessionKey string

	// Source describes the message origin (session events).
	Source *SessionSource

	// MediaType is set for media lifecycle events.
	MediaType MediaType

	// MediaPath is set for media lifecycle events.
	MediaPath string

	// Error holds any error that caused the event (e.g., disconnect reason).
	Error error

	// Metadata for arbitrary key-value data.
	Metadata map[string]string
}

// LifecycleHookFunc is the function signature for lifecycle hook callbacks.
type LifecycleHookFunc func(ctx context.Context, event *LifecycleEvent) error

// LifecycleHookRegistration represents a registered lifecycle hook.
type LifecycleHookRegistration struct {
	Name     string
	Type     string
	Fn       LifecycleHookFunc
	Priority int
}

// LifecycleHooks manages platform and session lifecycle hooks.
type LifecycleHooks struct {
	mu       sync.RWMutex
	registry *HookRegistry
	hooks    map[string][]LifecycleHookRegistration
}

// NewLifecycleHooks creates a lifecycle hook manager. It optionally wraps
// an existing HookRegistry for unified hook discovery.
func NewLifecycleHooks(registry *HookRegistry) *LifecycleHooks {
	return &LifecycleHooks{
		registry: registry,
		hooks:    make(map[string][]LifecycleHookRegistration),
	}
}

// Register adds a lifecycle hook for the given type.
func (lh *LifecycleHooks) Register(hookType, name string, fn LifecycleHookFunc, priority int) {
	lh.mu.Lock()
	defer lh.mu.Unlock()

	reg := LifecycleHookRegistration{
		Name:     name,
		Type:     hookType,
		Fn:       fn,
		Priority: priority,
	}

	hooks := lh.hooks[hookType]
	inserted := false
	for i, existing := range hooks {
		if priority < existing.Priority {
			hooks = append(hooks[:i+1], hooks[i:]...)
			hooks[i] = reg
			inserted = true
			break
		}
	}
	if !inserted {
		hooks = append(hooks, reg)
	}
	lh.hooks[hookType] = hooks
}

// Fire executes all registered lifecycle hooks for a type in priority order.
func (lh *LifecycleHooks) Fire(ctx context.Context, event *LifecycleEvent) error {
	lh.mu.RLock()
	hooks := make([]LifecycleHookRegistration, len(lh.hooks[event.Type]))
	copy(hooks, lh.hooks[event.Type])
	lh.mu.RUnlock()

	if len(hooks) == 0 {
		return nil
	}

	var firstErr error
	for _, hook := range hooks {
		if err := hook.Fn(ctx, event); err != nil {
			slog.Warn("Lifecycle hook error",
				"hook_type", event.Type,
				"hook_name", hook.Name,
				"platform", event.Platform,
				"error", err,
			)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// HasHooks returns true if any hooks are registered for the given type.
func (lh *LifecycleHooks) HasHooks(hookType string) bool {
	lh.mu.RLock()
	defer lh.mu.RUnlock()
	return len(lh.hooks[hookType]) > 0
}

// HookCount returns the number of hooks registered for a type.
func (lh *LifecycleHooks) HookCount(hookType string) int {
	lh.mu.RLock()
	defer lh.mu.RUnlock()
	return len(lh.hooks[hookType])
}

// EmitConnect fires the on_connect lifecycle event.
func (lh *LifecycleHooks) EmitConnect(ctx context.Context, platform Platform) error {
	return lh.Fire(ctx, &LifecycleEvent{
		Type:      HookOnConnect,
		Platform:  platform,
		Timestamp: time.Now(),
	})
}

// EmitDisconnect fires the on_disconnect lifecycle event.
func (lh *LifecycleHooks) EmitDisconnect(ctx context.Context, platform Platform, reason error) error {
	return lh.Fire(ctx, &LifecycleEvent{
		Type:      HookOnDisconnect,
		Platform:  platform,
		Timestamp: time.Now(),
		Error:     reason,
	})
}

// EmitSessionStart fires the on_session_start lifecycle event.
func (lh *LifecycleHooks) EmitSessionStart(ctx context.Context, platform Platform, sessionKey string, source *SessionSource) error {
	return lh.Fire(ctx, &LifecycleEvent{
		Type:       HookOnSessionStart,
		Platform:   platform,
		SessionKey: sessionKey,
		Source:     source,
		Timestamp:  time.Now(),
	})
}

// EmitSessionEnd fires the on_session_end lifecycle event.
func (lh *LifecycleHooks) EmitSessionEnd(ctx context.Context, platform Platform, sessionKey string) error {
	return lh.Fire(ctx, &LifecycleEvent{
		Type:       HookOnSessionEnd,
		Platform:   platform,
		SessionKey: sessionKey,
		Timestamp:  time.Now(),
	})
}

// EmitMediaReceive fires the on_media_receive lifecycle event.
func (lh *LifecycleHooks) EmitMediaReceive(ctx context.Context, platform Platform, mediaType MediaType, path string) error {
	return lh.Fire(ctx, &LifecycleEvent{
		Type:      HookOnMediaReceive,
		Platform:  platform,
		MediaType: mediaType,
		MediaPath: path,
		Timestamp: time.Now(),
	})
}

// EmitMediaSend fires the on_media_send lifecycle event.
func (lh *LifecycleHooks) EmitMediaSend(ctx context.Context, platform Platform, mediaType MediaType, path string) error {
	return lh.Fire(ctx, &LifecycleEvent{
		Type:      HookOnMediaSend,
		Platform:  platform,
		MediaType: mediaType,
		MediaPath: path,
		Timestamp: time.Now(),
	})
}
