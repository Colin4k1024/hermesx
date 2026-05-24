# Enterprise Readiness Assessment

> Current docs/API baseline: `v2.4.0-dev`
> Latest released baseline: `v2.3.0`
> Last updated: 2026-05-24

This top-level file points to the maintained bilingual readiness matrices:

| Language | Document |
|----------|----------|
| English | [docs/ENTERPRISE_READINESS.en.md](docs/ENTERPRISE_READINESS.en.md) |
| Chinese | [docs/ENTERPRISE_READINESS.md](docs/ENTERPRISE_READINESS.md) |

## Summary

HermesX is documented as enterprise-ready for the latest released baseline (`v2.3.0`), with current-branch additions tracked as `v2.4.0-dev`. Unreleased capabilities include the Eino 0.9 main path, admin usage aggregation, K8s Job sandbox mode, the pre-built observability pack, and Redis/MinIO backup scripts.

Production consumers should pin an image/version, run OpenAPI smoke tests, validate tenant isolation against the target database role, verify the selected sandbox mode, and confirm backup/restore drills for every enabled data store.

## Evidence Highlights

| Area | Evidence |
|------|----------|
| Observability | [Grafana dashboard](deploy/grafana/dashboards/hermesx-overview.json), [Prometheus alerts](deploy/prometheus/alerts.yml), and [Prometheus config](deploy/prometheus/prometheus.yml) are present and locally syntax-checked. |
| Disaster recovery | [scripts/dr-test.sh](scripts/dr-test.sh), [scripts/pitr-drill.sh](scripts/pitr-drill.sh), and [pg-pitr-recovery.md](docs/runbooks/pg-pitr-recovery.md) cover PostgreSQL, Redis, and MinIO checks. |
| OIDC | [enterprise-OIDC-integration-test-plan.md](docs/runbooks/enterprise-OIDC-integration-test-plan.md) defines Keycloak/Auth0/local IdP E2E validation; live execution evidence is still required. |
