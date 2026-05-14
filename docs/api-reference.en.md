# API Reference

> Complete endpoint documentation for the HermesX Enterprise Agent Platform. All authenticated endpoints require an `Authorization: Bearer <token>` header.

## Base Information

| Field | Value |
|-------|-------|
| Base URL | `http://localhost:8080` |
| Authentication | Bearer Token (Static Token / API Key / JWT) |
| Content Type | `application/json` |
| Rate Limiting | Returned via `X-RateLimit-Limit` and `X-RateLimit-Remaining` response headers |

## Public Endpoints (No Authentication Required)

### GET /health/live

Liveness probe. Returns 200 as soon as the service starts.

```bash
curl http://localhost:8080/health/live
# {"status":"ok"}
```

### GET /health/ready

Readiness probe. Checks database connection status.

```bash
curl http://localhost:8080/health/ready
# {"status":"ready","database":"ok"}
```

### GET /metrics

Prometheus metrics endpoint. Returns `text/plain` format.

```bash
curl http://localhost:8080/metrics
```

Metrics include:
- `hermes_http_requests_total{method, path, status, tenant_id}` — Total HTTP requests
- `hermes_http_request_duration_seconds{method, path, tenant_id}` — Request latency histogram
- `hermes_http_requests_in_flight` — Current concurrent request count

---

## Admin Endpoints (Require `admin` Role)

The following endpoints require admin role. Access using the Static Token (`HERMES_ACP_TOKEN`) or an API Key with admin role.

### Bootstrap /admin/v1/bootstrap

#### GET /admin/v1/bootstrap/status — Check if initialization is required

Public endpoint, no authentication required.

```bash
curl http://localhost:8080/admin/v1/bootstrap/status
# {"bootstrap_required":true}
```

#### POST /admin/v1/bootstrap — Create the first default tenant admin key

Only available when no default tenant admin key exists. This endpoint does not go through the admin scope middleware, but must carry `HERMES_ACP_TOKEN` and enforces independent rate limiting per source IP (default `HERMES_BOOTSTRAP_RATE_LIMIT_RPM=5`).

```bash
curl -X POST http://localhost:8080/admin/v1/bootstrap \
  -H "Authorization: Bearer $HERMES_ACP_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"initial-admin-key"}'
```

The `key` in the response is returned only once.

### Tenant Management /v1/tenants

#### POST /v1/tenants — Create a tenant

```bash
curl -X POST http://localhost:8080/v1/tenants \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "plan": "pro",
    "rate_limit_rpm": 120,
    "max_sessions": 50
  }'
```

Response:

```json
{
  "id": "a1b2c3d4-...",
  "name": "Acme Corp",
  "plan": "pro",
  "rate_limit_rpm": 120,
  "max_sessions": 50,
  "created_at": "2026-04-29T12:00:00Z",
  "updated_at": "2026-04-29T12:00:00Z"
}
```

> When creating a tenant with MinIO configured, the system asynchronously provisions all 81 built-in skills and a default SOUL.md personality file for the new tenant.

#### GET /v1/tenants — List all tenants

```bash
curl http://localhost:8080/v1/tenants \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

Response:

```json
{
  "tenants": [
    {
      "id": "a1b2c3d4-...",
      "name": "Acme Corp",
      "plan": "pro",
      "rate_limit_rpm": 120,
      "max_sessions": 50,
      "created_at": "...",
      "updated_at": "..."
    }
  ]
}
```

#### GET /v1/tenants/{id} — Get a single tenant

```bash
curl http://localhost:8080/v1/tenants/a1b2c3d4-... \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

#### PUT /v1/tenants/{id} — Update a tenant

```bash
curl -X PUT http://localhost:8080/v1/tenants/a1b2c3d4-... \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"plan": "enterprise", "rate_limit_rpm": 300}'
```

#### DELETE /v1/tenants/{id} — Delete a tenant

```bash
curl -X DELETE http://localhost:8080/v1/tenants/a1b2c3d4-... \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### API Key Management /v1/api-keys

#### POST /v1/api-keys — Create an API Key

```bash
curl -X POST http://localhost:8080/v1/api-keys \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "production-key",
    "tenant_id": "a1b2c3d4-...",
    "roles": ["user"]
  }'
```

Response:

```json
{
  "id": "key-uuid-...",
  "key": "hk_a1b2c3d4e5f6...",
  "prefix": "hk_a1b2c",
  "name": "production-key",
  "tenant_id": "a1b2c3d4-...",
  "roles": ["user"],
  "created_at": "..."
}
```

> The `key` field is returned only once at creation time. API Keys are stored as SHA-256 hashes in the database and cannot be retrieved again.

#### GET /v1/api-keys — List all API Keys

```bash
curl http://localhost:8080/v1/api-keys \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

#### DELETE /v1/api-keys/{id} — Revoke an API Key

```bash
curl -X DELETE http://localhost:8080/v1/api-keys/key-uuid-... \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Audit Logs /v1/audit-logs

#### GET /v1/audit-logs — Query audit logs

Requires `auditor` role.

```bash
curl "http://localhost:8080/v1/audit-logs?limit=50" \
  -H "Authorization: Bearer hk_your_api_key"
