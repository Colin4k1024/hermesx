# RBAC Permission Matrix

> hermes-agent-go — Enterprise Agent Runtime  
> Version: v1.4.0

---

## Current Implementation

### Route-Based RBAC (Production)

RBAC 通过 `method×path` 前缀匹配实现（`internal/middleware/rbac.go`）。

当前规则配置（`cmd/hermes/saas.go`）：

```go
RBACConfig{
    DefaultRole: "user",
    Rules: map[string]string{
        "/v1/tenants":    "admin",
        "/v1/tenants/":   "admin",
        "/v1/api-keys":   "admin",
        "/v1/api-keys/":  "admin",
        "/v1/audit-logs": "admin",
        "/v1/gdpr/":      "admin",
    },
}
```

**规则语义：**
- 未列出的路由 → 需要 `DefaultRole`（"user"）
- `admin` role 隐式绕过所有检查（`!ac.HasRole("admin")` bypass）
- method-specific 规则优先级更高（score += 1000）

---

## Permission Matrix

### Route × Role

| Endpoint | Method | admin | user | auditor | Notes |
|----------|--------|-------|------|---------|-------|
| `/v1/tenants` | GET | ✅ | ❌ | ❌ | List all tenants |
| `/v1/tenants` | POST | ✅ | ❌ | ❌ | Create tenant |
| `/v1/tenants/{id}` | GET | ✅ | ❌ | ❌ | Get tenant details |
| `/v1/tenants/{id}` | PUT | ✅ | ❌ | ❌ | Update tenant |
| `/v1/api-keys` | GET | ✅ | ❌ | ❌ | List API keys |
| `/v1/api-keys` | POST | ✅ | ❌ | ❌ | Create API key |
| `/v1/api-keys/{id}` | DELETE | ✅ | ❌ | ❌ | Revoke API key |
| `/v1/audit-logs` | GET | ✅ | ❌ | ❌ | Query audit logs |
| `/v1/gdpr/export` | GET | ✅ | ❌ | ❌ | Export tenant data |
| `/v1/gdpr/data` | DELETE | ✅ | ❌ | ❌ | Delete tenant data |
| `/v1/gdpr/cleanup-minio` | POST | ✅ | ❌ | ❌ | Clean MinIO objects |
| `/v1/agent/chat` | POST | ✅ | ✅ | ❌ | Agent conversation |
| `/v1/chat/completions` | POST | ✅ | ✅ | ❌ | OpenAI-compat chat |
| `/v1/sessions` | GET | ✅ | ✅ | ❌ | List own sessions |
| `/v1/sessions/{id}` | GET | ✅ | ✅ | ❌ | Get session (ownership check) |
| `/v1/sessions/{id}` | DELETE | ✅ | ✅ | ❌ | Delete own session |
| `/v1/me` | GET | ✅ | ✅ | ❌ | Current user profile |
| `/v1/memories` | GET/POST | ✅ | ✅ | ❌ | Memory CRUD (own user) |
| `/health/ready` | GET | 🌐 | 🌐 | 🌐 | Public (no auth) |
| `/health/live` | GET | 🌐 | 🌐 | 🌐 | Public (no auth) |
| `/metrics` | GET | 🌐 | 🌐 | 🌐 | Public (Prometheus scrape) |

Legend: ✅ allowed | ❌ forbidden | 🌐 public (no auth required)

---

## Scope Model (API Key)

API Key 创建时指定 scopes，控制细粒度权限：

| Scope | Grants Access To |
|-------|-----------------|
| `chat` | `/v1/agent/chat`, `/v1/chat/completions`, session CRUD |
| `admin` | Tenant CRUD, API key CRUD, GDPR, audit logs |
| `tools` | Tool execution within agent conversations |
| `memories` | Memory read/write for authenticated user |
| `audit` | Read audit log queries |
| `gdpr` | Data export and deletion operations |

### Scope Enforcement

```go
// HasScope checks if the authenticated context has a specific scope
func (ac *AuthContext) HasScope(scope string) bool {
    for _, s := range ac.Scopes {
        if s == scope {
            return true
        }
    }
    return false
}
```

---

