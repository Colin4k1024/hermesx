# Security Model

> HermesX — Enterprise Security Architecture
> Answers: Who can access? What data? How is it isolated? How is it audited?

---

## Threat Model

### Trust Boundaries

```
┌─────────────────────────────────────────────────────────────┐
│                    PUBLIC INTERNET                            │
└──────────────────────────┬──────────────────────────────────┘
                           │ TLS (reverse proxy)
┌──────────────────────────▼──────────────────────────────────┐
│              API SERVER (Auth Boundary)                       │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Auth Chain: Static Token → API Key → JWT → Reject    │   │
│  └──────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Tenant Derivation: AuthContext → tenant_id           │   │
│  │  (NEVER from request headers or body for non-admin)   │   │
│  └──────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  RBAC: Role + Scope → Allow/Deny                      │   │
│  └──────────────────────────────────────────────────────┘   │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│              DATA LAYER (Isolation Boundary)                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  PostgreSQL RLS: SET LOCAL app.current_tenant = ?      │   │
│  │  Every query filtered at DB level, not just app level  │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Key Principals

| Principal | Identity Source | Trust Level |
|-----------|----------------|-------------|
| Anonymous | No credential | Untrusted — rejected |
| Tenant User | API Key (scoped) | Trusted within own tenant |
| Tenant Admin | API Key (admin role) | Full access within own tenant |
| Platform Admin | Static Token / Super Admin | Cross-tenant access |
| Auditor | API Key (auditor role) | Read-only audit access |

---

## Authentication

### Auth Chain

```
Request
  │
  ├─ Header: Authorization: Bearer <token>
  │
  ▼
┌─────────────────────────────┐
│ 1. Static Token Check       │ → Match? → AuthContext{role: admin, tenant: *}
└──────────────┬──────────────┘
               │ No match
┌──────────────▼──────────────┐
│ 2. API Key Lookup           │ → SHA-256(token) → DB lookup
│    Check: not revoked       │ → Check: not expired
│    Check: tenant active     │ → AuthContext{role, tenant, scopes}
└──────────────┬──────────────┘
               │ Not found
┌──────────────▼──────────────┐
│ 3. JWT Validation           │ → Verify signature → Extract claims
│    (prepared, not active)   │ → AuthContext{role, tenant, user}
└──────────────┬──────────────┘
               │ No valid credential
               ▼
            401 Unauthorized
```

### API Key Security

- **Storage**: Only SHA-256 hash stored; raw key returned once at creation
- **Generation**: 32 bytes from `crypto/rand.Read` with explicit panic on failure
- **Format**: `sk-` prefix + base64url encoding (43 chars)
- **Lookup**: O(1) hash comparison, no timing side-channel
- **Lifecycle**: Create → Active → Revoked (soft delete, never hard delete)
- **Expiry**: Optional `expires_at` field; expired keys rejected at auth time

---

## Tenant Isolation

### Design Principle

**Tenant identity is NEVER derived from user-supplied headers or request body (for non-admin callers).**

The `TenantMiddleware` extracts `tenant_id` exclusively from `AuthContext`, which is set by the auth chain based on the credential presented.

### Defense in Depth

| Layer | Mechanism | Bypass Difficulty |
|-------|-----------|-------------------|
| 1. Application | `AuthContext.TenantID` from credential | Requires valid key for target tenant |
| 2. Middleware | All store calls include tenant_id | Requires code modification |
| 3. Database (RLS) | `SET LOCAL app.current_tenant` + policy | Requires superuser DB access |
| 4. Index | Unique indexes include tenant_id | Schema-level enforcement |

### PostgreSQL Row-Level Security

Every tenant-scoped table has:

```sql
ALTER TABLE <table> ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_<table> ON <table>
  USING (tenant_id::text = current_setting('app.current_tenant', true))
  WITH CHECK (tenant_id::text = current_setting('app.current_tenant', true));
