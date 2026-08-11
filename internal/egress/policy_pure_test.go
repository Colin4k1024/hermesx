package egress

import (
	"context"
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// NewAllowAllPolicy tests
// ---------------------------------------------------------------------------

func TestNewAllowAllPolicy_IsAllowed(t *testing.T) {
	p := NewAllowAllPolicy()
	ctx := context.Background()

	allowed, err := p.IsAllowed(ctx, "tenant-1", "any.host.com", "/any/path")
	if err != nil {
		t.Fatalf("IsAllowed: unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("AllowAllPolicy should always return true")
	}
}

func TestNewAllowAllPolicy_Reload(t *testing.T) {
	p := NewAllowAllPolicy()
	ctx := context.Background()

	if err := p.Reload(ctx); err != nil {
		t.Fatalf("Reload: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// NewDenyAllPolicy tests
// ---------------------------------------------------------------------------

func TestNewDenyAllPolicy_IsAllowed(t *testing.T) {
	p := NewDenyAllPolicy()
	ctx := context.Background()

	allowed, err := p.IsAllowed(ctx, "tenant-1", "any.host.com", "/any/path")
	if err != nil {
		t.Fatalf("IsAllowed: unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("DenyAllPolicy should always return false")
	}
}

func TestNewDenyAllPolicy_Reload(t *testing.T) {
	p := NewDenyAllPolicy()
	ctx := context.Background()

	if err := p.Reload(ctx); err != nil {
		t.Fatalf("Reload: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// NewLogOnlyPolicy tests
// ---------------------------------------------------------------------------

func TestNewLogOnlyPolicy_IsAllowed(t *testing.T) {
	p := NewLogOnlyPolicy()
	ctx := context.Background()

	allowed, err := p.IsAllowed(ctx, "tenant-1", "example.com", "/api/v1")
	if err != nil {
		t.Fatalf("IsAllowed: unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("LogOnlyPolicy should always return true")
	}
}

func TestNewLogOnlyPolicy_Reload(t *testing.T) {
	p := NewLogOnlyPolicy()
	ctx := context.Background()

	if err := p.Reload(ctx); err != nil {
		t.Fatalf("Reload: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// normalizePolicyToken tests (exercised directly since package-internal)
// ---------------------------------------------------------------------------

func TestNormalizePolicyToken(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"allow-all", "allow-all"},
		{"ALLOW-ALL", "allow-all"},
		{"Allow_All", "allow-all"},
		{"deny_all", "deny-all"},
		{"DENY_ALL", "deny-all"},
		{"log-only", "log-only"},
		{"LOG_ONLY", "log-only"},
		{"  deny-all  ", "deny-all"},
		{"production", "production"},
		{"PRODUCTION", "production"},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalizePolicyToken(tt.input)
		if got != tt.want {
			t.Errorf("normalizePolicyToken(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ResolveDefaultPolicy extended tests
// ---------------------------------------------------------------------------

func TestResolveDefaultPolicy_EnvironmentVariants(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		override    string
		want        DefaultPolicy
		wantErr     bool
	}{
		// Environment-based resolution
		{name: "production", environment: "production", want: DefaultDenyAll},
		{name: "prod", environment: "prod", want: DefaultDenyAll},
		{name: "PRODUCTION uppercase", environment: "PRODUCTION", want: DefaultDenyAll},
		{name: "PROD uppercase", environment: "PROD", want: DefaultDenyAll},
		{name: "development", environment: "development", want: DefaultAllowAll},
		{name: "staging", environment: "staging", want: DefaultAllowAll},
		{name: "test", environment: "test", want: DefaultAllowAll},
		{name: "empty env", environment: "", want: DefaultAllowAll},
		{name: "whitespace env", environment: "   ", want: DefaultAllowAll},

		// Override takes precedence
		{name: "override allow-all over production", environment: "production", override: "allow-all", want: DefaultAllowAll},
		{name: "override deny-all over development", environment: "development", override: "deny-all", want: DefaultDenyAll},
		{name: "override log-only", environment: "production", override: "log-only", want: DefaultLogOnly},
		{name: "override with underscore", environment: "", override: "allow_all", want: DefaultAllowAll},
		{name: "override with uppercase", environment: "", override: "DENY-ALL", want: DefaultDenyAll},
		{name: "override with mixed case underscore", environment: "", override: "Log_Only", want: DefaultLogOnly},

		// Invalid override
		{name: "invalid override", environment: "", override: "invalid", wantErr: true},
		{name: "invalid override partial", environment: "", override: "allow", wantErr: true},
		{name: "invalid override garbage", environment: "production", override: "xyz123", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveDefaultPolicy(tt.environment, tt.override)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveDefaultPolicyFromEnv tests
// ---------------------------------------------------------------------------

func TestResolveDefaultPolicyFromEnv(t *testing.T) {
	// Clear all environment variables that may interfere
	envKeys := append([]string{EnvDefaultPolicy}, defaultEnvironmentKeys...)
	originalValues := make(map[string]string)
	for _, key := range envKeys {
		originalValues[key] = os.Getenv(key)
		os.Unsetenv(key)
	}
	t.Cleanup(func() {
		for key, val := range originalValues {
			if val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	})

	t.Run("no env vars defaults to allow-all", func(t *testing.T) {
		got, err := ResolveDefaultPolicyFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != DefaultAllowAll {
			t.Fatalf("expected allow-all when no env set, got %s", got)
		}
	})

	t.Run("HERMES_EGRESS_DEFAULT override", func(t *testing.T) {
		os.Setenv(EnvDefaultPolicy, "deny-all")
		defer os.Unsetenv(EnvDefaultPolicy)

		got, err := ResolveDefaultPolicyFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != DefaultDenyAll {
			t.Fatalf("expected deny-all with HERMES_EGRESS_DEFAULT=deny-all, got %s", got)
		}
	})

	t.Run("HERMES_EGRESS_DEFAULT log-only", func(t *testing.T) {
		os.Setenv(EnvDefaultPolicy, "log-only")
		defer os.Unsetenv(EnvDefaultPolicy)

		got, err := ResolveDefaultPolicyFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != DefaultLogOnly {
			t.Fatalf("expected log-only, got %s", got)
		}
	})

	t.Run("HERMES_EGRESS_DEFAULT invalid", func(t *testing.T) {
		os.Setenv(EnvDefaultPolicy, "bogus")
		defer os.Unsetenv(EnvDefaultPolicy)

		_, err := ResolveDefaultPolicyFromEnv()
		if err == nil {
			t.Fatal("expected error for invalid override")
		}
	})

	t.Run("HERMES_ENV production fallback", func(t *testing.T) {
		os.Setenv("HERMES_ENV", "production")
		defer os.Unsetenv("HERMES_ENV")

		got, err := ResolveDefaultPolicyFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != DefaultDenyAll {
			t.Fatalf("expected deny-all with HERMES_ENV=production, got %s", got)
		}
	})

	t.Run("override takes precedence over env", func(t *testing.T) {
		os.Setenv("HERMES_ENV", "production")
		os.Setenv(EnvDefaultPolicy, "allow-all")
		defer os.Unsetenv("HERMES_ENV")
		defer os.Unsetenv(EnvDefaultPolicy)

		got, err := ResolveDefaultPolicyFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != DefaultAllowAll {
			t.Fatalf("expected allow-all (override wins), got %s", got)
		}
	})
}

// ---------------------------------------------------------------------------
// environmentFromEnv tests (exercised indirectly through ResolveDefaultPolicyFromEnv)
// ---------------------------------------------------------------------------

func TestEnvironmentFromEnv_Priority(t *testing.T) {
	// Clear all env vars
	envKeys := append([]string{EnvDefaultPolicy}, defaultEnvironmentKeys...)
	originalValues := make(map[string]string)
	for _, key := range envKeys {
		originalValues[key] = os.Getenv(key)
		os.Unsetenv(key)
	}
	t.Cleanup(func() {
		for key, val := range originalValues {
			if val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	})

	// HERMES_ENV is checked first
	os.Setenv("HERMES_ENV", "prod")
	os.Setenv("APP_ENV", "development")
	defer os.Unsetenv("HERMES_ENV")
	defer os.Unsetenv("APP_ENV")

	got, err := ResolveDefaultPolicyFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != DefaultDenyAll {
		t.Fatalf("HERMES_ENV should take priority, expected deny-all, got %s", got)
	}
}
