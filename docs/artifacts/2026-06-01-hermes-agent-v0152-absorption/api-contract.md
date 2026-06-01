# API Contract: MCP Catalog Governance

> Date: 2026-06-01  
> Status: phase-2-implemented

## Scope

These endpoints expose a governed MCP catalog through the Admin control plane. They are intentionally separate from runtime `mcp.json` loading.

## Endpoints

| Method | Path | Scopes | Purpose |
| --- | --- | --- | --- |
| GET | `/admin/v1/mcp-catalog` | `security:read` or `ops:read` | List catalog items |
| GET | `/admin/v1/mcp-catalog/{id}` | `security:read` or `ops:read` | Get one catalog item |
| PUT | `/admin/v1/mcp-catalog/{id}` | `security:write` or `ops:write` | Upsert catalog item |
| GET | `/admin/v1/mcp-catalog/tenants/{id}` | `security:read` or `tenant:read` | List tenant item policies |
| PUT | `/admin/v1/mcp-catalog/tenants/{id}/items/{itemID}` | `security:write` or `tenant:write` | Enable or disable one item for a tenant |

Legacy `admin` scope remains accepted through `RequireAnyScope` compatibility.

## Catalog Item

```json
{
  "id": "n8n",
  "name": "n8n",
  "version": "1.0.0",
  "description": "n8n MCP server",
  "source_url": "https://github.com/n8n-io/n8n",
  "trust_tier": "trusted",
  "review_status": "approved",
  "transport": "stdio",
  "command": "npx",
  "args": ["-y", "@n8n/mcp"],
  "url": "",
  "required_credentials": ["N8N_API_KEY"],
  "scopes": ["workflow:read"],
  "egress_domains": ["api.n8n.io"],
  "sandbox_required": true
}
```

Validation:

- `id`, `name`, and `source_url` are required.
- `trust_tier` must be `official`, `trusted`, or `community`.
- `review_status` must be `approved`, `pending`, or `blocked`.
- `transport` must be `stdio` or `sse`.
- `stdio` items require `command`.
- `sse` items require `url`.

## Tenant Item Policy

```json
{
  "enabled": true,
  "reason": "approved for tenant"
}
```

Response:

```json
{
  "tenant_id": "tenant-a",
  "item_id": "n8n",
  "enabled": true,
  "reason": "approved for tenant",
  "updated_by": "admin-user",
  "updated_at": "2026-06-01T00:00:00Z"
}
```

## Audit Events

| Action | Trigger |
| --- | --- |
| `admin.mcp_catalog.item.upsert` | Catalog item upsert |
| `admin.mcp_catalog.tenant_policy.set` | Tenant item policy update |