```

Supported query parameters: `action`, `from` (ISO 8601 time), `to`, `limit` (default 50), `offset`.

Each audit record contains: `tenant_id`, `user_id`, `action`, `detail`, `request_id`, `status_code`, `latency_ms`, `created_at`.

### Execution Receipts /v1/execution-receipts

#### GET /v1/execution-receipts — List tool execution receipts

Requires `auditor` role. Isolated by tenant, returns tool call records (input/output/status/duration).

```bash
curl "http://localhost:8080/v1/execution-receipts?limit=50" \
  -H "Authorization: Bearer hk_your_api_key"
```

Supported query parameters:

| Parameter | Type | Description |
|-----------|------|-------------|
| `session_id` | string | Filter by session ID |
| `tool_name` | string | Filter by tool name |
| `status` | string | Filter by status (`success`/`error`/`timeout`) |
| `limit` | integer | Items per page (default 50) |
| `offset` | integer | Offset (default 0) |

Response:

```json
{
  "execution_receipts": [
    {
      "id": "uuid-...",
      "tenant_id": "uuid-...",
      "session_id": "sess-...",
      "user_id": "user-...",
      "tool_name": "code-review",
      "input": "...",
      "output": "...",
      "status": "success",
      "duration_ms": 1234,
      "idempotency_id": "idem-...",
      "trace_id": "trace-...",
      "created_at": "2026-04-29T12:00:00Z"
    }
  ],
  "total": 100
}
```

#### GET /v1/execution-receipts/{id} — Get a single execution receipt

Requires `auditor` role.

```bash
curl http://localhost:8080/v1/execution-receipts/uuid-... \
  -H "Authorization: Bearer hk_your_api_key"
```

### GDPR Compliance /v1/gdpr

#### GET /v1/gdpr/export — Export user data

```bash
curl http://localhost:8080/v1/gdpr/export \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

#### DELETE /v1/gdpr/data — Delete user data

```bash
curl -X DELETE http://localhost:8080/v1/gdpr/data \
  -H "Authorization: Bearer hk_your_api_key"
```

#### POST /v1/gdpr/cleanup-minio — Clean up orphaned MinIO objects

Cleans media files that exist in MinIO for the current tenant but have no database references.

```bash
curl -X POST http://localhost:8080/v1/gdpr/cleanup-minio \
  -H "Authorization: Bearer hk_your_api_key"
```

---

## User Endpoints (Authentication Required, `user` or `admin` Role)

### POST /v1/chat/completions — Send a Chat Request

OpenAI-compatible chat interface. Automatically associated with the tenant of the requester.

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer hk_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mock",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

Response:

```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "..."
      },
      "finish_reason": "stop"
    }
  ]
}
```

Supported request headers:

| Header | Description |
|--------|-------------|
| `X-Hermes-Session-Id` | Specify a session ID to maintain multi-turn conversations |
| `X-Hermes-User-Id` | Specify a user ID for memory and profile isolation (defaults to API Key identity if not provided) |

Chat requests automatically inject the following context (requires MinIO and PostgreSQL to be configured):

- **Soul**: Loads the tenant's `SOUL.md` personality file from MinIO
- **Memory and profiles**: Loads user-level memories and profiles from PostgreSQL
- **Skill summary**: Loads a list of the tenant's installed skills from MinIO

### POST /v1/agent/chat — Agent Chat Interface (Alias)

Same functionality as `/v1/chat/completions`, providing the Agent tool call loop.

```bash
curl -X POST http://localhost:8080/v1/agent/chat \
  -H "Authorization: Bearer hk_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mock",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### GET /v1/me — Current Identity Information

Returns the identity, tenant, and roles of the currently authenticated user.

```bash
curl http://localhost:8080/v1/me \
  -H "Authorization: Bearer hk_your_api_key"
```

Response:

```json
{
  "identity": "key-uuid-...",
  "tenant_id": "a1b2c3d4-...",
  "roles": ["user"],
  "auth_method": "api_key"
}
```

### GET /v1/usage — Usage Statistics

Returns session and message usage statistics for the current tenant.

```bash
curl http://localhost:8080/v1/usage \
  -H "Authorization: Bearer hk_your_api_key"
```

### Session Management /v1/sessions

#### GET /v1/sessions — List user sessions

Returns a paginated list of sessions for the current tenant (optionally filtered by `user_id`).

```bash
curl "http://localhost:8080/v1/sessions?limit=50&offset=0&user_id=xxx" \
  -H "Authorization: Bearer hk_your_api_key"
```

Supported query parameters: `limit` (default 50), `offset` (default 0), `user_id`.

Response:

```json
{
  "sessions": [
    {
      "id": "sess-...",
      "tenant_id": "uuid-...",
      "user_id": "user-...",
      "model": "mock",
      "created_at": "...",
      "updated_at": "..."
    }
  ],
  "total": 10
}
```

#### GET /v1/sessions/{id} — Get session details

Returns the specified session and its message history.

```bash
curl http://localhost:8080/v1/sessions/sess-... \
  -H "Authorization: Bearer hk_your_api_key"
