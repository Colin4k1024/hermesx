# ADR-005: hermesx-webui Admin Bootstrap Endpoint Design

## Decision Info

| Field | Value |
|-------|-------|
| Number | ADR-005 |
| Title | POST /admin/v1/bootstrap — First-Deploy Super Admin Bootstrap Endpoint |
| Status | Accepted |
| Date | 2026-05-08 |
| Owner | architect + backend-engineer |
| Related Requirement | docs/artifacts/2026-05-08-hermesx-webui/prd.md (US-Bootstrap) |

## Background and Constraints

- PRD Decision #3: On first deployment when no admin key exists, show a Bootstrap onboarding page and use `HERMES_ACP_TOKEN` to create the first admin key.
- The challenge session revealed: ACP (Agent Communication Protocol) is an editor integration protocol package, **not** an admin bootstrap token mechanism.
  - The `internal/acp/` package implements the editor ↔ agent JSON-RPC protocol
  - `HERMES_ACP_TOKEN` is a static bearer handled by `StaticTokenExtractor` in the auth chain, granting `system:acp-admin` identity
  - Therefore, the Bootstrap flow can reuse `HERMES_ACP_TOKEN` as a one-time authorization credential, but requires a dedicated Bootstrap endpoint
- There is currently no `GET /admin/v1/tenants/{id}/api-keys` endpoint (identified as a backend gap during intake).

## Design Decisions

### Bootstrap Endpoint Specification

#### GET /admin/v1/bootstrap/status
- No authentication required (public endpoint)
- Response: `{"bootstrap_required": true}` — returns true when the number of admin-role API keys in the DB is 0
- Frontend purpose: checked on page load to decide whether to show "Login" or "Bootstrap Onboarding"

#### POST /admin/v1/bootstrap
- Auth: `Authorization: Bearer <HERMES_ACP_TOKEN>` (static token, known only during initialization)
- Request body: `{"name": "initial-admin-key", "expires_at": "2027-01-01T00:00:00Z"}`
- Logic:
  1. Validate ACP token (via existing StaticTokenExtractor chain)
  2. **Atomic check**: if admin API key count > 0, return `403 Forbidden {"error": "bootstrap already completed"}`
  3. Create API key with roles: `["admin"]`, scopes: `["admin", "chat", "read"]`
  4. Return plaintext key (**only returned once**): `{"api_key": "hx-...", "key_id": "...", "name": "..."}`
- Security gate: endpoint returns 403 when bootstrap_required=false, preventing repeated calls

### GET /admin/v1/tenants/{id}/api-keys

New endpoint listing all API keys under a tenant (masked display):
- Auth: admin API key (`RequireScope("admin")`)
- Response: `{"api_keys": [{"id": "...", "name": "...", "prefix": "hx-...", "roles": [...], "scopes": [...], "expires_at": "...", "revoked_at": null, "created_at": "..."}]}`
- Implementation: query `api_keys` table filtered by `tenant_id`, does not return `key_hash`

### Frontend Bootstrap Flow

```
GET /admin/v1/bootstrap/status
  → {bootstrap_required: true}  → BootstrapPage.vue
  → {bootstrap_required: false} → AdminLoginPage.vue

BootstrapPage.vue:
  1. User enters HERMES_ACP_TOKEN (provided by operations team)
  2. Enter new admin key name and expiry
  3. POST /admin/v1/bootstrap → display one-time plaintext key (close after copying)
  4. Redirect to AdminLoginPage, log in with new key
```

### Security Notes

- HERMES_ACP_TOKEN is not stored in the frontend, only used for a single POST request, not placed in sessionStorage
- The bootstrap endpoint permanently returns 403 once the system is in production (because an admin key already exists)
- The plaintext key appears only once in the POST /admin/v1/bootstrap response; the backend stores only the SHA-256 hash

## Follow-up Actions

| Action | Owner | Completion Criteria |
|--------|-------|---------------------|
| Implement GET /admin/v1/bootstrap/status | backend-engineer | Phase 0 |
| Implement POST /admin/v1/bootstrap | backend-engineer | Phase 0 |
| Implement GET /admin/v1/tenants/{id}/api-keys | backend-engineer | Phase 0 |
| Frontend BootstrapPage.vue | frontend-engineer | Phase 1 |
| Frontend bootstrap status check integrated into AdminApp startup | frontend-engineer | Phase 1 |
