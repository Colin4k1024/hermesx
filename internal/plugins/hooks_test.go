package plugins

import (
	"errors"
	"testing"
)

func TestGlobalPluginHooks_ReturnsNonNil(t *testing.T) {
	r := GlobalPluginHooks()
	if r == nil {
		t.Fatal("GlobalPluginHooks() returned nil")
	}
}

func TestRegister_AndHasHooks(t *testing.T) {
	r := &PluginHookRegistry{hooks: make(map[string][]registeredHook)}
	t.Cleanup(func() { r.Clear() })

	r.Register(HookPreToolCall, "test-hook", 10, func(event *HookEvent) error {
		return nil
	})

	if !r.HasHooks(HookPreToolCall) {
		t.Fatal("HasHooks should return true for registered hook type")
	}
	if r.HasHooks(HookPostToolCall) {
		t.Fatal("HasHooks should return false for unregistered hook type")
	}
	if r.HasHooks(HookPreLLMCall) {
		t.Fatal("HasHooks should return false for unregistered hook type")
	}
}

func TestFire_Success(t *testing.T) {
	r := &PluginHookRegistry{hooks: make(map[string][]registeredHook)}
	t.Cleanup(func() { r.Clear() })

	called := false
	r.Register(HookPostToolCall, "success-hook", 10, func(event *HookEvent) error {
		called = true
		return nil
	})

	event := &HookEvent{ToolName: "test-tool"}
	err := r.Fire(HookPostToolCall, event)
	if err != nil {
		t.Fatalf("Fire returned unexpected error: %v", err)
	}
	if !called {
		t.Fatal("hook was not called")
	}
	if event.Type != HookPostToolCall {
		t.Fatalf("event.Type = %q, want %q", event.Type, HookPostToolCall)
	}
}

func TestFire_Error(t *testing.T) {
	r := &PluginHookRegistry{hooks: make(map[string][]registeredHook)}
	t.Cleanup(func() { r.Clear() })

	expectedErr := errors.New("hook failed")
	r.Register(HookPreLLMCall, "failing-hook", 10, func(event *HookEvent) error {
		return expectedErr
	})

	event := &HookEvent{}
	err := r.Fire(HookPreLLMCall, event)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("Fire returned %v, want %v", err, expectedErr)
	}
}

func TestFire_PriorityOrdering(t *testing.T) {
	r := &PluginHookRegistry{hooks: make(map[string][]registeredHook)}
	t.Cleanup(func() { r.Clear() })

	var callOrder []string

	// Register higher priority (larger number) first
	r.Register(HookSessionStart, "high-priority", 100, func(event *HookEvent) error {
		callOrder = append(callOrder, "high")
		return nil
	})
	// Register lower priority (smaller number) second
	r.Register(HookSessionStart, "low-priority", 1, func(event *HookEvent) error {
		callOrder = append(callOrder, "low")
		return nil
	})

	event := &HookEvent{}
	err := r.Fire(HookSessionStart, event)
	if err != nil {
		t.Fatalf("Fire returned unexpected error: %v", err)
	}

	if len(callOrder) != 2 {
		t.Fatalf("expected 2 hooks called, got %d", len(callOrder))
	}
	// Lower priority number should execute first
	if callOrder[0] != "low" {
		t.Fatalf("expected low priority hook first, got %q", callOrder[0])
	}
	if callOrder[1] != "high" {
		t.Fatalf("expected high priority hook second, got %q", callOrder[1])
	}
}

func TestFire_ErrorStopsExecution(t *testing.T) {
	r := &PluginHookRegistry{hooks: make(map[string][]registeredHook)}
	t.Cleanup(func() { r.Clear() })

	var callOrder []string

	r.Register(HookPostLLMCall, "first", 1, func(event *HookEvent) error {
		callOrder = append(callOrder, "first")
		return errors.New("stop here")
	})
	r.Register(HookPostLLMCall, "second", 2, func(event *HookEvent) error {
		callOrder = append(callOrder, "second")
		return nil
	})

	event := &HookEvent{}
	_ = r.Fire(HookPostLLMCall, event)

	if len(callOrder) != 1 {
		t.Fatalf("expected only 1 hook called (error should stop), got %d", len(callOrder))
	}
	if callOrder[0] != "first" {
		t.Fatalf("expected first hook to be the one called, got %q", callOrder[0])
	}
}

func TestClear(t *testing.T) {
	r := &PluginHookRegistry{hooks: make(map[string][]registeredHook)}

	r.Register(HookPreToolCall, "hook-a", 10, func(event *HookEvent) error { return nil })
	r.Register(HookSessionEnd, "hook-b", 20, func(event *HookEvent) error { return nil })

	if !r.HasHooks(HookPreToolCall) {
		t.Fatal("expected hooks to exist before clear")
	}

	r.Clear()

	if r.HasHooks(HookPreToolCall) {
		t.Fatal("HasHooks should return false after Clear")
	}
	if r.HasHooks(HookSessionEnd) {
		t.Fatal("HasHooks should return false after Clear")
	}
}

func TestFire_NoHooksRegistered(t *testing.T) {
	r := &PluginHookRegistry{hooks: make(map[string][]registeredHook)}

	event := &HookEvent{ToolName: "some-tool"}
	err := r.Fire(HookPreToolCall, event)
	if err != nil {
		t.Fatalf("Fire with no hooks should return nil, got %v", err)
	}
}

func TestNewPluginHookRegistry(t *testing.T) {
	r := &PluginHookRegistry{hooks: make(map[string][]registeredHook)}

	if r == nil {
		t.Fatal("new registry should not be nil")
	}
	if r.HasHooks(HookPreToolCall) {
		t.Fatal("new registry should have no hooks")
	}

	r.Register(HookPreToolCall, "test", 5, func(event *HookEvent) error { return nil })
	if !r.HasHooks(HookPreToolCall) {
		t.Fatal("registry should have hooks after Register")
	}
}

func TestGlobalPluginHooks_RegisterAndClear(t *testing.T) {
	r := GlobalPluginHooks()
	t.Cleanup(func() { r.Clear() })

	r.Register(HookSessionEnd, "global-test", 50, func(event *HookEvent) error { return nil })
	if !r.HasHooks(HookSessionEnd) {
		t.Fatal("global registry should have the registered hook")
	}

	r.Clear()
	if r.HasHooks(HookSessionEnd) {
		t.Fatal("global registry should be empty after Clear")
	}
}