```

The application sets the tenant context per transaction:

```go
func withTenantTx(ctx context.Context, pool *pgxpool.Pool, tenantID string, fn func(pgx.Tx) error) error {
    tx, _ := pool.Begin(ctx)
    tx.Exec(ctx, "SET LOCAL app.current_tenant = $1", tenantID)
    err := fn(tx)
    // commit or rollback
}
```

**Important**: RLS affects non-superuser roles. The application connects as a restricted role, not the database owner.

### Tables with RLS

| Table | Policy | Indexes |
|-------|--------|---------|
| sessions | tenant_isolation_sessions | idx_sessions_tenant |
| messages | tenant_isolation_messages | idx_messages_tenant_session |
| tenants | tenant_isolation_tenants | pk |
| api_keys | tenant_isolation_api_keys | idx_api_keys_tenant |
| audit_logs | tenant_isolation_audit | idx_audit_tenant |
| memories | tenant_isolation_memories | idx_memories_tenant_user |
| user_profiles | tenant_isolation_profiles | idx_profiles_tenant |
| roles | tenant_isolation_roles | idx_roles_tenant |
| cron_jobs | tenant_isolation_cron | idx_cron_tenant |
| execution_receipts | tenant_isolation_exec_receipts | idx_exec_receipts_tenant |

---

## Tool Execution Isolation

### Sandbox Model

```
Agent Runtime
  │
  ├─ Policy Check: Is tool in AllowedTools?
  │     No → outcome: skipped
  │
  ├─ Idempotency Check: Has this idempotency_key been seen?
  │     Yes → outcome: deduplicated (return cached result)
  │
  ├─ Sandbox Selection: SandboxPolicy.AllowDocker?
  │     ├─ Local: subprocess with timeout + env stripping
  │     └─ Docker: --network=none, --memory, --cpus
  │
  ├─ Execution: Run tool with timing capture
  │
  └─ Receipt: Record ExecutionReceipt with outcome + trace_id
```

### Sandbox Controls

| Control | Local | Docker |
|---------|-------|--------|
| Timeout | Process kill after N seconds | Container kill after N seconds |
| Network | Inherited (no restriction) | `--network=none` available |
| Filesystem | Process CWD only | Ephemeral container filesystem |
| Memory | OS limits | `--memory` flag |
| CPU | OS scheduling | `--cpus` flag |
| Env vars | Stripped to PATH/HOME/LANG/TERM/TMPDIR | Minimal env |
| Output | Truncated at 50KB | Truncated at 50KB |
| Tool calls | Max 50 per session | Max 50 per session |

### Per-Tenant Policy

```json
{
  "enabled": true,
  "max_timeout_seconds": 60,
  "allowed_tools": ["read_file", "write_file", "terminal", "web_search"],
  "allow_docker": true,
  "restrict_network": true,
  "max_stdout_kb": 50
}
```

---

## Audit Trail

### What Gets Audited

| Event | Actor | Resource | Metadata |
|-------|-------|----------|----------|
| API Key created | user | api_key | key_id, scopes |
| API Key revoked | admin | api_key | key_id, reason |
| Session created | user | session | session_id |
| Tool executed | agent | tool | tool_name, duration, status |
| GDPR export | admin | tenant | export_format |
| GDPR delete | admin | tenant | tables_affected |
| Tenant created | platform | tenant | plan, config |
| Sandbox policy changed | admin | tenant | old/new policy |

### Execution Receipts

Every tool invocation produces an `ExecutionReceipt`:

| Field | Purpose |
|-------|---------|
| id | Unique receipt identifier |
| tenant_id | Tenant boundary |
| session_id | Session context |
| tool_name | Which tool was called |
| input | Truncated input (4KB max) |
| output | Truncated output (4KB max) |
| status | success / error |
| duration_ms | Execution time |
| idempotency_id | At-most-once guarantee |
| trace_id | Distributed trace correlation |

### Idempotency

Unique index on `(tenant_id, idempotency_id)` ensures at-most-once execution. If a duplicate request arrives:
1. Lookup by idempotency_id
2. Return cached output from existing receipt
3. No re-execution occurs

---

## Rate Limiting

### Architecture

```
Request → Extract tenant_id + user_id
  │
  ├─ Tenant limit: sliding window per tenant (Redis Lua script)
  ├─ User limit: sliding window per user within tenant
  │
  ├─ Both pass? → Allow
  ├─ Either fails? → 429 Too Many Requests
  │
  └─ Redis down? → Local LRU fallback (degraded accuracy)
```

### Redis Sliding Window

Atomic Lua script ensures no race condition:
```
MULTI
  ZREMRANGEBYSCORE key 0 (now - window)
  ZADD key now now
  ZCARD key
EXEC
```

---

## Secret Management

### Principles

1. No hardcoded credentials in source code
2. All secrets via environment variables
3. API keys stored as SHA-256 hashes only
4. Raw keys returned exactly once at creation
5. `crypto/rand` for all key generation with explicit failure handling
6. No default passwords in any configuration

### Credential Rotation

- API Keys: Create new → migrate clients → revoke old
- Database: Connection string rotation via env var update + restart
- LLM keys: Hot-swap via env var (no restart required with config reload)

---

## Network Security Recommendations

| Component | Recommendation |
|-----------|---------------|
| API Server | Behind reverse proxy with TLS termination |
| PostgreSQL | Private network only, SSL required |
| Redis | Private network, AUTH enabled |
| MinIO | Private network, TLS for production |
| OTel Collector | Internal only, no public exposure |
| Metrics endpoint | Internal network or authenticated proxy |
