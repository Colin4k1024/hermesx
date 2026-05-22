# Platform Governance Center

> Scope: internal platform administration for tenants, usage, sharing, sandbox, egress, secrets, safety, and retention controls.

## Governance Principle

Platform control actions must be provable:

- Who changed the policy.
- What changed before and after.
- Why it changed.
- Which tenant or platform domain was affected.
- How it can be disabled or rolled back.

## Control Domains

| Domain | Primary Scope | Current API |
|--------|---------------|-------------|
| Sandbox policy | `ops:*`, `tenant:*` | `/admin/v1/tenants/{id}/sandbox-policy` |
| API keys | `key:*`, `tenant:read` | `/admin/v1/tenants/{id}/api-keys` |
| Pricing and billing | `billing:*` | `/admin/v1/pricing-rules`, `/admin/v1/usage` |
| Cross-tenant audit | `audit:read` | `/admin/v1/audit-logs` |
| Secrets and safety | `security:*` | `/admin/v1/secrets/*`, `/admin/v1/safety/*` |
| Egress | `security:write`, `ops:write` | `/admin/v1/egress/*` |
| Shared learning | `sharing:*`, `security:*` | `/admin/v1/evolution/*` |

The legacy `admin` scope is retained as explicit break-glass compatibility only. Daily automation should use domain scopes.

## Shared Learning Controls

Global policy:

- `GET /admin/v1/evolution/sharing-policy`
- `GET /admin/v1/evolution/sharing-policy/history`
- `PUT /admin/v1/evolution/sharing-policy`
- `POST /admin/v1/evolution/sharing-policy/rollback`

Tenant policy:

- `GET /admin/v1/evolution/tenants/{id}/sharing-policy`
- `GET /admin/v1/evolution/tenants/{id}/sharing-policy/history`
- `PUT /admin/v1/evolution/tenants/{id}/sharing-policy`
- `POST /admin/v1/evolution/tenants/{id}/sharing-policy/rollback`

Rollback:

- Policy rollback creates a fresh current version instead of mutating prior history.
- Use history endpoints to identify the target `version`, then call the corresponding rollback endpoint with that version.
- `POST /admin/v1/evolution/shared-knowledge/revoke`
- Shared revoke now executes as bounded batches against the evolution gene store, reducing operator risk during large rollback windows.

Policy fields:

| Field | Meaning |
|-------|---------|
| `mode` | Global maximum sharing level: `disabled`, `anonymous`, or `trusted`. |
| `consume_shared` | Whether a tenant may read shared knowledge. |
| `contribution_mode` | Tenant contribution level, capped by global `mode`. |
| `labels` | Governance labels for review, sensitivity, and audit context. |
| `reason` | Operator-provided change reason written to audit logs. |

## Audit Contract

Every policy mutation must write an `audit_logs` entry with:

- `action`: stable action name, for example `admin.evolution.sharing_policy.update`.
- `tenant_id`: operator tenant from auth context.
- `user_id`: operator identity when available.
- `detail`: JSON object containing before/after state, criteria, result count, and reason.
- `request_id`, `source_ip`, `user_agent`: request metadata when available.

## Release Gate

A release that changes governance behavior must include:

- Domain scope regression.
- Admin route/OpenAPI parity check.
- Audit log sample for each changed control domain.
- Rollback path for every mutable policy.
- MySQL and PostgreSQL validation when persistence is affected.
