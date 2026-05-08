package objstore

import "context"

// ObjectStore abstracts S3-compatible object storage (MinIO, RustFS, etc.).
type ObjectStore interface {
	EnsureBucket(ctx context.Context) error
	Bucket() string
	Ping(ctx context.Context) error
	GetObject(ctx context.Context, key string) ([]byte, error)
	PutObject(ctx context.Context, key string, data []byte) error
	DeleteObject(ctx context.Context, key string) error
	ObjectExists(ctx context.Context, key string) (bool, error)
	ListObjects(ctx context.Context, prefix string) ([]string, error)
}
