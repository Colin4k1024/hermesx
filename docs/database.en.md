# Database

> Schema design, data models, migration system, and tenant isolation for the Hermes SaaS API.

## Storage Backends

| Backend | Driver | Use Case | Package Path |
|---------|--------|----------|-------------|
| PostgreSQL 16+ | pgx/v5 | SaaS multi-tenant mode | `internal/store/pg/` |
| SQLite | go-sqlite3 | CLI single-user mode | `internal/store/sqlite/` |

SaaS mode requires PostgreSQL. The connection is configured via the `DATABASE_URL` environment variable.

## Tables

### tenants — Tenants

```sql
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    plan TEXT NOT NULL DEFAULT 'free',
    rate_limit_rpm INT NOT NULL DEFAULT 60,
    max_sessions INT NOT NULL DEFAULT 100,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Unique tenant identifier |
| `name` | TEXT | Tenant name |
| `plan` | TEXT | Plan tier (free / pro / enterprise) |
| `rate_limit_rpm` | INT | Requests per minute limit |
| `max_sessions` | INT | Maximum session count |

### sessions — Sessions

```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    platform TEXT NOT NULL,
    user_id TEXT NOT NULL,
    model TEXT,
    system_prompt TEXT,
    parent_session_id TEXT,
    title TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at TIMESTAMPTZ,
    end_reason TEXT,
    message_count INT DEFAULT 0,
    tool_call_count INT DEFAULT 0,
    input_tokens INT DEFAULT 0,
    output_tokens INT DEFAULT 0,
    cache_read_tokens INT DEFAULT 0,
    cache_write_tokens INT DEFAULT 0,
    estimated_cost_usd NUMERIC(10,6),
    metadata JSONB DEFAULT '{}',
    session_key TEXT
);
```

| Field | Description |
|-------|-------------|
| `tenant_id` | Owning tenant (FK) |
| `platform` | Platform identifier |
| `user_id` | User identifier |
| `model` | LLM model used |
| `system_prompt` | System prompt |
| `parent_session_id` | Parent session (for branching) |
| `message_count` | Message count |
| `input_tokens` / `output_tokens` | Token usage |
| `estimated_cost_usd` | Estimated cost |
| `session_key` | Unique session key (added in v20) |

**Indexes**:
- `idx_sessions_tenant` — `(tenant_id)`
- `idx_sessions_user` — `(tenant_id, user_id)`
- `idx_sessions_platform` — `(tenant_id, platform)`
- `idx_sessions_key` — `UNIQUE (session_key) WHERE session_key IS NOT NULL`

### messages — Messages

```sql
CREATE TABLE messages (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT,
    tool_call_id TEXT,
    tool_calls JSONB,
    tool_name TEXT,
    reasoning TEXT,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT now(),
    token_count INT,
    finish_reason TEXT
);
```

| Field | Description |
|-------|-------------|
| `role` | Message role (user / assistant / system / tool) |
| `content` | Message content |
| `tool_calls` | Tool call data (JSONB) |
| `tool_name` | Tool name |
| `reasoning` | Reasoning process |
| `finish_reason` | End reason (stop / tool_calls / length) |

**Indexes**:
- `idx_messages_session` — `(tenant_id, session_id)`
- `idx_messages_ts` — `(tenant_id, session_id, timestamp)`
- `idx_messages_fts` — `GIN(to_tsvector('english', coalesce(content, '')))` full-text search

### users — Users

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    external_id TEXT NOT NULL,
    username TEXT,
    display_name TEXT,
    role TEXT DEFAULT 'user',
    approved_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}'
);
```

**Indexes**:
- `idx_users_external` — `UNIQUE (tenant_id, external_id)`

### api_keys — API Keys

```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    prefix TEXT NOT NULL,
    roles TEXT[] DEFAULT '{user}',
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

| Field | Description |
|-------|-------------|
| `key_hash` | SHA-256 hash of the original key |
| `prefix` | Key prefix (`hk_` + first few chars), used for identification in admin UI |
| `roles` | Roles array (`{user}` or `{admin}`) |
| `expires_at` | Expiry time (NULL = never expires) |
| `revoked_at` | Revocation time (NOT NULL = revoked) |

**Indexes**:
- `idx_apikeys_hash` — `UNIQUE (key_hash)`
- `idx_apikeys_tenant` — `(tenant_id)`

### audit_logs — Audit Logs

```sql
CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    user_id UUID,
    session_id TEXT,
    action TEXT NOT NULL,
    detail TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    request_id TEXT,      -- added in v24
    status_code INT,      -- added in v25
    latency_ms INT        -- added in v26
);
```

**Indexes**:
- `idx_audit_tenant` — `(tenant_id)`
- `idx_audit_request` — `(request_id)`

### cron_jobs — Scheduled Jobs

```sql
CREATE TABLE cron_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    prompt TEXT NOT NULL,
    schedule TEXT NOT NULL,
    deliver TEXT,
    enabled BOOLEAN DEFAULT true,
    model TEXT,
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    run_count INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata JSONB DEFAULT '{}'
);
```

**Indexes**:
- `idx_cron_tenant` — `(tenant_id)`
- `idx_cron_next` — `(next_run_at) WHERE enabled = true` partial index

### memories — Memories

```sql
CREATE TABLE memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    user_id TEXT NOT NULL,
    key TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, user_id, key)
);
```

The memories table stores the Agent's long-term memory, with a unique constraint on `(tenant_id, user_id, key)` supporting upsert operations.

### user_profiles — User Profiles

```sql
CREATE TABLE user_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    user_id TEXT NOT NULL,
    content TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, user_id)
);
```

User profiles stores the Agent's user profiles, unique per `(tenant_id, user_id)`.

## Tenant Isolation

All business tables implement tenant isolation via foreign key `tenant_id UUID NOT NULL REFERENCES tenants(id)`:

```
tenants
  ├── sessions (FK: tenant_id)
  ├── messages (FK: tenant_id)
  ├── users (FK: tenant_id)
  ├── api_keys (FK: tenant_id)
  ├── audit_logs (FK: tenant_id)
  ├── cron_jobs (FK: tenant_id)
  ├── memories (FK: tenant_id)
  └── user_profiles (FK: tenant_id)
