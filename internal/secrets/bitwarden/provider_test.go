package bitwarden_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Colin4k1024/hermesx/internal/secrets"
	"github.com/Colin4k1024/hermesx/internal/secrets/bitwarden"
)

// mockClient implements BitwardenClient for testing.
type mockClient struct {
	secrets []bitwarden.SecretMeta
	values  map[string]string
	listErr error
	getErr  map[string]error
}

func (m *mockClient) ListSecrets(_ context.Context, _ string) ([]bitwarden.SecretMeta, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.secrets, nil
}

func (m *mockClient) GetSecret(_ context.Context, id string) (string, error) {
	if m.getErr != nil {
		if err, ok := m.getErr[id]; ok {
			return "", err
		}
	}
	if val, ok := m.values[id]; ok {
		return val, nil
	}
	return "", errors.New("mock: id not found")
}

func newMock(kvs ...string) *mockClient {
	m := &mockClient{
		values: map[string]string{},
		getErr: map[string]error{},
	}
	// kvs: id, key, value, id, key, value, …
	for i := 0; i+2 < len(kvs); i += 3 {
		id, key, val := kvs[i], kvs[i+1], kvs[i+2]
		m.secrets = append(m.secrets, bitwarden.SecretMeta{ID: id, Key: key})
		m.values[id] = val
	}
	return m
}

func TestBitwardenProvider_Get(t *testing.T) {
	mock := newMock("uuid-1", "DB_PASSWORD", "hunter2")
	p := bitwarden.NewWithClient("org-1", mock)

	val, err := p.Get(context.Background(), "DB_PASSWORD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "hunter2" {
		t.Fatalf("expected 'hunter2', got %q", val)
	}
}

func TestBitwardenProvider_GetNotFound(t *testing.T) {
	mock := newMock("uuid-1", "DB_PASSWORD", "hunter2")
	p := bitwarden.NewWithClient("org-1", mock)

	_, err := p.Get(context.Background(), "MISSING_KEY")
	if !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestBitwardenProvider_ListError(t *testing.T) {
	mock := &mockClient{listErr: errors.New("network down")}
	p := bitwarden.NewWithClient("org-1", mock)

	_, err := p.Get(context.Background(), "ANY")
	if err == nil {
		t.Fatal("expected error on list failure")
	}
}

func TestBitwardenProvider_GetError(t *testing.T) {
	mock := newMock("uuid-1", "DB_PASSWORD", "hunter2")
	mock.getErr["uuid-1"] = errors.New("permission denied")
	p := bitwarden.NewWithClient("org-1", mock)

	_, err := p.Get(context.Background(), "DB_PASSWORD")
	if err == nil {
		t.Fatal("expected error on get failure")
	}
}

func TestBitwardenProvider_List(t *testing.T) {
	mock := newMock(
		"uuid-1", "DB_PASSWORD", "s3cr3t",
		"uuid-2", "API_KEY", "key123",
	)
	p := bitwarden.NewWithClient("org-1", mock)

	keys, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d: %v", len(keys), keys)
	}
	found := map[string]bool{}
	for _, k := range keys {
		found[k] = true
	}
	for _, want := range []string{"DB_PASSWORD", "API_KEY"} {
		if !found[want] {
			t.Errorf("expected %q in list, got %v", want, keys)
		}
	}
}

func TestBitwardenProvider_Name(t *testing.T) {
	p := bitwarden.NewWithClient("org-1", &mockClient{})
	if p.Name() != "bitwarden" {
		t.Fatalf("expected 'bitwarden', got %q", p.Name())
	}
}
