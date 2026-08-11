package store

import (
	"context"
	"errors"
	"testing"
)

func TestRegisterDriver_And_NewStore(t *testing.T) {
	const driverName = "test-factory-driver"

	// Ensure clean state (no lingering registration from other test runs).
	delete(registry, driverName)
	defer delete(registry, driverName)

	// Before registration, NewStore should return error.
	_, err := NewStore(context.Background(), StoreConfig{Driver: driverName})
	if err == nil {
		t.Fatal("expected error for unregistered driver")
	}

	// Register a driver that returns a mock store.
	RegisterDriver(driverName, func(ctx context.Context, cfg StoreConfig) (Store, error) {
		return NewTracedStore(&mockStore{
			sessions:          &mockSessionStore{},
			messages:          &mockMessageStore{},
			users:             &mockUserStore{},
			tenants:           &mockTenantStore{},
			auditLogs:         &mockAuditLogStore{},
			apiKeys:           &mockAPIKeyStore{},
			memories:          &mockMemoryStore{},
			userProfiles:      &mockUserProfileStore{},
			cronJobs:          &mockCronJobStore{},
			roles:             &mockRoleStore{},
			pricingRules:      &mockPricingRuleStore{},
			executionReceipts: &mockExecutionReceiptStore{},
			fileEntries:       &mockFileEntryStore{},
			workflows:         &mockWorkflowStore{},
			agentProfiles:     &mockAgentProfileStore{},
		}), nil
	})

	// Now NewStore should succeed.
	s, err := NewStore(context.Background(), StoreConfig{Driver: driverName})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestNewStore_DefaultDriverPostgres(t *testing.T) {
	// Default driver "postgres" is not registered in tests, so should fail with "unknown driver".
	_, err := NewStore(context.Background(), StoreConfig{})
	if err == nil {
		t.Skip("postgres driver is registered, skipping")
	}
}

func TestNewStore_DriverInitError(t *testing.T) {
	const driverName = "test-init-error-driver"
	delete(registry, driverName)
	defer delete(registry, driverName)

	initErr := errors.New("init failed")
	RegisterDriver(driverName, func(_ context.Context, _ StoreConfig) (Store, error) {
		return nil, initErr
	})

	_, err := NewStore(context.Background(), StoreConfig{Driver: driverName})
	if err == nil {
		t.Fatal("expected error from init failure")
	}
}

func TestRegisteredDrivers(t *testing.T) {
	const driverName = "test-list-driver"
	delete(registry, driverName)
	defer delete(registry, driverName)

	before := len(registeredDrivers())
	RegisterDriver(driverName, func(_ context.Context, _ StoreConfig) (Store, error) {
		return nil, nil
	})
	after := len(registeredDrivers())
	if after != before+1 {
		t.Errorf("registeredDrivers count: before=%d, after=%d, want +1", before, after)
	}
}