```

**Isolation mechanisms**:
- Tenant ID is derived from authentication credentials — never read from request headers
- All Store method queries automatically append `WHERE tenant_id = $1`
- Foreign key constraints ensure referential integrity

## Index Reference

| Index Name | Table | Columns | Type |
|------------|-------|---------|------|
| `idx_sessions_tenant` | sessions | `(tenant_id)` | B-tree |
| `idx_sessions_user` | sessions | `(tenant_id, user_id)` | B-tree |
| `idx_sessions_platform` | sessions | `(tenant_id, platform)` | B-tree |
| `idx_sessions_key` | sessions | `(session_key)` | Unique, Partial |
| `idx_messages_session` | messages | `(tenant_id, session_id)` | B-tree |
| `idx_messages_ts` | messages | `(tenant_id, session_id, timestamp)` | B-tree |
| `idx_messages_fts` | messages | `to_tsvector(content)` | GIN |
| `idx_users_external` | users | `(tenant_id, external_id)` | Unique |
| `idx_apikeys_hash` | api_keys | `(key_hash)` | Unique |
| `idx_apikeys_tenant` | api_keys | `(tenant_id)` | B-tree |
| `idx_audit_tenant` | audit_logs | `(tenant_id)` | B-tree |
| `idx_audit_request` | audit_logs | `(request_id)` | B-tree |
| `idx_cron_tenant` | cron_jobs | `(tenant_id)` | B-tree |
| `idx_cron_next` | cron_jobs | `(next_run_at)` | B-tree, Partial |

## Migration System

### How It Works

Hermes manages database migrations using embedded Go code defined in `internal/store/pg/migrate.go`.

```
Startup → Create schema_version table → Read current version → Execute new migrations in order → Record version
```

- **Version tracking**: `schema_version` table records applied migration versions and timestamps
- **Idempotent execution**: Uses `IF NOT EXISTS` and `ADD COLUMN IF NOT EXISTS`
- **Sequential guarantee**: Migrations execute in ascending version order
- **Auto-execute on startup**: Automatically checked and executed each time `hermes saas-api` starts

### Current Version

27 migrations total, organized into phases:

| Version Range | Content |
|--------------|---------|
| v1 | Create `tenants` table |
| v2-v5 | Create `sessions` table + 3 indexes |
| v6-v9 | Create `messages` table + 3 indexes (including GIN full-text search) |
| v10-v11 | Create `users` table + unique index |
| v12-v13 | Create `audit_logs` table + index |
| v14-v16 | Create `cron_jobs` table + 2 indexes |
| v17-v19 | Create `api_keys` table + 2 indexes |
| v20-v21 | Add `session_key` column to `sessions` + unique index |
| v22 | Create `memories` table |
| v23 | Create `user_profiles` table |
| v24-v27 | Add `request_id`, `status_code`, `latency_ms` to `audit_logs` + index |

### Adding New Migrations

Append new entries to the `migrations` slice in `internal/store/pg/migrate.go`:

```go
var migrations = []migration{
    // ... existing 27 migrations ...

    // New migration example
    {28, `ALTER TABLE sessions ADD COLUMN IF NOT EXISTS tags TEXT[]`},
}
```

**Notes**:
- Version numbers must increment and must not repeat
- Use `IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS` for idempotency
- DDL statements should be backward-compatible (avoid dropping columns, changing types, etc.)
- For testing, clearing the `schema_version` table re-executes all migrations

## Go Data Models

### Session

```go
type Session struct {
    ID              string
    TenantID        string
    Platform        string
    UserID          string
    Model           string
    SystemPrompt    string
    ParentSessionID string
    Title           string
    StartedAt       time.Time
    EndedAt         *time.Time
    EndReason       string
    MessageCount    int
    ToolCallCount   int
    InputTokens     int
    OutputTokens    int
    CacheReadTokens int
    CacheWriteTokens int
    EstimatedCostUSD *float64
    Metadata        json.RawMessage
    SessionKey      string
    // ... more fields
}
```

### Tenant

```go
type Tenant struct {
    ID           string
    Name         string
    Plan         string
    RateLimitRPM int
    MaxSessions  int
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### APIKey

```go
type APIKey struct {
    ID        string
    TenantID  string
    Name      string
    KeyHash   string
    Prefix    string
    Roles     []string
    ExpiresAt *time.Time
    RevokedAt *time.Time
    CreatedAt time.Time
}
```

Full data model definitions are in `internal/store/types.go`.

## Connection Management

Recommended configuration:

```bash
# Basic connection
DATABASE_URL="postgres://hermes:password@host:5432/hermes?sslmode=require"

# Connection pool parameters (via URL parameters)
DATABASE_URL="postgres://hermes:password@host:5432/hermes?sslmode=require&pool_max_conns=20&pool_min_conns=5"
```

Production recommendations:
- Use PgBouncer as connection pooling proxy
- Configure `sslmode=require` or `sslmode=verify-full`
- Inject `DATABASE_URL` via Kubernetes Secret

## Related Documentation

- [Architecture Overview](architecture.md) — Store layer design
- [Configuration Guide](configuration.md) — DATABASE_URL configuration
- [Deployment Guide](deployment.md) — PostgreSQL deployment options
- [Observability](observability.md) — pgx Tracer and database monitoring
