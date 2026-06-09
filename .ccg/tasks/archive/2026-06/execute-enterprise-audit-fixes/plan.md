# Execution Plan

## Phase 1: Evidence Bundle

- Create `docs/artifacts/2026-06-09-enterprise-release-evidence/`.
- Run and record local evidence for Go tests, tenant SQL checks, OIDC unit harness, Docker Compose config, Helm template if available, and DR scripts where dependencies exist.
- Record unavailable infrastructure explicitly instead of overstating release readiness.

## Phase 2: Governance Admin UI

- Add typed API hooks for evolution governance.
- Add Admin Governance page for global policy, tenant policy, history, rollback, and shared revoke.
- Register route and sidebar item.
- Add e2e mock coverage for the page.

## Phase 3: GDPR AlertEvents Pagination

- Extend AlertEventStore with paginated listing.
- Update concrete stores and tests.
- Change GDPR export to collect AlertEvents in bounded pages instead of `limit=0` unlimited query.

## Phase 4: Docs And Policy

- Update enterprise readiness docs with the new release evidence artifact and precise caveats.
- Clarify LocalDualLimiter Redis-outage policy in deployment/ADR docs if needed.

## Phase 5: Verification

- Run Go, tenant SQL, webui typecheck, webui lint, and focused tests.
- Review diff for scope.
- Archive the CCG task.
