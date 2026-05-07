# Security Model

> hermes-agent-go — Enterprise Agent Runtime  
> Version: v1.4.0

---

## 1. Who Can Access?

### Authentication Chain

请求经过 4 层链式认证（按优先级从高到低）：

```
Static Token → API Key → JWT → OIDC
```

| Layer | Header | Credential | Use Case |
|-------|--------|-----------|----------|
| Static Token | `Authorization: Bearer {token}` | `HERMES_ACP_TOKEN` env var | Admin bootstrap, CI scripts |
| API Key | `Authorization: Bearer hk_xxx` | SHA-256 hash stored in `api_keys` table | Application integration |
| JWT | `Authorization: Bearer {jwt}` | RSA/ECDSA signed token | Service-to-service |
| OIDC | `Authorization: Bearer {id_token}` | JWKS-validated, auto-rotation | Enterprise SSO (IdP) |

**Design Principle:** 第一个成功匹配的 extractor 终止链。匹配失败时返回 401。

### Credential Security

- API Key raw value **仅在创建时返回一次**，数据库存储 SHA-256 hash
- API Key 有 `hk_` 前缀便于 secret scanning 工具识别
- Static Token **必须 ≥ 32 字符**，从环境变量加载
- OIDC 支持 JWKS 端点自动轮换（无需重启服务）

---

## 2. Access Which Tenant?

### Tenant Derivation — Never Trust Client

```
┌──────────────────────────────────────────────────┐
│  CRITICAL DESIGN DECISION                        │
│                                                  │
│  tenant_id 永远从认证凭证派生，                    │
│  绝不从请求头（X-Tenant-ID）读取。                 │
│                                                  │
│  这防止了租户伪造攻击。                            │
└──────────────────────────────────────────────────┘
```

**Derivation paths:**

| Auth Method | tenant_id Source |
|-------------|-----------------|
| Static Token | 硬编码默认租户 `00000000-...0001` |
| API Key | `api_keys.tenant_id` (创建时绑定) |
| JWT | Token claim `tenant_id` |
| OIDC | ClaimMapper 映射的 claim（默认 `tenant_id`） |

**Context Propagation:**

```go
AuthContext.TenantID  // from credential
     ↓
TenantMiddleware     // writes to context
     ↓
Store queries        // WHERE tenant_id = $1
     ↓
PostgreSQL RLS       // SET LOCAL app.tenant_id = $1
```

### Cross-Tenant Attack Prevention

| Attack Vector | Defense |
|---------------|---------|
| Header injection (`X-Tenant-ID: other`) | Ignored — derived from credential only |
| Path traversal (`../other-tenant/`) | UUID validation + path sanitization |
| SQL injection | Parameterized queries ($1 binding) |
| Session hijacking | Session ownership check (user_id match) |
| RLS bypass | Application uses restricted role; superuser only for migrations |

---

## 3. How Are API Keys Authorized?

### Key Lifecycle

```
Create (admin) → Active → Revoke (admin/owner) → Soft-deleted
                    │
                    └── Scopes: [chat, admin, tools, memories, ...]
```

### Scope Enforcement

```go
// API Key carries scopes
type APIKeyRecord struct {
    TenantID  string
    KeyHash   string
    Scopes    []string  // e.g. ["chat", "tools"]
    Roles     []string  // e.g. ["user"]
}

// Endpoint checks scope
if !ac.HasScope("admin") {
    return 403
}
```

### Key Permissions

| Scope | Allows |
|-------|--------|
| `chat` | POST /v1/agent/chat, session CRUD |
| `tools` | Tool execution within agent |
| `memories` | Memory read/write for own user |
| `admin` | Tenant management, API key CRUD, user management |
| `audit` | Read audit logs |
| `gdpr` | Data export and deletion |

### Role Hierarchy

```
admin > owner > user > auditor
```

`admin` role implicitly passes all RBAC checks (line 48 of rbac.go: `!ac.HasRole("admin")`).

---

## 4. How Is Tool Execution Isolated?

### Sandbox Architecture

```
Agent Runtime
     ↓
Tool Call Request
     ↓
┌─────────────────────┐
│  Policy Check        │ ← AllowedTools whitelist
│  (allow/deny)        │ ← MaxToolCalls limit (default: 50)
└─────────┬───────────┘
          ↓
┌─────────────────────┐     ┌─────────────────────┐
│  Local Sandbox       │ OR  │  Docker Sandbox      │
│  (process isolation) │     │  (container isolation)│
│  - env stripped      │     │  - --network=none    │
│  - timeout enforced  │     │  - --memory limit    │
│  - stdout truncated  │     │  - --cpus limit      │
└─────────────────────┘     └─────────────────────┘
```

### Per-Tenant Sandbox Policy

```sql
ALTER TABLE tenants ADD COLUMN sandbox_policy JSONB;
```

