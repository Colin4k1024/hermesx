package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
)

// All noop store methods return errSQLiteUnsupported. This test validates that
// contract is upheld so that callers can rely on consistent error handling.

func TestNoopTenantStore(t *testing.T) {
	ctx := context.Background()
	n := &noopTenantStore{}

	if err := n.Create(ctx, &store.Tenant{}); err != errSQLiteUnsupported {
		t.Errorf("Create: got %v, want errSQLiteUnsupported", err)
	}
	if _, err := n.Get(ctx, "id"); err != errSQLiteUnsupported {
		t.Errorf("Get: got %v, want errSQLiteUnsupported", err)
	}
	if err := n.Update(ctx, &store.Tenant{}); err != errSQLiteUnsupported {
		t.Errorf("Update: got %v, want errSQLiteUnsupported", err)
	}
	if err := n.Delete(ctx, "id"); err != errSQLiteUnsupported {
		t.Errorf("Delete: got %v, want errSQLiteUnsupported", err)
	}
	if _, _, err := n.List(ctx, store.ListOptions{}); err != errSQLiteUnsupported {
		t.Errorf("List: got %v, want errSQLiteUnsupported", err)
	}
	if _, err := n.ListDeleted(ctx, time.Now()); err != errSQLiteUnsupported {
		t.Errorf("ListDeleted: got %v, want errSQLiteUnsupported", err)
	}
	if err := n.HardDelete(ctx, "id"); err != errSQLiteUnsupported {
		t.Errorf("HardDelete: got %v, want errSQLiteUnsupported", err)
	}
	if err := n.Restore(ctx, "id"); err != errSQLiteUnsupported {
		t.Errorf("Restore: got %v, want errSQLiteUnsupported", err)
	}
}

func TestNoopAuditLogStore(t *testing.T) {
	ctx := context.Background()
	n := &noopAuditLogStore{}

	if err := n.Append(ctx, &store.AuditLog{}); err != errSQLiteUnsupported {
		t.Errorf("Append: got %v, want errSQLiteUnsupported", err)
	}
	if _, _, err := n.List(ctx, "t1", store.AuditListOptions{}); err != errSQLiteUnsupported {
		t.Errorf("List: got %v, want errSQLiteUnsupported", err)
	}
	if _, err := n.DeleteByTenant(ctx, "t1"); err != errSQLiteUnsupported {
		t.Errorf("DeleteByTenant: got %v, want errSQLiteUnsupported", err)
	}
	if _, err := n.ArchiveOlderThan(ctx, time.Now(), 100); err != errSQLiteUnsupported {
		t.Errorf("ArchiveOlderThan: got %v, want errSQLiteUnsupported", err)
	}
	if _, err := n.ArchiveCount(ctx, time.Now()); err != errSQLiteUnsupported {
		t.Errorf("ArchiveCount: got %v, want errSQLiteUnsupported", err)
	}
}

func TestNoopWorkflowStore(t *testing.T) {
	ctx := context.Background()
	n := &noopWorkflowStore{}

	if err := n.CreateDefinition(ctx, &store.WorkflowDefinition{}); err != errSQLiteUnsupported {
		t.Errorf("CreateDefinition: got %v, want errSQLiteUnsupported", err)
	}
	if err := n.UpdateDefinition(ctx, &store.WorkflowDefinition{}); err != errSQLiteUnsupported {
		t.Errorf("UpdateDefinition: got %v, want errSQLiteUnsupported", err)
	}
	if _, err := n.GetDefinition(ctx, "t1", "def1"); err != errSQLiteUnsupported {
		t.Errorf("GetDefinition: got %v, want errSQLiteUnsupported", err)
	}
	if _, err := n.ListDefinitions(ctx, "t1"); err != errSQLiteUnsupported {
		t.Errorf("ListDefinitions: got %v, want errSQLiteUnsupported", err)
	}
	if err := n.CreateVersion(ctx, &store.WorkflowVersion{}); err != errSQLiteUnsupported {
		t.Errorf("CreateVersion: got %v, want errSQLiteUnsupported", err)
	}
	if _, err := n.GetVersion(ctx, "t1", "v1"); err != errSQLiteUnsupported {
		t.Errorf("GetVersion: got %v, want errSQLiteUnsupported", err)
	}
	if _, err := n.GetLatestVersion(ctx, "t1", "def1"); err != errSQLiteUnsupported {
		t.Errorf("GetLatestVersion: got %v, want errSQLiteUnsupported", err)
	}
	if err := n.CreateRun(ctx, &store.WorkflowRun{}, nil); err != errSQLiteUnsupported {
		t.Errorf("CreateRun: got %v, want errSQLiteUnsupported", err)
	}
	if _, err := n.GetRun(ctx, "t1", "run1"); err != errSQLiteUnsupported {
		t.Errorf("GetRun: got %v, want errSQLiteUnsupported", err)
	}
	if _, _, err := n.ListRuns(ctx, "t1", store.WorkflowRunListOptions{}); err != errSQLiteUnsupported {
		t.Errorf("ListRuns: got %v, want errSQLiteUnsupported", err)
	}
	if err := n.UpdateRun(ctx, &store.WorkflowRun{}); err != errSQLiteUnsupported {
		t.Errorf("UpdateRun: got %v, want errSQLiteUnsupported", err)
	}
	if _, err := n.GetStepRun(ctx, "t1", "sr1"); err != errSQLiteUnsupported {
		t.Errorf("GetStepRun: got %v, want errSQLiteUnsupported", err)
	}
	if _, err := n.ListStepRuns(ctx, "t1", "run1"); err != errSQLiteUnsupported {
		t.Errorf("ListStepRuns: got %v, want errSQLiteUnsupported", err)
	}
	if err := n.UpdateStepRun(ctx, &store.WorkflowStepRun{}); err != errSQLiteUnsupported {
		t.Errorf("UpdateStepRun: got %v, want errSQLiteUnsupported", err)
	}
	if _, err := n.ListPendingHumanTasks(ctx, "t1", "u1", nil); err != errSQLiteUnsupported {
		t.Errorf("ListPendingHumanTasks: got %v, want errSQLiteUnsupported", err)
	}
	if _, err := n.DeleteAllByTenant(ctx, "t1"); err != errSQLiteUnsupported {
		t.Errorf("DeleteAllByTenant: got %v, want errSQLiteUnsupported", err)
	}
}
