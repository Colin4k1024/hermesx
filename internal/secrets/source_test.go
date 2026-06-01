package secrets_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/secrets"
)

// staticSource is a trivial SecretSource for testing.
type staticSource struct {
	name  string
	store map[string]string
}

func (s *staticSource) Name() string { return s.name }
func (s *staticSource) Get(_ context.Context, key string) (string, error) {
	if val, ok := s.store[key]; ok {
		return val, nil
	}
	return "", secrets.ErrNotFound
}
func (s *staticSource) List(_ context.Context) ([]string, error) {
	keys := make([]string, 0, len(s.store))
	for k := range s.store {
		keys = append(keys, k)
	}
	return keys, nil
}

func TestChain_GetFirstWins(t *testing.T) {
	a := &staticSource{name: "a", store: map[string]string{"KEY": "from_a"}}
	b := &staticSource{name: "b", store: map[string]string{"KEY": "from_b"}}
	c := secrets.NewChain(a, b)

	val, err := c.Get(context.Background(), "KEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "from_a" {
		t.Fatalf("expected 'from_a', got %q", val)
	}
}

func TestChain_GetFallsThrough(t *testing.T) {
	a := &staticSource{name: "a", store: map[string]string{}}
	b := &staticSource{name: "b", store: map[string]string{"KEY": "from_b"}}
	c := secrets.NewChain(a, b)

	val, err := c.Get(context.Background(), "KEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "from_b" {
		t.Fatalf("expected 'from_b', got %q", val)
	}
}

func TestChain_GetNotFound(t *testing.T) {
	a := &staticSource{name: "a", store: map[string]string{}}
	c := secrets.NewChain(a)

	_, err := c.Get(context.Background(), "MISSING")
	if !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestChain_GetPropagatesUnexpectedError(t *testing.T) {
	errBoom := errors.New("boom")
	bad := &errSource{err: errBoom}
	c := secrets.NewChain(bad)

	_, err := c.Get(context.Background(), "KEY")
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected wrapped errBoom, got %v", err)
	}
}

func TestChain_ListDeduplicates(t *testing.T) {
	a := &staticSource{name: "a", store: map[string]string{"A": "1", "SHARED": "from_a"}}
	b := &staticSource{name: "b", store: map[string]string{"B": "2", "SHARED": "from_b"}}
	c := secrets.NewChain(a, b)

	keys, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	seen := map[string]int{}
	for _, k := range keys {
		seen[k]++
	}
	for k, n := range seen {
		if n > 1 {
			t.Errorf("key %q appears %d times (expected 1)", k, n)
		}
	}
	if len(seen) != 3 { // A, B, SHARED
		t.Fatalf("expected 3 unique keys, got %d: %v", len(seen), keys)
	}
}

func TestChain_Name(t *testing.T) {
	c := secrets.NewChain(&staticSource{name: "only", store: nil})
	if c.Name() != "chain" {
		t.Fatalf("expected 'chain', got %q", c.Name())
	}
}

// errSource returns a non-ErrNotFound error to test propagation.
type errSource struct{ err error }

func (e *errSource) Name() string                              { return "err_source" }
func (e *errSource) Get(_ context.Context, _ string) (string, error) { return "", e.err }
func (e *errSource) List(_ context.Context) ([]string, error)  { return nil, e.err }
