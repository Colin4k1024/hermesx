# Enterprise Release Evidence - 2026-06-09

## Scope

This artifact records local release-gate evidence collected after the enterprise platform audit. It closes local regression evidence for tenant SQL and audit archival parity, and records which enterprise gates still require live infrastructure.

## Environment

| Item | Value |
|---|---|
| Date | 2026-06-09 |
| Workspace | `/Users/jiafan/Desktop/poc/hermes-agent-go` |
| Branch | `main` |
| Docker | `Docker version 29.5.2`, `Docker Compose version v5.1.4` |
| Helm | `v3.9.4+gdbc6d8e` |

## Local Gates

| Gate | Command | Result | Notes |
|---|---|---|---|
| Go test suite | `go test ./...` | Pass | All packages passed or had no test files. |
| PostgreSQL tenant SQL static check | `scripts/check_tenant_sql.sh` | Pass | `OK: All SQL queries on tenant-scoped tables include tenant_id filter.` |
| MySQL tenant SQL static check | `scripts/check_tenant_sql_mysql.sh` | Pass | `OK: All MySQL SQL queries on tenant-scoped tables include tenant_id filter.` |
| OIDC local harness | `go test ./internal/auth -run 'TestOIDCExtractor' -count=1` | Pass | Validates in-process OIDC/JWKS/claim-mapping cases. |
| Docker Compose render | `docker compose -f docker-compose.prod.yml config` | Pass | Rendered 250 lines to `/tmp/hermesx-compose-prod-config.txt`. |
| Helm render | `helm template hermesx deploy/helm/hermesx` | Pass | Rendered 181 lines to `/tmp/hermesx-helm-template.yaml`. |
| PITR compose render | `docker compose -f deploy/pitr/docker-compose.pitr.yml config` | Pass | Rendered 65 lines to `/tmp/hermesx-pitr-compose-config.txt`. |
| Focused GDPR/alert store tests | `go test ./internal/api ./internal/metering ./internal/store/pg -count=1` | Pass | Covers paginated AlertEvents export path. |
| Web UI typecheck | `npm --prefix webui run typecheck` | Pass | Admin Governance UI compiles. |
| Web UI lint | `npm --prefix webui run lint` | Pass with warnings | 0 errors; 4 existing Fast Refresh warnings in router files. |
| Web UI production build | `npm --prefix webui run build` | Pass | Vite build emitted `Governance-*.js`; no Rollup native optional dependency failure reproduced. |
| Governance UI smoke | `npm exec -- playwright test e2e/authenticated.spec.ts --grep "Governance"` from `webui/` | Pass | Desktop and mobile projects passed; screenshots written under `webui/e2e/screenshots/`. |

## Data Isolation Closure

The two highest-priority audit items from the source report have been revalidated against the current branch:

- PostgreSQL tenant SQL checker passes.
- MySQL tenant SQL checker passes.
- PostgreSQL audit archival global retention queries carry adjacent `tenant_sql_check:skip` rationale.
- MySQL audit archival implements `ArchiveOlderThan` and `ArchiveCount`.

These items should remain release gates through `scripts/check_tenant_sql.sh`, `scripts/check_tenant_sql_mysql.sh`, and focused store tests.

## OIDC Evidence

Local unit evidence is green:

```text
ok  	github.com/Colin4k1024/hermesx/internal/auth	0.892s
```

Live IdP evidence is still open. Keycloak/Auth0 runs were not executed in this local pass because no live IdP realm/client was configured for this workspace. Before enterprise sign-off, execute `docs/runbooks/enterprise-OIDC-integration-test-plan.md` and attach redacted evidence for:

- `/v1/me` with `auth_method=oidc`.
- RBAC allow and deny cases.
- wrong audience, expired token, and missing tenant claim failures.
- API key fallback.
- JWKS rotation.

## DR And Backup Evidence

`./scripts/dr-test.sh` was executed locally and failed because this workstation has no configured backup directories or live backup credentials:

```text
FAIL: PostgreSQL backup file exists - No hermes_*.sql.gz backup found in /backup/postgres
FAIL: Redis backup file exists - No backup found in /backup/redis
SKIP: Redis connectivity test - redis-cli not found
FAIL: MinIO backup directory exists - No backup found in /backup/minio
SKIP: MinIO live connectivity - mc not found or credentials not set
DR TEST RESULT: FAIL
```

`./scripts/pitr-drill.sh` was executed locally and stopped at pre-flight:

```text
Container hermesx-pg-pitr not running. Start the stack first:
docker compose -f deploy/pitr/docker-compose.pitr.yml up -d
```

This means DR remains unverified for enterprise release until run against a prepared backup/PITR environment with measured RTO/RPO.

## Release Decision

Local code and static deployment gates passed. Enterprise release sign-off is still conditional on live OIDC evidence and real backup/PITR or staging DR evidence.
