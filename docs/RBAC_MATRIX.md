# RBAC Permission Matrix

> Role-Based Access Control for HermesX SaaS API
> Roles are assigned per API Key. A key can hold multiple roles.

---

## Roles

| Role | Description | Scope |
|------|-------------|-------|
| `super_admin` | Platform operator | Cross-tenant, all operations |
| `admin` | Tenant administrator | Own tenant, all operations |
| `owner` | Tenant owner | Own tenant, management + execution |
| `user` | Regular user | Own tenant, execution + self-data |
| `auditor` | Read-only compliance | Own tenant, read audit/receipts |

---

## Permission Matrix

### Resource: Tenants

| Operation | super_admin | admin | owner | user | auditor |
|-----------|:-----------:|:-----:|:-----:|:----:|:-------:|
| List all tenants | Yes | — | — | — | — |
| Get own tenant | Yes | Yes | Yes | — | — |
| Create tenant | Yes | — | — | — | — |
| Update tenant | Yes | Yes | — | — | — |
| Delete tenant | Yes | — | — | — | — |

### Resource: API Keys

| Operation | super_admin | admin | owner | user | auditor |
|-----------|:-----------:|:-----:|:-----:|:----:|:-------:|
| Create (any tenant) | Yes | — | — | — | — |
| Create (own tenant) | Yes | Yes | Yes | — | — |
| List (own tenant) | Yes | Yes | Yes | — | — |
| Revoke (own tenant) | Yes | Yes | Yes | — | — |
| Rotate | Yes | Yes | Yes | — | — |

### Resource: Sessions

| Operation | super_admin | admin | owner | user | auditor |
|-----------|:-----------:|:-----:|:-----:|:----:|:-------:|
| Create | Yes | Yes | Yes | Yes | — |
| List (all in tenant) | Yes | Yes | Yes | — | — |
| List (own) | Yes | Yes | Yes | Yes | — |
| Get | Yes | Yes | Yes | Yes* | Yes |
| Delete | Yes | Yes | Yes | — | — |

*user can only access own sessions

### Resource: Chat Completions

| Operation | super_admin | admin | owner | user | auditor |
|-----------|:-----------:|:-----:|:-----:|:----:|:-------:|
| POST /v1/chat/completions | Yes | Yes | Yes | Yes | — |

### Resource: Tools

| Operation | super_admin | admin | owner | user | auditor |
|-----------|:-----------:|:-----:|:-----:|:----:|:-------:|
| Execute (allowed) | Yes | Yes | Yes | Yes | — |
| Execute (any) | Yes | Yes | — | — | — |
| Configure sandbox | Yes | Yes | — | — | — |

### Resource: Memories

| Operation | super_admin | admin | owner | user | auditor |
|-----------|:-----------:|:-----:|:-----:|:----:|:-------:|
| Set (own) | Yes | Yes | Yes | Yes | — |
| Get (own) | Yes | Yes | Yes | Yes | — |
| List (tenant) | Yes | Yes | Yes | — | — |
| Delete (own) | Yes | Yes | Yes | Yes | — |
| Delete (any in tenant) | Yes | Yes | — | — | — |

### Resource: Audit Logs

| Operation | super_admin | admin | owner | user | auditor |
|-----------|:-----------:|:-----:|:-----:|:----:|:-------:|
| List (own tenant) | Yes | Yes | — | — | Yes |
| Export | Yes | Yes | — | — | Yes |

### Resource: Execution Receipts

| Operation | super_admin | admin | owner | user | auditor |
|-----------|:-----------:|:-----:|:-----:|:----:|:-------:|
| List (own tenant) | Yes | Yes | — | — | Yes |
| Get by ID | Yes | Yes | — | — | Yes |

### Resource: Usage / Metering

| Operation | super_admin | admin | owner | user | auditor |
|-----------|:-----------:|:-----:|:-----:|:----:|:-------:|
| Query (own tenant) | Yes | Yes | Yes | — | Yes |
| Query (cross-tenant) | Yes | — | — | — | — |

### Resource: GDPR

| Operation | super_admin | admin | owner | user | auditor |
|-----------|:-----------:|:-----:|:-----:|:----:|:-------:|
| Export tenant data | Yes | Yes | — | — | — |
| Delete tenant data | Yes | Yes | — | — | — |

### Resource: Sandbox Policy

| Operation | super_admin | admin | owner | user | auditor |
|-----------|:-----------:|:-----:|:-----:|:----:|:-------:|
| Get | Yes | Yes | Yes | — | — |
| Update | Yes | Yes | — | — | — |

---

## Route-to-Role Mapping

Current enforcement in `cmd/hermes/saas.go`:

```
Default role: user (all routes accessible unless overridden)

Admin-gated routes:
  /v1/tenants          → admin
  /v1/tenants/         → admin
  /v1/api-keys         → admin
  /v1/api-keys/        → admin
  /v1/gdpr/            → admin

Auditor-gated routes:
  GET /v1/audit-logs         → auditor
  /v1/execution-receipts     → auditor
```

---

## Scope Model

Scopes provide fine-grained access within a role:

| Scope | Grants |
|-------|--------|
| `admin` | Full administrative access |
| `read` | Read operations on all accessible resources |
| `write` | Create/update operations |
| `execute` | Tool and chat execution |
| `audit` | Read audit logs and execution receipts |
| `gdpr` | Data export and deletion |

### Legacy Key Behavior

Keys with empty scopes (pre-scope migration):
- Allowed: `read`, `write`, `execute`
- Denied: `admin` (must be explicitly granted)

---

## Tenant Boundary Rules

1. **Non-admin callers**: `tenant_id` always derived from credential context
2. **Admin callers**: May specify `tenant_id` in request body for cross-tenant operations
3. **Body-supplied tenant_id**: Only honored when `AuthContext.HasRole("admin")` or `HasRole("super_admin")`
4. **X-Tenant-ID header**: NEVER trusted for tenant derivation (ignored by middleware)
