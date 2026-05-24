# Enterprise Readiness Matrix

> HermesX - Agent-first Runtime Control Plane
> Current docs/API baseline: `v2.4.0-dev`
> Latest released baseline: `v2.3.0`
> Last updated: 2026-05-24

This document is evidence-based. "Released" means part of the latest released baseline documented in the changelog. "Unreleased" means present in the current branch or the changelog's Unreleased section, but not part of the latest stable baseline until `v2.4.0` is cut.

## Summary

| Capability | Current status | Release state | Evidence | Follow-up |
|------------|----------------|---------------|----------|-----------|
| Multi-tenancy | Done | Released baseline | PostgreSQL RLS migrations, tenant middleware, tenant-scoped stores, integration tests under `tests/integration/` | Keep live PostgreSQL RLS tests in CI |
| Auth, API keys, RBAC | Done | Released baseline plus OIDC plan | `internal/auth/`, `internal/middleware/rbac.go`, API key scopes, `RBAC_MATRIX.md`, [OIDC integration test plan](runbooks/enterprise-OIDC-integration-test-plan.md) | Execute Keycloak/Auth0/local IdP E2E and attach evidence |
| Agent runtime | Done | Released baseline plus Unreleased Eino path | `internal/agent/`, chat APIs, tool loop, skills, memory, MCP, changelog Unreleased Eino 0.9 entry | Treat Eino 0.9 behavior as `v2.4.0-dev` until release |
| Execution receipts and audit | Done | Released baseline | Execution receipt store/API, audit middleware/store, trace correlation documented in API/reference docs | Add longer retention/export policy if regulated deployments require it |
| Workflow and human tasks | Done | Released baseline plus Unreleased Eino default executor | `internal/workflow/`, workflow stores, OpenAPI workflow paths, `workflow-guide.en.md` | Keep workflow Eino executor release notes separated from stable workflow API |
| Sandbox isolation | Done | Released baseline plus Unreleased K8s Job mode | Local/Docker sandbox policy and tenant controls; K8s Job mode listed in Unreleased | Validate cluster RBAC, image policy, and resource limits before production use |
| Egress control | Done | Current branch `v2.4.0-dev` | `SecureTransport`, tenant allowlist rules, production `deny-all` default, `HERMES_EGRESS_DEFAULT` override | Add live allowlist smoke test in production-like deployment |
| Metering and usage | Partial | Released tenant usage plus Unreleased admin aggregation | `usage_records`, tenant usage API, admin aggregation listed in Unreleased | Clarify billing-system integration remains out of scope |
| Observability | Done | Released baseline plus Unreleased pre-built pack | Prometheus metrics, OTel tracing, structured logging, [Grafana dashboard](../deploy/grafana/dashboards/hermesx-overview.json), [alerts](../deploy/prometheus/alerts.yml), [Prometheus config](../deploy/prometheus/prometheus.yml) | Import dashboard and dry-run alerts in staging |
| Backup and disaster recovery | Partial | Released PG backup baseline plus Unreleased Redis/MinIO scripts | PG backup/restore docs, [scripts/dr-test.sh](../scripts/dr-test.sh), [scripts/pitr-drill.sh](../scripts/pitr-drill.sh), [PITR runbook](runbooks/pg-pitr-recovery.md) | Record RTO/RPO from a production-scale restore drill |
| OpenAPI contract | Done | Current docs/API baseline `v2.4.0-dev` | `internal/api/openapi.go`, `GET /v1/openapi`, OpenAPI tests | Keep `info.version` aligned with README release-state note |
| CI and security gates | Done | Released baseline | Go tests, integration tests, race/coverage workflows, security workflow docs | Add DAST/container runtime checks if production policy requires them |

## Release-state Notes

| Area | Released baseline (`v2.3.0`) | Current branch (`v2.4.0-dev`) |
|------|------------------------------|-------------------------------|
| API versioning | Public docs identify `v2.3.0` as the latest released baseline | OpenAPI reports `2.4.0-dev` because it describes the current branch contract |
| Agent runtime | Stable chat/tool/memory/skill/MCP runtime | Eino 0.9 main path, checkpoint resume, and debug agentic blocks |
| Operations | Metrics, tracing, structured logs, PG backup/restore | Grafana dashboard, Prometheus alerts, OTel collector compose, Redis/MinIO backup scripts |
| Sandbox | Local and Docker sandbox modes | K8s Job sandbox mode |
| Admin usage | Tenant usage is available | Per-tenant admin usage aggregation is Unreleased |

## Known Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| `v2.4.0-dev` docs may be read as stable release promises | Medium | README, OpenAPI, changelog, and this matrix separate current branch from latest released baseline |
| K8s Job sandbox requires cluster-specific policy validation | Medium | Keep marked Unreleased until cluster RBAC, network policy, image policy, and resource limits are verified |
| OIDC E2E is not yet recorded against external IdPs | Medium | Run [enterprise-OIDC-integration-test-plan.md](runbooks/enterprise-OIDC-integration-test-plan.md) before enterprise sign-off |
| Grafana/Prometheus configs need live validation | Low | JSON/YAML are locally checked; import into staging and capture dashboard/alert evidence |
| Backup/DR is uneven across data stores | Medium | PG baseline exists; Redis/MinIO scripts remain Unreleased until restore drills are automated |
| Billing remains usage recording, not invoicing | Low | State usage APIs as metering/control-plane features, not a billing platform |
