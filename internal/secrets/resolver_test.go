package secrets

import (
	"context"
	"errors"
	"regexp"
	"testing"
)

// fakeResolver is a minimal SecretResolver used in tests.
type fakeResolver struct {
	data     map[string]string
	resolved map[string]string
}

func newFakeResolver(data map[string]string) *fakeResolver {
	return &fakeResolver{data: data, resolved: make(map[string]string)}
}

func (f *fakeResolver) Resolve(_ context.Context, name string) (string, error) {
	v, ok := f.data[name]
	if !ok {
		return "", errors.New("not found: " + name)
	}
	f.resolved[name] = v
	return v, nil
}

func (f *fakeResolver) RegisterPattern(_ string, _ *regexp.Regexp) {}
func (f *fakeResolver) ListRegistered() []string                   { return nil }
func (f *fakeResolver) ResolvedValues() map[string]string          { return f.resolved }

// Compile-time: restrictedResolver must satisfy SecretResolver.
var _ SecretResolver = (*restrictedResolver)(nil)

func TestWithAllowedKeys_NilReturnsOriginal(t *testing.T) {
	base := newFakeResolver(nil)
	got := WithAllowedKeys(base, nil)
	if got != base {
		t.Fatal("expected original resolver returned when keys is nil")
	}
}

func TestWithAllowedKeys_EmptyReturnsOriginal(t *testing.T) {
	base := newFakeResolver(nil)
	got := WithAllowedKeys(base, []string{})
	if got != base {
		t.Fatal("expected original resolver returned when keys is empty")
	}
}

func TestWithAllowedKeys_AllowedKeyResolves(t *testing.T) {
	base := newFakeResolver(map[string]string{"DB_PASS": "secret"})
	r := WithAllowedKeys(base, []string{"DB_PASS"})

	val, err := r.Resolve(context.Background(), "DB_PASS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "secret" {
		t.Fatalf("expected %q, got %q", "secret", val)
	}
}

func TestWithAllowedKeys_DisallowedKeyReturnsError(t *testing.T) {
	base := newFakeResolver(map[string]string{"DB_PASS": "secret", "API_KEY": "tok"})
	r := WithAllowedKeys(base, []string{"DB_PASS"})

	_, err := r.Resolve(context.Background(), "API_KEY")
	if err == nil {
		t.Fatal("expected error for disallowed key, got nil")
	}
	if !errors.Is(err, ErrKeyNotAllowed) {
		t.Fatalf("expected ErrKeyNotAllowed, got: %v", err)
	}
}

func TestWithAllowedKeys_ErrorMessageContainsKey(t *testing.T) {
	base := newFakeResolver(nil)
	r := WithAllowedKeys(base, []string{"ALLOWED"})

	_, err := r.Resolve(context.Background(), "FORBIDDEN")
	if err == nil {
		t.Fatal("expected error")
	}
	const want = "FORBIDDEN"
	if !containsStr(err.Error(), want) {
		t.Fatalf("expected key name %q in error %q", want, err.Error())
	}
}

func TestWithAllowedKeys_DelegatesListRegistered(t *testing.T) {
	base := newFakeResolver(nil)
	base.RegisterPattern("pat", regexp.MustCompile(`\d+`))
	r := WithAllowedKeys(base, []string{"X"})
	// restrictedResolver.ListRegistered() must delegate to inner
	_ = r.ListRegistered() // must not panic
}

func TestWithAllowedKeys_DelegatesResolvedValues(t *testing.T) {
	base := newFakeResolver(map[string]string{"DB_PASS": "s"})
	r := WithAllowedKeys(base, []string{"DB_PASS"})
	_, _ = r.Resolve(context.Background(), "DB_PASS")
	vals := r.ResolvedValues()
	if vals["DB_PASS"] != "s" {
		t.Fatalf("ResolvedValues not delegated correctly: %v", vals)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
