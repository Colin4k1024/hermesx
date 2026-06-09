# Enterprise Audit Fix Plan

## Current Evidence Snapshot

Source report: `docs/artifacts/2026-06-09-enterprise-agent-platform-audit/harness-audit-report.md`

Local re-validation on 2026-06-09:

| Item | Current Status | Evidence |
|---|---|---|
| PostgreSQL tenant SQL static check | Closed | `scripts/check_tenant_sql.sh` passes |
| MySQL tenant SQL static check | Closed | `scripts/check_tenant_sql_mysql.sh` passes |
| PostgreSQL audit archival exception | Closed with rationale | `internal/store/pg/auditlog.go` has `tenant_sql_check:skip` comments for global archival/delete/count paths |
| MySQL audit archival parity | Closed | `internal/store/mysql/auditlog.go` implements `ArchiveOlderThan` and `ArchiveCount` |
| OIDC live IdP evidence | Open | Runbook exists, but no captured Keycloak/Auth0 evidence artifact found |
| Release evidence bundle | Open | DR/observability/smoke scripts and docs exist, but latest release evidence artifact is not attached |
| Governance Admin UI | Open | Backend endpoints exist; `webui/src/admin/router.tsx` has no governance route/page |
| GDPR AlertEvents scalability | Open | `internal/api/gdpr.go` exports AlertEvents with `ListByTenant(ctx, tenantID, 0)` unlimited |
| LocalDualLimiter multi-replica behavior | Accepted risk, needs release policy/evidence | Code documents availability-over-precision tradeoff in `internal/middleware/dual_limiter.go` |
| Dynamic CORS | Backlog unless tenant-specific domains enter scope | Current CORS is environment-driven in `internal/api/server.go` |

External dual-model analysis was attempted per CCG, but `~/.claude/bin/codeagent-wrapper` is not installed in this environment. This plan is based on local repository evidence and command validation.

## Release Gate Classification

### Must Complete Before Enterprise Release

1. **Release evidence bundle**
   - Scope: capture fresh evidence for Docker/Compose/K8s smoke, migration/bootstrap checks, backup restore drill, Grafana/Prometheus import or dry-run, and RTO/RPO results.
   - Primary files: new artifact under `docs/artifacts/2026-06-09-enterprise-release-evidence/`; existing inputs from `docs/deployment.md`, `docs/runbooks/mysql-backup-restore.md`, `docs/runbooks/redis-minio-backup.md`, `scripts/dr-test.sh`, `scripts/pitr-drill.sh`, `deploy/prometheus/alerts.yml`, `deploy/grafana/dashboards/hermesx-overview.json`.
   - Acceptance:
     - Artifact records command, environment, timestamp, result, and redacted output.
     - Evidence explicitly states whether tests ran locally, in staging, or were deferred due missing infrastructure.
     - RTO/RPO values are measured or clearly marked as unverified.
   - Suggested commands:
     - `go test ./...`
     - `scripts/check_tenant_sql.sh`
     - `scripts/check_tenant_sql_mysql.sh`
     - `docker compose -f docker-compose.prod.yml config`
     - `helm template hermesx deploy/helm/hermesx/`
     - `./scripts/dr-test.sh`
     - `./scripts/pitr-drill.sh`

2. **OIDC live E2E evidence**
   - Scope: execute existing Keycloak/Auth0/local IdP plan and store redacted evidence.
   - Primary files: `docs/runbooks/enterprise-OIDC-integration-test-plan.md`, `internal/auth/oidc.go`, `cmd/hermesx/saas.go`, `internal/middleware/rbac.go`.
   - Acceptance:
     - Local OIDC unit harness passes.
     - Keycloak or Auth0 evidence captures `/v1/me`, RBAC allow/deny, wrong audience, expired token, missing tenant claim, API key fallback, and JWKS rotation results.
     - Tokens, client secrets, and issuer-private values are redacted.
   - Suggested commands:
     - `go test ./internal/auth -run 'TestOIDCExtractor' -count=1`
     - Keycloak/Auth0 curl commands from `docs/runbooks/enterprise-OIDC-integration-test-plan.md`

3. **Data isolation regression evidence**
   - Scope: keep the now-closed audit archival findings from regressing.
   - Primary files: `internal/store/pg/auditlog.go`, `internal/store/mysql/auditlog.go`, `internal/store/mysql/auditlog_test.go`, tenant SQL scripts.
   - Acceptance:
     - Static checks pass.
     - Store tests cover batch size normalization, archive delete parity, and count behavior.
     - `tenant_sql_check:skip` rationale remains adjacent to intentionally global audit retention queries.
   - Suggested commands:
     - `scripts/check_tenant_sql.sh`
     - `scripts/check_tenant_sql_mysql.sh`
     - `go test ./internal/store/pg ./internal/store/mysql -count=1`

### Next Platform Sprint

