package secrets

import (
	"context"
	"strings"
	"testing"
)

// Compile-time interface compliance check.
var _ SecretStore = (*EnvSecretStore)(nil)

func TestGet_ExistingEnvVar(t *testing.T) {
	store := NewEnvSecretStore("TEST_SECRET_")
	t.Setenv("TEST_SECRET_DB_PASS", "hunter2")

	val, err := store.Get(context.Background(), "DB_PASS")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if val != "hunter2" {
		t.Fatalf("expected %q, got %q", "hunter2", val)
	}
}

func TestGet_MissingEnvVar(t *testing.T) {
	store := NewEnvSecretStore("TEST_SECRET_")

	_, err := store.Get(context.Background(), "NONEXISTENT_KEY")
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
	if !strings.Contains(err.Error(), "not found in environment") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestGet_PrefixApplied(t *testing.T) {
	store := NewEnvSecretStore("MYAPP_")
	t.Setenv("MYAPP_TOKEN", "abc123")

	val, err := store.Get(context.Background(), "TOKEN")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if val != "abc123" {
		t.Fatalf("expected %q, got %q", "abc123", val)
	}
}

func TestGet_PrefixInErrorMessage(t *testing.T) {
	store := NewEnvSecretStore("PFX_")

	_, err := store.Get(context.Background(), "MISSING")
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
	if !strings.Contains(err.Error(), "PFX_MISSING") {
		t.Fatalf("expected full key PFX_MISSING in error, got: %v", err)
	}
}

func TestGet_EmptyPrefix(t *testing.T) {
	store := NewEnvSecretStore("")
	t.Setenv("BARE_KEY", "value123")

	val, err := store.Get(context.Background(), "BARE_KEY")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if val != "value123" {
		t.Fatalf("expected %q, got %q", "value123", val)
	}
}
