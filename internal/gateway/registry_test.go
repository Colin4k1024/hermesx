package gateway

import (
	"testing"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	r := &Registry{registrations: make(map[Platform]*PlatformRegistration)}

	reg := &PlatformRegistration{
		Platform:    PlatformTelegram,
		DisplayName: "Telegram",
		Factory: func(cfg *PlatformConfig) (PlatformAdapter, error) {
			return nil, nil
		},
		Capabilities: PlatformCapabilities{
			SupportsImages:   true,
			MaxMessageLength: 4096,
		},
	}

	r.Register(reg)

	got, ok := r.Get(PlatformTelegram)
	if !ok {
		t.Fatal("expected to find registered platform")
	}
	if got.DisplayName != "Telegram" {
		t.Errorf("got DisplayName=%q, want %q", got.DisplayName, "Telegram")
	}
	if !got.Capabilities.SupportsImages {
		t.Error("expected SupportsImages=true")
	}
}

func TestRegistryList(t *testing.T) {
	r := &Registry{registrations: make(map[Platform]*PlatformRegistration)}

	r.Register(&PlatformRegistration{Platform: PlatformTelegram, DisplayName: "Telegram"})
	r.Register(&PlatformRegistration{Platform: PlatformDiscord, DisplayName: "Discord"})
	r.Register(&PlatformRegistration{Platform: PlatformSlack, DisplayName: "Slack"})

	list := r.List()
	if len(list) != 3 {
		t.Errorf("got %d registrations, want 3", len(list))
	}
}

func TestRegistryCapabilities(t *testing.T) {
	r := &Registry{registrations: make(map[Platform]*PlatformRegistration)}

	r.Register(&PlatformRegistration{
		Platform: PlatformDiscord,
		Capabilities: PlatformCapabilities{
			MaxMessageLength: 2000,
			MaxImages:        10,
			SupportsThreads:  true,
		},
	})

	caps, ok := r.Capabilities(PlatformDiscord)
	if !ok {
		t.Fatal("expected to find capabilities")
	}
	if caps.MaxMessageLength != 2000 {
		t.Errorf("MaxMessageLength=%d, want 2000", caps.MaxMessageLength)
	}
	if caps.MaxImages != 10 {
		t.Errorf("MaxImages=%d, want 10", caps.MaxImages)
	}

	_, ok = r.Capabilities(Platform("unknown"))
	if ok {
		t.Error("expected unknown platform to return false")
	}
}

func TestRegistryAutoDiscover(t *testing.T) {
	r := &Registry{registrations: make(map[Platform]*PlatformRegistration)}

	called := false
	r.Register(&PlatformRegistration{
		Platform:    PlatformAPI,
		DisplayName: "API",
		Factory: func(cfg *PlatformConfig) (PlatformAdapter, error) {
			called = true
			return nil, nil
		},
	})

	gwCfg := &GatewayConfig{
		Platforms: map[Platform]*PlatformConfig{
			PlatformAPI: {Enabled: true, Settings: map[string]string{"port": "8080"}},
		},
	}

	adapters := r.AutoDiscover(gwCfg)
	if !called {
		t.Error("expected factory to be called")
	}
	// nil adapter is not appended (we return nil from factory)
	_ = adapters
}

func TestRegistryAutoDiscoverSkipsDisabled(t *testing.T) {
	r := &Registry{registrations: make(map[Platform]*PlatformRegistration)}

	called := false
	r.Register(&PlatformRegistration{
		Platform:    PlatformTelegram,
		DisplayName: "Telegram",
		Factory: func(cfg *PlatformConfig) (PlatformAdapter, error) {
			called = true
			return nil, nil
		},
	})

	gwCfg := &GatewayConfig{
		Platforms: map[Platform]*PlatformConfig{
			PlatformTelegram: {Enabled: false, Token: "xxx"},
		},
	}

	r.AutoDiscover(gwCfg)
	if called {
		t.Error("factory should not be called for disabled platform")
	}
}

func TestGlobalRegistrySingleton(t *testing.T) {
	r1 := GlobalRegistry()
	r2 := GlobalRegistry()
	if r1 != r2 {
		t.Error("expected same singleton instance")
	}
}