4. **Governance Admin UI**
   - Scope: expose backend evolution governance APIs in the Admin console.
   - Primary files:
     - `webui/src/admin/router.tsx`
     - `webui/src/admin/components/AdminShell.tsx`
     - new `webui/src/admin/pages/Governance.tsx`
     - new `webui/src/admin/hooks/useGovernance.ts`
     - `webui/src/shared/types/index.ts`
     - `webui/e2e/authenticated.spec.ts`
   - Backend endpoints already exist in `internal/api/admin/evolution.go` and are registered in `internal/api/admin/handler.go`.
   - Acceptance:
     - Global sharing policy current/history/update/rollback is usable.
     - Tenant sharing policy current/history/update/rollback is usable.
     - Shared knowledge revoke requires bounded criteria or explicit `confirm_all`.
     - UI shows service-unavailable state when `evolutionStore` is not configured.
   - Suggested commands:
     - `npm --prefix webui run typecheck`
     - `npm --prefix webui run lint`
     - `npm --prefix webui run test:e2e` if available in package scripts; otherwise run the existing Playwright command documented by the project.

5. **GDPR AlertEvents streaming/pagination**
   - Scope: replace unlimited alert event export with bounded pagination or streaming so large tenants do not create memory spikes.
   - Primary files:
     - `internal/api/gdpr.go`
     - `internal/metering/alerts.go`
     - concrete AlertEventStore implementations under `internal/store/pg`, `internal/store/mysql`, and in-memory tests.
   - Acceptance:
     - GDPR export remains complete.
     - Implementation does not require holding all alert events in memory for large tenants.
     - Tests cover multiple pages/chunks and zero-event tenants.
   - Suggested commands:
     - `go test ./internal/api ./internal/metering ./internal/store/pg ./internal/store/mysql -count=1`

6. **Admin DI full cleanup**
   - Scope: finish server-level dependency construction so admin dependencies are consistently injected and testable.
   - Primary files:
     - `internal/api/server.go`
     - `internal/api/admin/handler.go`
     - related tests in `internal/api/admin/*_test.go`
   - Current state: `AdminHandler` supports options for safety, MCP, usage, evolution, and egress; remaining risk is initialization sprawl at server assembly boundaries.
   - Acceptance:
     - Admin dependency wiring has a single construction path.
     - Nil optional dependencies produce intentional 503/disabled states.
     - Tests cover configured and unconfigured optional services.
   - Suggested commands:
     - `go test ./internal/api ./internal/api/admin -count=1`

7. **LocalDualLimiter release policy**
   - Scope: decide whether Redis outage behavior remains fail-open local fallback or becomes configurable fail-closed for strict deployments.
   - Primary files:
     - `internal/middleware/dual_limiter.go`
     - `internal/middleware/ratelimit.go`
     - `docs/adr/ADR-002-dual-layer-rate-limiter.md`
     - deployment docs for `REDIS_URL`
   - Acceptance:
     - Enterprise release docs state exact behavior during Redis outage.
     - If adding a config flag, tests cover fail-open and fail-closed behavior.
     - Multi-replica smoke evidence exists or the risk is explicitly accepted.
   - Suggested commands:
     - `go test ./internal/middleware -count=1`
     - `scripts/verify-multi-replica.sh` where Docker is available.

8. **K8s sandbox evidence**
   - Scope: validate `SANDBOX_MODE=k8s-job` against cluster RBAC, image policy, network policy, and resource limits before enterprise release claims rely on it.
   - Primary files:
     - sandbox implementation files under `internal/tools/environments`
     - `docs/configuration.md`
     - `docs/ENTERPRISE_READINESS.md`
     - Helm/K8s manifests under `deploy/`
   - Acceptance:
     - Evidence artifact records allowed and denied sandbox executions.
     - Claims remain marked `v2.4.0-dev` or unreleased until validation is captured.

### Backlog / Product Candidates

9. **Dynamic CORS**
   - Trigger only if tenant-specific custom domains are in release scope.
   - Current env-based CORS remains acceptable for single-domain or operator-managed multi-domain deployments.

10. **Usage dashboard and billing polish**
   - User usage and admin aggregate endpoints exist; keep billing/invoicing claims out of enterprise release language unless invoicing rules are implemented.

11. **GDPR self-service UI**
   - Backend export/delete/restore exists; UI is product polish unless self-service compliance workflow is part of the release contract.

12. **Remote skill hub sync and WASM sandbox**
   - Non-blocking unless customer security requirements make WASM mandatory.

## Recommended Execution Order

1. Re-run and attach data isolation evidence so the two former High findings are formally closed.
2. Produce the release evidence bundle skeleton and fill all local commands first.
3. Run OIDC local harness, then Keycloak E2E; add Auth0 only if a sandbox tenant is available.
4. Implement Governance Admin UI as the first code-bearing sprint item.
5. Implement GDPR AlertEvents streaming/pagination.
6. Decide LocalDualLimiter strict mode vs documented accepted risk.
7. Clean Admin DI initialization if implementation work touches admin server assembly.
8. Update `docs/ENTERPRISE_READINESS.md` and English counterpart with exact evidence links and remaining caveats.

## Review And Quality Gates

- Any code implementation phase touching auth, tenant isolation, admin governance, or database paths requires double-model review when the wrapper is available.
- Minimum local gates before handoff:
  - `go test ./...`
  - `scripts/check_tenant_sql.sh`
  - `scripts/check_tenant_sql_mysql.sh`
  - `npm --prefix webui run typecheck`
  - `npm --prefix webui run lint`
- For frontend changes, capture desktop and mobile Playwright screenshots for the affected admin page.
- For evidence-only changes, review the artifact for redacted secrets and exact command outputs.
