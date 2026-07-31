package objstore

import (
	"testing"
)

func TestNewMinIOClient(t *testing.T) {
	// Valid construction — should not error.
	client, err := NewMinIOClient("localhost:9000", "access", "secret", "test-bucket", false)
	if err != nil {
		t.Fatalf("NewMinIOClient: %v", err)
	}
	if client == nil {
		t.Fatal("NewMinIOClient returned nil client")
	}
	if client.Bucket() != "test-bucket" {
		t.Errorf("Bucket() = %q, want %q", client.Bucket(), "test-bucket")
	}
}

func TestNewMinIOClient_SSL(t *testing.T) {
	client, err := NewMinIOClient("localhost:9000", "access", "secret", "bucket", true)
	if err != nil {
		t.Fatalf("NewMinIOClient with SSL: %v", err)
	}
	if client == nil {
		t.Fatal("NewMinIOClient returned nil")
	}
}

func TestNewObjStoreClient_ReturnsMinIOClient(t *testing.T) {
	store, err := NewObjStoreClient("localhost:9000", "access", "secret", "bucket", false)
	if err != nil {
		t.Fatalf("NewObjStoreClient: %v", err)
	}
	if store == nil {
		t.Fatal("NewObjStoreClient returned nil")
	}
	if store.Bucket() != "bucket" {
		t.Errorf("Bucket() = %q", store.Bucket())
	}
}

func TestMinIOClient_ImplementsObjectStore(t *testing.T) {
	// Compile-time check is in minio.go via `var _ ObjectStore = (*MinIOClient)(nil)`.
	// Runtime check for completeness.
	client, _ := NewMinIOClient("localhost:9000", "a", "s", "b", false)
	var _ ObjectStore = client
}
