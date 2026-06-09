# Enterprise Agent Platform Audit Report - 2026-06-09

## Executive Conclusion

HermesX is already shaped like an enterprise agent runtime control plane, not a prototype chat app. The core platform capabilities are present: governed agent execution, multi-tenant SaaS control plane, RBAC/API key/OIDC wiring, audit logs, execution receipts, workflows, sandbox policy, OpenAPI, CI, deployment assets, and observability artifacts.

The remaining work is not "build the platform from scratch"; it is enterprise release hardening. The gaps are concentrated in release evidence, a PostgreSQL tenant SQL static-check failure, MySQL audit archival parity, live OIDC evidence, DR/observability rehearsals, and Admin UI/control-plane completeness.

Recommended decision: continue, but focus the next sprint on evidence and release gates before adding new agent features.

## Overall Score: 84/100

| Dimension | Score | Status | Key Finding |
|---|---:|---|---|
| Agent Runtime Capability | 89 | Good | Agent loop, tools, skills, memory, MCP, Eino path, checkpoint/resume signals are present. v2.4.0-dev items must remain clearly marked unreleased. |
| Enterprise Governance & Security | 84 | Needs hardening | RBAC/API key/OIDC/safety/egress/security scans exist, but OIDC live IdP evidence and Admin DI cleanup remain open. |
| Multi-Tenant Data Isolation | 78 | Needs action | Go tests pass and MySQL static check passes, but PostgreSQL tenant SQL checker fails on `audit_logs` archival count. MySQL audit archival is not implemented. |
| Workflow & Human-in-the-Loop | 86 | Good | Workflow definitions, immutable versions, runs, tasks, retry/cancel, and agent task executor are represented. Teams shutdown protocol remains experimental debt. |
| Operations, Observability & DR | 80 | Needs evidence | Compose/K8s/Helm, Prometheus, Grafana, OTel, backup scripts exist. Staging/live validation, RTO/RPO records, and release gate artifacts are still required. |
| Admin/Product Control Plane | 75 | Incomplete | Backend control-plane capability is stronger than the UI. Governance history/rollback/revoke UI, usage dashboard, pricing rules, and GDPR self-service UI remain backlog/candidates. |
| Documentation & Contract Evidence | 91 | Strong | README, architecture, OpenAPI tests, enterprise readiness matrix, runbooks, and docs links are coherent. Some readiness claims need sharper backend-specific caveats. |

## Evidence Sample

| Check | Result |
|---|---|
| `go test ./...` via `/usr/local/go/bin/go` | Passed |
| `scripts/check_tenant_sql.sh` | Failed: `internal/store/pg/auditlog.go:189` counts `audit_logs` without `tenant_id` |
| `scripts/check_tenant_sql_mysql.sh` | Passed |
| `npm --prefix webui run typecheck` | Passed |
| `npm --prefix webui run lint` | Passed with 4 Fast Refresh warnings |
| `npm --prefix webui run build` | Failed locally due Rollup native optional dependency/code-signing issue under Node 24; CI uses Node 20 + `npm ci` |
| Markdown link check for top-level docs | Passed, 0 missing links |
| OpenAPI evidence | `info.version` is `2.4.0-dev`; path/branding regression tests exist |

## Top Actions

1. **High: close the PostgreSQL tenant SQL static-check failure.** Decide whether `ArchiveCount` is a legitimate global archival operation. If yes, add an explicit `tenant_sql_check:skip` marker plus rationale and test coverage; if no, make it tenant-scoped.
2. **High: resolve MySQL audit archival parity.** `internal/store/mysql/auditlog.go` returns `mysql: audit archival not implemented`. Either implement archive count/batch deletion or narrow the production support claim for MySQL.
3. **High: create release evidence artifacts.** Record Docker/Compose/K8s smoke, migration/bootstrap checks, backup restore drills, Grafana/Prometheus import/dry-run, and RTO/RPO results.
4. **High: run OIDC live E2E evidence.** Execute the existing Keycloak/Auth0/local IdP plan and store redacted `/v1/me`, RBAC allow/deny, JWKS rotation, and negative-case results.
5. **Medium: finish governance/admin control plane.** Platform governance UI for policy history, rollback, revoke, and the Admin DI refactor are the next control-plane maturity items.
6. **Medium: harden scalability leftovers.** Address GDPR AlertEvents streaming pagination, multi-replica LocalDualLimiter behavior under Redis failure, and dynamic CORS when multi-domain tenants become real.
7. **Medium: clarify unreleased sandbox scope.** K8s Job sandbox needs cluster RBAC, image policy, network policy, and resource-limit validation before enterprise release claims; WASM sandbox remains deferred by ADR-006.
8. **Low: clean developer reproducibility.** Document or script local toolchain paths for Go/npm, and use clean `npm ci` to avoid Rollup optional dependency drift.

## Remaining Work By Release Gate

### Must Complete Before Enterprise Release

- PostgreSQL tenant SQL static check must pass or have reviewed, documented exceptions.
- MySQL production support must not claim audit archival until implemented or explicitly excluded.
- OIDC live E2E evidence must be captured.
- Backup/restore and observability evidence must be attached to a release artifact.
- Clean Docker/Compose/K8s smoke must be recorded after the latest commit.

### Should Complete In The Next Platform Sprint

- Admin DI full refactor to reduce singleton/test race risk.
- Platform governance UI: policy history, rollback, revoke.
- GDPR AlertEvents streaming export.
- Teams agent shutdown protocol.
- Dynamic CORS if tenant-specific domains are entering scope.

### Product Candidates, Not Blockers

- Admin pricing rules UI.
- Usage dashboard / billing report UI.
- GDPR self-service export UI.
- Remote skill hub sync.
- WASM sandbox, unless security requirements make it mandatory.

## Decision

HermesX can credibly be presented as an enterprise agent runtime control-plane candidate. It should not yet be treated as fully release-ready for regulated enterprise deployment until the evidence gates above are closed.

