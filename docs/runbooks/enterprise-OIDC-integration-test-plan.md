# Enterprise OIDC Integration Test Plan

## Purpose

Validate that HermesX accepts standards-compliant OIDC ID tokens from a real IdP, maps enterprise claims into `AuthContext`, and keeps existing API key authentication working as a fallback.

## Code Paths Under Test

| Area | Evidence |
|------|----------|
| OIDC verifier and claim mapper | `internal/auth/oidc.go`, `internal/auth/oidc_test.go` |
| Runtime wiring | `cmd/hermesx/saas.go` (`OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_TENANT_CLAIM`, `OIDC_ROLES_CLAIM`, `OIDC_ACR_CLAIM`) |
| Identity echo endpoint | `GET /v1/me` |
| RBAC enforcement | `internal/middleware/rbac.go`, admin/audit routes |

## Provider Matrix

| Provider | Required Result | Notes |
|----------|-----------------|-------|
| Local in-process IdP | `go test ./internal/auth -run 'TestOIDCExtractor' -count=1` passes | Fast regression for JWKS, issuer, audience, expiry, tenant claim, roles, and custom mapper behavior. |
| Keycloak | `GET /v1/me` returns `auth_method: oidc`, expected `tenant_id`, and expected roles | Preferred repeatable E2E provider for CI or staging. |
| Auth0 | Same `/v1/me` and RBAC results with Auth0 custom claims/actions | Confirms SaaS customer IdP compatibility. |

## Environment Contract

```bash
export OIDC_ISSUER_URL="http://localhost:8081/realms/hermesx"
export OIDC_CLIENT_ID="hermesx-api"
export OIDC_TENANT_CLAIM="tenant_id"
export OIDC_ROLES_CLAIM="roles"
export OIDC_ACR_CLAIM="acr"
```

`OIDC_ISSUER_URL` and `OIDC_CLIENT_ID` are mandatory when OIDC is enabled. Claim variables are optional; defaults are `tenant_id`, `roles`, and `acr`.

## Local Unit Harness

Run the existing local IdP-style tests first:

```bash
go test ./internal/auth -run 'TestOIDCExtractor' -count=1
```

Acceptance:
- Valid RS256 ID token maps `sub` to `identity` and `user_id`.
- `tenant_id` is required.
- `roles` accepts string or array values.
- Wrong audience, expired token, and invalid JWT all fail closed.

## Keycloak E2E Plan

Start Keycloak:

```bash
docker run --rm --name hermesx-keycloak \
  -p 8081:8080 \
  -e KEYCLOAK_ADMIN=admin \
  -e KEYCLOAK_ADMIN_PASSWORD=admin \
  quay.io/keycloak/keycloak:latest start-dev
```

Create a `hermesx` realm, a public client named `hermesx-api` with direct access grants enabled, and a user named `alice` with:

| Field | Value |
|-------|-------|
| password | `password` |
| user attribute `tenant_id` | `tenant-acme` |
| user attribute `roles` | `admin,auditor` |
| user attribute `acr` | `urn:mfa:present` |

Add Keycloak OIDC protocol mappers so the three user attributes are included in the ID token as `tenant_id`, `roles`, and `acr`.

Fetch an ID token:

```bash
export OIDC_ID_TOKEN="$(
  curl -sS -X POST \
    "$OIDC_ISSUER_URL/protocol/openid-connect/token" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -d grant_type=password \
    -d client_id="$OIDC_CLIENT_ID" \
    -d username=alice \
    -d password=password \
  | jq -r '.id_token'
)"
```

Run HermesX with OIDC enabled, then verify identity:

```bash
curl -sS http://localhost:8080/v1/me \
  -H "Authorization: Bearer $OIDC_ID_TOKEN" \
  | jq .
```

Expected response fields:

```json
{
  "tenant_id": "tenant-acme",
  "identity": "<alice-sub>",
  "roles": ["admin", "auditor"],
  "auth_method": "oidc"
}
```

## Auth0 E2E Plan

Configure an Auth0 application/API pair so the issued ID token has:

| Claim | Source |
|-------|--------|
| `tenant_id` | Auth0 Action or app metadata |
| `roles` | Auth0 Action mapping assigned roles to a string array or comma-separated string |
| `acr` | Auth0 MFA policy/action output where available |

Run HermesX with:

```bash
export OIDC_ISSUER_URL="https://<tenant>.<region>.auth0.com/"
export OIDC_CLIENT_ID="<auth0-client-id>"
```

Repeat the `/v1/me`, admin route, and negative token cases below with the Auth0 ID token.

## Required Test Cases

| Case | Request | Expected |
|------|---------|----------|
| Valid OIDC token | `GET /v1/me` with Keycloak/Auth0 ID token | `200`, `auth_method=oidc`, mapped `tenant_id`, mapped roles |
| Wrong audience | ID token minted for another client | `401`, audit log has auth failure |
| Expired token | Expired ID token | `401`, no API key fallback |
| Missing tenant claim | ID token without mapped tenant claim | `401` |
| Role mapping | OIDC token with `auditor` role calls `GET /v1/audit-logs` | `200` or route-specific success |
| RBAC denial | OIDC token with only `user` role calls admin route | `403` |
| API key fallback | Non-JWT API key token still authenticates through `APIKeyExtractor` | Existing API key flows unchanged |
| JWKS rotation | Rotate Keycloak signing key, mint new token, retry `/v1/me` | New token accepted after provider JWKS refresh |

## Evidence To Record

Capture the following in the release or CI artifact:

- Provider name and issuer URL.
- HermesX env vars used, excluding secrets.
- `/v1/me` response with token redacted.
- Negative-case HTTP status table.
- `go test ./internal/auth -run 'TestOIDCExtractor' -count=1` output.
- Any provider setup export, with client secrets redacted.