## Target Permission Matrix (Proposed)

### Role Definitions

| Role | Description | Assignment |
|------|-------------|------------|
| `super_admin` | Platform operator | Static token only |
| `admin` | Tenant administrator | API key with admin scope |
| `owner` | Tenant owner | First user of tenant |
| `user` | Regular user | API key with chat scope |
| `auditor` | Read-only audit access | API key with audit scope |
| `service` | Machine-to-machine | JWT/OIDC with service claim |

### Resource × Action Matrix

| Resource | Action | super_admin | admin | owner | user | auditor | service |
|----------|--------|-------------|-------|-------|------|---------|---------|
| **tenants** | create | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **tenants** | read | ✅ | ✅ (own) | ✅ (own) | ❌ | ❌ | ❌ |
| **tenants** | update | ✅ | ✅ (own) | ✅ (own) | ❌ | ❌ | ❌ |
| **tenants** | delete | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **api_keys** | create | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| **api_keys** | list | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| **api_keys** | revoke | ✅ | ✅ | ✅ (own) | ❌ | ❌ | ❌ |
| **sessions** | create | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
| **sessions** | read | ✅ | ✅ (tenant) | ✅ (tenant) | ✅ (own) | ❌ | ✅ (own) |
| **sessions** | delete | ✅ | ✅ (tenant) | ✅ (own) | ✅ (own) | ❌ | ❌ |
| **messages** | read | ✅ | ✅ (tenant) | ✅ (tenant) | ✅ (own session) | ❌ | ✅ (own) |
| **memories** | read | ✅ | ✅ (tenant) | ✅ (own) | ✅ (own) | ❌ | ✅ (own) |
| **memories** | write | ✅ | ✅ (tenant) | ✅ (own) | ✅ (own) | ❌ | ✅ (own) |
| **tools** | execute | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
| **audit_logs** | read | ✅ | ✅ (tenant) | ❌ | ❌ | ✅ (tenant) | ❌ |
| **gdpr** | export | ✅ | ✅ (own tenant) | ✅ (own) | ❌ | ❌ | ❌ |
| **gdpr** | delete | ✅ | ✅ (own tenant) | ❌ | ❌ | ❌ | ❌ |
| **usage** | read | ✅ | ✅ (tenant) | ✅ (own) | ✅ (own) | ✅ (tenant) | ❌ |
| **pricing** | manage | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **receipts** | read | ✅ | ✅ (tenant) | ✅ (own) | ✅ (own) | ✅ (tenant) | ✅ (own) |

Legend: ✅ = full access | ✅ (own) = only own resources | ✅ (tenant) = all within tenant | ❌ = denied

---

## Implementation Gap Analysis

| Current State | Target State | Effort |
|---------------|-------------|--------|
| 2 roles (admin, user) | 6 roles | Medium |
| Route prefix matching | Resource:action granular | High |
| No ownership check on API keys | Owner-scoped revocation | Low |
| admin bypasses all | super_admin vs admin distinction | Medium |
| No auditor role | Read-only audit access | Low |
| Scopes on API key only | Scopes unified with OIDC claims | Medium |

---

## Migration Path

### Phase 1 (Week 3) — Minimal fixes

1. Enforce: non-admin cannot pass `tenant_id` in API key creation body
2. Add `auditor` role with read-only audit_logs access
3. Add ownership check on session read (already partially done)

### Phase 2 (Week 4+) — Full resource:action model

1. Define `Permission` type: `{resource, action, scope}`
2. Build `PolicyEngine` that evaluates role + scopes + ownership
3. Replace route-prefix RBAC with policy-based middleware
4. Add `roles` table FK to support dynamic role assignment

---

## Security Notes

1. **admin bypass** — `HasRole("admin")` skips all RBAC checks. This is intentional for bootstrap but should be restricted to `super_admin` in production.
2. **Empty scopes** — Fixed: `HasScope("")` no longer returns true for empty scope lists.
3. **Static token** — Maps to default tenant with admin role. Suitable for CI/bootstrap, not for production API access.
4. **OIDC roles** — `ClaimMapper.rolesClaim()` extracts roles from IdP token. Must be validated against known role set.