```json
{
  "enabled": true,
  "max_timeout_seconds": 60,
  "allowed_tools": ["read_file", "write_file", "terminal"],
  "allow_docker": true,
  "restrict_network": true,
  "max_stdout_kb": 50
}
```

### Environment Isolation

Tool execution 进程只暴露最小环境变量：

```
PATH, HOME, LANG, TERM, TMPDIR
```

所有其他环境变量（包括 API keys、DB credentials）被剥离。

---

## 5. How Are Sensitive Operations Audited?

### Audit Scope

**所有认证后的请求** 自动记录到 `audit_logs` 表：

```sql
CREATE TABLE audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    user_id     TEXT,
    action      TEXT NOT NULL,       -- "POST /v1/agent/chat"
    resource    TEXT,                 -- request path
    request_id  TEXT,                 -- X-Request-ID (correlates with OTel)
    status_code INTEGER,
    latency_ms  INTEGER,
    ip_address  TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
```

### Tamper Protection

```sql
-- Migration v26: immutable audit trigger
CREATE OR REPLACE FUNCTION prevent_audit_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs cannot be modified or deleted';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_immutable
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_modification();
```

### Correlation

每条审计记录通过 `request_id` 关联：
- OpenTelemetry span（分布式 trace）
- Structured log entries（slog）
- LLM call metrics（Prometheus）

```
Audit Log ←── request_id ──→ OTel Trace ──→ Prometheus Metrics
```

---

## 6. How Is Cross-Tenant Data Leakage Prevented?

### Defense in Depth (3 Layers)

```
Layer 1: Application — WHERE tenant_id = $1 (50+ queries)
     ↓
Layer 2: PostgreSQL RLS — FORCE ROW LEVEL SECURITY
     ↓
Layer 3: Static Analysis — go-sql-tenant-enforcement
```

### PostgreSQL Row-Level Security

All 9 business tables have RLS enabled:

```sql
ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE sessions FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON sessions
    USING (tenant_id = current_setting('app.tenant_id')::uuid);

CREATE POLICY tenant_write_check ON sessions
    FOR INSERT WITH CHECK (tenant_id = current_setting('app.tenant_id')::uuid);
```

### Session Variable Lifecycle

```
Request arrives
  → TenantMiddleware extracts tenant_id from AuthContext
  → Before query: SET LOCAL app.tenant_id = '{uuid}'
  → Query executes (RLS enforced)
  → Connection returns to pool (SET LOCAL resets at transaction end)
```

### Object Storage Isolation

MinIO objects use tenant-prefixed keys:

```
{tenant_id}/soul/SOUL.md
{tenant_id}/{skill_name}/SKILL.md
```

Path traversal prevention: skill names validated against `../` patterns.

---

## 7. Threat Model Summary

| Threat | Severity | Mitigation | Status |
|--------|----------|------------|--------|
| Tenant impersonation | CRITICAL | Credential-derived tenant_id | ✅ Mitigated |
| Cross-tenant data read | CRITICAL | RLS + WHERE clause + static analysis | ✅ Mitigated |
| Cross-tenant data write | CRITICAL | RLS WITH CHECK policy | ✅ Mitigated |
| API key theft | HIGH | SHA-256 hash storage, one-time reveal | ✅ Mitigated |
| Tool escape | HIGH | Sandbox whitelist + env strip + Docker isolation | ✅ Mitigated |
| Audit tampering | HIGH | PG trigger prevents UPDATE/DELETE | ✅ Mitigated |
| Session hijacking | MEDIUM | Session ownership check (user_id) | ✅ Mitigated |
| Rate limit bypass | MEDIUM | Redis Lua atomic + local fallback | ✅ Mitigated |
| LLM prompt injection | MEDIUM | Partial sanitization (compress/curator) | ⚠️ Incomplete |
| Admin privilege escalation | MEDIUM | HasScope fix deployed, needs test coverage | ⚠️ Needs tests |
| Denial of service | LOW | Circuit breaker + rate limit + timeout | ✅ Mitigated |

---

## 8. Security Assumptions

1. PostgreSQL 连接使用 TLS（生产环境 `sslmode=require`）
2. Redis 在内网部署，无公网暴露
3. MinIO 使用 access key 认证，bucket policy 限制
4. Docker daemon 仅本机可达
5. 环境变量由 orchestrator（K8s secrets / Docker secrets）注入
6. OIDC IdP 的 JWKS endpoint 可信且 TLS 保护

---

## 9. Gaps & Planned Improvements

| Gap | Priority | Target Week |
|-----|----------|-------------|
| ExecutionReceipt（tool call 审计凭证） | P0 | Week 4-5 |
| Cross-tenant penetration tests | P0 | Week 2 |
| API Key tenant_id boundary enforcement | P0 | Week 3 |
| rand.Read error handling | P1 | Week 3 |
| Full prompt injection defense | P1 | Backlog |
| SAST/DAST integration | P2 | Week 7 |
| SOC 2 evidence collection | P2 | Post v1.0 |
