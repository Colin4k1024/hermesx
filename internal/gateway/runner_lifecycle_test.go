package gateway

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/Colin4k1024/hermesx/internal/agent"
)

func TestRunner_WithLifecycleHooks_EmitsConnectDisconnect(t *testing.T) {
	lh := NewLifecycleHooks(nil)

	var connectFired, disconnectFired atomic.Bool
	lh.Register(HookOnConnect, "test-connect", func(_ context.Context, _ *LifecycleEvent) error {
		connectFired.Store(true)
		return nil
	}, 0)
	lh.Register(HookOnDisconnect, "test-disconnect", func(_ context.Context, _ *LifecycleEvent) error {
		disconnectFired.Store(true)
		return nil
	}, 0)

	ctx, cancel := context.WithCancel(context.Background())
	r := &Runner{
		adapters:      make(map[Platform]PlatformAdapter),
		delivery:      NewDeliveryRouter(),
		hooks:         NewHookRegistry(),
		pairing:       NewPairingStore(),
		status:        NewRuntimeStatus(),
		sessions:      NewSessionStore(&GatewayConfig{}),
		mediaCache:    NewMediaCache(),
		agentCache:    lru.NewLRU[string, *agent.AIAgent](10, nil, 5*time.Minute),
		adapterErrors: make(map[Platform]int),
		ctx:           ctx,
		cancel:        cancel,
	}
	r.WithLifecycleHooks(lh)

	adapter := &mediaTestAdapter{platform: PlatformTelegram}
	r.RegisterAdapter(adapter)

	if err := r.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Allow the goroutine to connect and fire the hook.
	deadline := time.Now().Add(200 * time.Millisecond)
	for !connectFired.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !connectFired.Load() {
		t.Error("HookOnConnect did not fire after connect")
	}

	// Cancel context — triggers disconnect and disconnect hook.
	cancel()

	deadline = time.Now().Add(200 * time.Millisecond)
	for !disconnectFired.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if !disconnectFired.Load() {
		t.Error("HookOnDisconnect did not fire after disconnect")
	}
}

func TestRunner_WithLifecycleHooks_NilSafe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &Runner{
		adapters:      make(map[Platform]PlatformAdapter),
		delivery:      NewDeliveryRouter(),
		hooks:         NewHookRegistry(),
		pairing:       NewPairingStore(),
		status:        NewRuntimeStatus(),
		sessions:      NewSessionStore(&GatewayConfig{}),
		mediaCache:    NewMediaCache(),
		agentCache:    lru.NewLRU[string, *agent.AIAgent](10, nil, 5*time.Minute),
		adapterErrors: make(map[Platform]int),
		ctx:           ctx,
		cancel:        cancel,
	}
	// No lifecycle hooks attached — Start must not panic.
	adapter := &mediaTestAdapter{platform: PlatformTelegram}
	r.RegisterAdapter(adapter)

	if err := r.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
}
