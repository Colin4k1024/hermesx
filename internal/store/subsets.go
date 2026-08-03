package store

import "context"

// SessionQuerier is the minimal interface for reading sessions.
// Consumers that only need to list/get sessions should depend on this
// instead of the full Store.
type SessionQuerier interface {
	List(ctx context.Context, tenantID string, opts ListOptions) ([]*Session, int, error)
	Get(ctx context.Context, tenantID, sessionID string) (*Session, error)
}

// MessageAppender is the minimal interface for appending messages.
// Consumers that only need to write messages should depend on this.
type MessageAppender interface {
	Append(ctx context.Context, tenantID, sessionID string, msg *Message) (int64, error)
}

// AuditAppender is the minimal interface for writing audit logs.
// Consumers that only need to record audit events should depend on this.
type AuditAppender interface {
	Append(ctx context.Context, log *AuditLog) error
}

// TenantQuerier is the minimal interface for reading tenant info.
// Consumers that only need to look up tenant config should depend on this.
type TenantQuerier interface {
	Get(ctx context.Context, id string) (*Tenant, error)
}

// MemoryReadWriter is the minimal interface for per-user memory CRUD.
// Consumers that need memory read/write but not full Store should depend on this.
type MemoryReadWriter interface {
	Get(ctx context.Context, tenantID, userID, key string) (string, error)
	List(ctx context.Context, tenantID, userID string) ([]MemoryEntry, error)
	Upsert(ctx context.Context, tenantID, userID, key, content string) error
	Delete(ctx context.Context, tenantID, userID, key string) error
}
