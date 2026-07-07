package mysql

import (
	"context"
	"fmt"

	"github.com/Colin4k1024/hermesx/internal/store"
)

var errMySQLFileEntryUnsupported = fmt.Errorf("file entries not supported in MySQL mode — use PostgreSQL")

type noopFileEntryStore struct{}

func (n *noopFileEntryStore) List(_ context.Context, _, _ string) ([]*store.FileEntry, error) {
	return nil, errMySQLFileEntryUnsupported
}
func (n *noopFileEntryStore) Get(_ context.Context, _, _, _ string) (*store.FileEntry, error) {
	return nil, errMySQLFileEntryUnsupported
}
func (n *noopFileEntryStore) Create(_ context.Context, _ *store.FileEntry) error {
	return errMySQLFileEntryUnsupported
}
func (n *noopFileEntryStore) Delete(_ context.Context, _, _, _ string) error {
	return errMySQLFileEntryUnsupported
}
func (n *noopFileEntryStore) GetUserStorageUsage(_ context.Context, _, _ string) (int64, error) {
	return 0, errMySQLFileEntryUnsupported
}

var _ store.FileEntryStore = (*noopFileEntryStore)(nil)
