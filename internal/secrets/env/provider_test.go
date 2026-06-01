package env
package env_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/secrets"
	"github.com/Colin4k1024/hermesx/internal/secrets/env"
)

func TestEnvProvider_Get(t *testing.T) {
	t.Setenv("APP_TOKEN", "secret123")

	p := env.New("APP_")
	val, err := p.Get(context.Background(), "TOKEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "secret123" {
		t.Fatalf("expected 'secret123', got %q", val)
	}
}

func TestEnvProvider_GetNotFound(t *testing.T) {
	p := env.New("APP_")
	_, err := p.Get(context.Background(), "DEFINITELY_MISSING_XYZ123")
	if !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestEnvProvider_GetEmptyValue(t *testing.T) {
	t.Setenv("APP_EMPTY", "")

	p := env.New("APP_")
	_, err := p.Get(context.Background(), "EMPTY")
	if !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("empty env var should be treated as not found, got %v", err)
	}
}

func TestEnvProvider_NoPrefix(t *testing.T) {
	t.Setenv("RAW_SECRET", "rawval")

	p := env.New("")
	val, err := p.Get(context.Background(), "RAW_SECRET")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "rawval" {
		t.Fatalf("expected 'rawval', got %q", val)
	}
}

func TestEnvProvider_List(t *testing.T) {
	t.Setenv("MYAPP_FOO", "1")
	t.Setenv("MYAPP_BAR", "2")

	p := env.New("MYAPP_")
	keys, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := map[string]bool{}
	for _, k := range keys {
		found[k] = true
	}
	for _, want := range []string{"FOO", "BAR"} {
		if !found[want] {
			t.Errorf("expected key %q in List output, got %v", want, keys)
		}
	}
	// Keys must NOT contain the prefix.
	for _, k := range keys {
		if strings.HasPrefix(k, "MYAPP_") {
			t.Errorf("List returned key with prefix still attached: %q", k)
		}
	}
}

func TestEnvProvider_Name(t *testing.T) {
	p := env.New("")
	if p.Name() != "env" {
		t.Fatalf("expected 'env', got %q", p.Name())
	}
}
