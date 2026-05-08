package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Colin4k1024/hermesx/internal/store"
	_ "github.com/go-sql-driver/mysql"
)

func init() {
	store.RegisterDriver("mysql", func(ctx context.Context, cfg store.StoreConfig) (store.Store, error) {
		return New(ctx, cfg.URL)
	})
}

// MySQLStore implements store.Store backed by MySQL.
type MySQLStore struct {
	db                *sql.DB
	sessions          *mySessionStore
	messages          *myMessageStore
	users             *myUserStore
	tenants           *myTenantStore
	auditLogs         *myAuditLogStore
	apiKeys           *myAPIKeyStore
	memories          *myMemoryStore
	userProfiles      *myUserProfileStore
	cronJobs          *myCronJobStore
	roles             *myRoleStore
	pricingRules      *myPricingRuleStore
	executionReceipts *myExecutionReceiptStore
}

// New opens a MySQL connection pool, pings to verify, and wires all sub-stores.
func New(ctx context.Context, dsn string) (*MySQLStore, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("mysql ping: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	s := &MySQLStore{db: db}
	s.sessions = &mySessionStore{db: db}
	s.messages = &myMessageStore{db: db}
	s.users = &myUserStore{db: db}
	s.tenants = &myTenantStore{db: db}
	s.auditLogs = &myAuditLogStore{db: db}
	s.apiKeys = &myAPIKeyStore{db: db}
	s.memories = &myMemoryStore{db: db}
	s.userProfiles = &myUserProfileStore{db: db}
	s.cronJobs = &myCronJobStore{db: db}
	s.roles = &myRoleStore{db: db}
	s.pricingRules = &myPricingRuleStore{db: db}
	s.executionReceipts = &myExecutionReceiptStore{db: db}
	return s, nil
}

func (s *MySQLStore) Sessions() store.SessionStore                   { return s.sessions }
func (s *MySQLStore) Messages() store.MessageStore                   { return s.messages }
func (s *MySQLStore) Users() store.UserStore                         { return s.users }
func (s *MySQLStore) Tenants() store.TenantStore                     { return s.tenants }
func (s *MySQLStore) AuditLogs() store.AuditLogStore                 { return s.auditLogs }
func (s *MySQLStore) APIKeys() store.APIKeyStore                     { return s.apiKeys }
func (s *MySQLStore) Memories() store.MemoryStore                    { return s.memories }
func (s *MySQLStore) UserProfiles() store.UserProfileStore           { return s.userProfiles }
func (s *MySQLStore) CronJobs() store.CronJobStore                   { return s.cronJobs }
func (s *MySQLStore) Roles() store.RoleStore                         { return s.roles }
func (s *MySQLStore) PricingRules() store.PricingRuleStore           { return s.pricingRules }
func (s *MySQLStore) ExecutionReceipts() store.ExecutionReceiptStore { return s.executionReceipts }

func (s *MySQLStore) Close() error {
	return s.db.Close()
}

func (s *MySQLStore) Migrate(ctx context.Context) error {
	return runMigrations(ctx, s.db)
}

// Ping satisfies the api.DBPinger interface for health checks.
func (s *MySQLStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

var _ store.Store = (*MySQLStore)(nil)

// nullStr converts empty string to nil for nullable columns.
func nullStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// nullInt converts zero to nil for nullable integer columns.
func nullInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}