```

#### DELETE /v1/sessions/{id} — Delete a session

```bash
curl -X DELETE http://localhost:8080/v1/sessions/sess-... \
  -H "Authorization: Bearer hk_your_api_key"
```

### Long-Term Memory /v1/memories

#### GET /v1/memories — List user memories

Returns long-term memory entries for the current user (specified via the `X-Hermes-User-Id` header).

```bash
curl http://localhost:8080/v1/memories \
  -H "Authorization: Bearer hk_your_api_key" \
  -H "X-Hermes-User-Id: user-xxx"
```

Response:

```json
{
  "memories": [
    {
      "key": "preference",
      "value": "prefers dark mode",
      "created_at": "..."
    }
  ],
  "total": 5
}
```

#### DELETE /v1/memories/{key} — Delete a memory entry

```bash
curl -X DELETE http://localhost:8080/v1/memories/preference \
  -H "Authorization: Bearer hk_your_api_key" \
  -H "X-Hermes-User-Id: user-xxx"
```

Returns `204 No Content` on success.

### Skills Management /v1/skills

#### GET /v1/skills — List tenant skills

Returns all Skills installed for the current tenant, including source and modification status.

```bash
curl http://localhost:8080/v1/skills \
  -H "Authorization: Bearer hk_your_api_key"
```

Response:

```json
{
  "tenant_id": "a1b2c3d4-...",
  "skills": [
    {
      "name": "code-review",
      "description": "Code review assistant",
      "version": "1.0.0",
      "source": "builtin",
      "user_modified": false
    },
    {
      "name": "my-custom-skill",
      "description": "Custom business skill",
      "version": "1.0.0",
      "source": "user",
      "user_modified": true
    }
  ],
  "total": 2
}
```

#### GET /v1/skills/{name} — Get skill content

Returns the full SKILL.md content of the specified skill (raw text).

```bash
curl http://localhost:8080/v1/skills/my-custom-skill \
  -H "Authorization: Bearer hk_your_api_key"
```

#### PUT /v1/skills/{name} — Upload/update a skill

Uploads SKILL.md content as a tenant custom skill. After upload, the skill is marked as `user_modified` and will not be overwritten by system auto-sync.

```bash
curl -X PUT http://localhost:8080/v1/skills/my-custom-skill \
  -H "Authorization: Bearer hk_your_api_key" \
  -H "Content-Type: text/plain" \
  -d '---
name: "my-custom-skill"
description: "Custom business skill"
version: "1.0.0"
---

# My Custom Skill

You are a specialized assistant for my business domain.'
```

Response:

```json
{
  "status": "uploaded",
  "skill": "my-custom-skill"
}
```

Limit: maximum request body size is 1MB.

#### DELETE /v1/skills/{name} — Delete a skill

```bash
curl -X DELETE http://localhost:8080/v1/skills/my-custom-skill \
  -H "Authorization: Bearer hk_your_api_key"
```

Returns `204 No Content` on success.

### GET /v1/openapi — OpenAPI Specification

Returns the OpenAPI 3.0 specification document in JSON format.

```bash
curl http://localhost:8080/v1/openapi
```

---

## Error Codes

| HTTP Status | Description |
|-------------|-------------|
| 200 | Success |
| 204 | Success (no content, e.g., OPTIONS preflight) |
| 400 | Bad request parameters |
| 401 | Unauthenticated (missing or invalid token) |
| 403 | Insufficient permissions (role requirements not met) |
| 404 | Resource not found |
| 429 | Rate limit exceeded |
| 500 | Internal server error |

## Rate Limiting

Each response includes rate limit information in the headers:

| Response Header | Description |
|-----------------|-------------|
| `X-RateLimit-Limit` | Requests allowed in the current window (RPM) |
| `X-RateLimit-Remaining` | Remaining available requests |
| `Retry-After` | Returned when rate limited, suggested wait time in seconds (fixed 60s) |

Rate limiting is tracked per tenant — all API Keys under the same tenant share the quota. Unauthenticated requests are rate-limited by IP address.

## CORS

Configured via the `SAAS_ALLOWED_ORIGINS` environment variable:

- Set to `*` to allow all origins
- Set to a comma-separated list of domains for precise control

Allowed methods: `GET, POST, PUT, DELETE, OPTIONS`  
Allowed headers: `Authorization, Content-Type, X-Hermes-Session-Id, X-Hermes-User-Id`

## Admin Sub-Routes /admin/*

Admin panel dedicated routes requiring `admin` role. Provides RESTful interfaces for advanced management functions (pricing rules, platform configuration, etc.).

```bash
curl http://localhost:8080/admin/v1/pricing-rules \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

## Static Pages

When `SAAS_STATIC_DIR` is configured, the following routes are served from static files:

| Path | Description |
|------|-------------|
| `/` | Homepage (index.html) |
| `/admin.html` | Admin panel |
| `/static/*` | Static assets directory |
