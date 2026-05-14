# Authentication and Authorization

> Authentication chain, API Key management, and RBAC access control for the Hermes SaaS API.

## Authentication Chain (Auth Chain)

Hermes uses a chained authentication strategy, trying each method in order — the first successful match wins:

```
Request → Static Token → API Key → JWT → Anonymous (401)
```

| Order | Method | Use Case |
|-------|--------|----------|
| 1 | Static Token | Development / admin access |
| 2 | API Key | Multi-tenant user access |
| 3 | JWT | Production integration (reserved) |

All authentication methods produce a unified `AuthContext`:

```go
type AuthContext struct {
    Identity   string   // User ID or API Key ID
    TenantID   string   // Derived from credential, not from request header
    Roles      []string // ["user"] or ["admin"]
    AuthMethod string   // "static_token" / "api_key" / "jwt"
}
```

> The tenant ID is always derived from the credential and is never read from request headers. This is the core security guarantee for multi-tenant isolation.

## Static Token Authentication

The simplest authentication method, suitable for development testing and initial admin operations.

**Configuration**: Set the `HERMES_ACP_TOKEN` environment variable.

**Behavior**:
- Authentication succeeds when the Bearer Token matches `HERMES_ACP_TOKEN`
- Uses `crypto/subtle.ConstantTimeCompare` to prevent timing attacks
- Automatically maps to the default tenant `00000000-0000-0000-0000-000000000001`
- Fixed role: `admin`

```bash
# Use Static Token for admin operations
curl http://localhost:8080/v1/tenants \
  -H "Authorization: Bearer your-acp-token"
```

**Default tenant**: Created automatically on first startup with:
- ID: `00000000-0000-0000-0000-000000000001`
- Name: `Default Tenant`
- Plan: `pro`
- Rate Limit: 120 RPM
- Max Sessions: 10

## API Key Authentication

Create independent API Keys for each tenant with fine-grained permission control.

### Key Format

- Prefix: `hk_` (hermes key)
- Length: randomly generated, Base64 encoded
- Storage: raw key is SHA-256 hashed and stored in the `api_keys.key_hash` field

### Lifecycle

```
Create Key → Return raw value (once only) → In use → Expired / Revoked
```

### Create an API Key

```bash
curl -X POST http://localhost:8080/v1/api-keys \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-service",
    "tenant_id": "tenant-uuid",
    "roles": ["user"]
  }'
```

The `key` field in the response is returned only once — save it securely.

### Authentication Flow

1. Extract Token from `Authorization: Bearer hk_xxx` header
2. Compute SHA-256 hash
3. Look up matching `key_hash` in the `api_keys` table
4. Check if revoked (`revoked_at IS NOT NULL`)
5. Check if expired (`expires_at < now()`)
6. Return AuthContext (including `tenant_id` and `roles`)

### Revoke a Key

```bash
curl -X DELETE http://localhost:8080/v1/api-keys/key-uuid \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

## JWT Authentication (Reserved)

The JWT authentication framework is built in and can be enabled for production environment integration.

**Expected Claims**:

| Claim | Description |
|-------|-------------|
| `sub` | User ID |
| `tenant_id` | Tenant ID |
| `roles` | Roles array |
| `exp` | Expiry time |

**Signing Algorithm**: RS256

Uncomment in `saas.go` to enable:

```go
authChain.Add(auth.NewJWTExtractor(jwtConfig))
```

## RBAC Access Control

Role-based access control based on path prefixes.

### Role Definitions

| Role | Description |
|------|-------------|
| `admin` | Administrator, can access all endpoints |
| `user` | Regular user, can only access user endpoints |

### Endpoint Permission Matrix

| Path Prefix | Required Role | Description |
|-------------|---------------|-------------|
| `/v1/tenants` | `admin` | Tenant management |
| `/v1/api-keys` | `admin` | API Key management |
| `/v1/audit-logs` | `admin` | Audit log queries |
| `/v1/gdpr/` | `admin` | GDPR data management |
| `/v1/chat/completions` | `user` | Chat interface |
| `/v1/me` | `user` | Current identity |
| `/v1/usage` | `user` | Usage statistics |
| `/v1/mock-sessions` | `user` | Session management |
| `/v1/openapi` | `user` | API documentation |
| `/health/*` | No auth required | Health checks |
| `/metrics` | No auth required | Prometheus metrics |

### RBAC Decision Logic

```
1. Get AuthContext from Context
2. Not authenticated → 401 Unauthorized
3. Match path prefix to find required role
4. User roles include required role → allow
5. User roles include "admin" → allow (admin can access all endpoints)
6. Otherwise → 403 Forbidden
```

## Middleware Execution Order

Authentication and authorization position in the middleware chain:

```
Tracing → Metrics → RequestID → Auth → Tenant → Logging → Audit → RBAC → RateLimit → Handler
```

| Middleware | Responsibility |
|-----------|----------------|
| Auth | Extract AuthContext from request, write to Context |
| Tenant | Extract tenant_id from AuthContext |
| Audit | Record all authenticated requests to audit logs |
| RBAC | Check role permissions |
| RateLimit | Per-tenant rate limiting |

## Security Design

### Tenant Isolation

- Tenant ID is derived from credentials — never accepted from request headers
- All database queries automatically append `WHERE tenant_id = $1`
- API Keys from different tenants cannot access other tenants' data

### Timing Attack Protection

Static Token comparison uses `crypto/subtle.ConstantTimeCompare`, preventing token value inference through response timing.

### Key Security

- API Keys are stored as SHA-256 hashes — a database breach does not expose the original key
- The `prefix` field (first 8 characters) is used for identification in the admin UI without exposing the full key

### Rate Limiting

- Request rate is tracked per-tenant dimension (RPM)
- All API Keys under the same tenant share the quota
- Supports distributed rate limiting (Redis) + local LRU fallback
- Anonymous requests are rate-limited by IP address

## Related Documentation

- [API Reference](api-reference.md) — Complete endpoint documentation
- [Configuration Guide](configuration.md) — Environment variables
- [Architecture Overview](architecture.md) — Middleware chain details
