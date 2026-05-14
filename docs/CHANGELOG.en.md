# Changelog

All notable changes to HermesX are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

---

## [2.2.0] - 2026-05-14

Security hardening, bootstrap stabilization, and supply-chain improvements.

### Added

- IP-level rate limiting for `POST /admin/v1/bootstrap`, with matching Nginx limits for WebUI and production load-balancer entrypoints.
- Cross-replica bootstrap idempotency via `bootstrap_state` in PostgreSQL (`ON CONFLICT DO NOTHING` + `RETURNING id` sentinel).
- Session titles for SaaS chat sessions, surfaced in the WebUI conversation sidebar.
- `internal/store/pg` unit tests: compile-time interface assertions, bootstrap idempotency logic, and SQL shape validation for scopes (`COALESCE`) and `ON CONFLICT` idempotency.
- OpenAPI spec corrections: title/version/contact updated to HermesX v2.2.0; 11 missing routes added (`/health/live`, `/health/ready`, `/metrics`, `/v1/agent/chat`, `/v1/gdpr/cleanup-minio`, all `/admin/v1/*` endpoints); paths now accurate at 33+ documented routes.

### Fixed

- PostgreSQL API key persistence now writes and reads `scopes`, allowing admin API keys to satisfy `RequireScope("admin")`.
- Release workflow now builds with Go 1.25 and generates checksums for `hermesx-*` artifacts.
- Documentation now reflects the actual React WebUI stack and v2.2.0 baseline.
- WebUI security headers (CSP, HSTS, X-Frame-Options), URL encoding, auth retry on 401, and N+1 query eliminated in session list.

### Changed

- `HasScope` compatibility policy documented: empty `Scopes` = legacy key granted non-admin access; `admin` scope always requires explicit grant. New keys carry explicit scopes.
- Memory `Curator.deduplicateEntries`: Phase 1 now O(n) exact-key dedup via map; Phase 2 content-similarity scan limited to key-unique set (m ≤ n), resolving O(n²) worst case at `MaxMemories > 100`.

### Security

- GitHub Actions supply-chain hardening: `actions/checkout`, `actions/setup-go`, and `softprops/action-gh-release` all pinned to full commit SHA.
- Bootstrap endpoint now enforces 5 RPM IP rate limit at application layer, Nginx WebUI layer, and production LB layer.

---

## [2.0.0] - 2026-05-08

Major release: complete rebrand from Hermes to HermesX, combined with enterprise hardening Phase 1.

### Added

- **ExecutionReceipt API**: auditable tool invocation records with idempotency deduplication and trace correlation
  - `POST /v1/execution-receipts` — create receipt via `DispatchWithReceipt()`
  - `GET /v1/execution-receipts` — list with pagination and filters (auditor role)
  - `GET /v1/execution-receipts/{id}` — get by ID (auditor role)
- **Prometheus business metrics**: 11+ custom metrics covering HTTP requests, LLM completions, tool executions, rate limiting, and store operations
- **MiniMaxi Anthropic API mode**: Anthropic API-compatible mode via MiniMaxi provider, including stress test validation
- **`auditor` RBAC role**: read-only access to audit logs and execution receipts
- **Full OpenAPI specification**: 22 documented endpoints with schemas, tags, and security schemes, available at `GET /v1/openapi`
- **Production Docker compose**: `docker-compose.prod.yml` with PostgreSQL 16, Redis 7 (AOF + LRU), MinIO, OTel Collector, and Jaeger
- **Enterprise demo script**: 11-step `./examples/enterprise-saas-demo/demo.sh` walkthrough
- **Backup/restore scripts**: `scripts/backup/backup.sh` (pg_dump + gzip, 7-day retention) and `scripts/backup/restore.sh` (single-transaction restore)

### Changed

- **Project name**: Hermes → HermesX — independent enterprise agent platform
- **Binary name**: `hermes` → `hermesx`
- **Entry point**: `cmd/hermes/` → `cmd/hermesx/`
- **GitHub repository**: `https://github.com/Colin4k1024/hermesx.git`
- **All internal references**: package imports, variable names, comments, log messages, and environment variables updated from `hermes`/`HERMES` to `hermesx`/`HERMESX`
- **Configuration files**: `docker-compose.yml`, `.env.example`, and CI workflow files updated
- **Documentation**: all docs reflect HermesX branding and v2.0.0 version

### Fixed

- **CI RLS pool URL replacement**: corrected credential substitution for `hermesx_test` in CI environment (was failing lint and integration tests)
- **API key tenant boundary**: tenant derivation is now strictly from credential context; body-supplied `tenant_id` only honored for admin role callers
- **`generateRawKey()` hardening**: explicit panic on `crypto/rand.Read` failure (previously silently returned partial key)

### Refactored

- **Complete codebase rebrand**: hermes → hermesx across all source files, test files, configs, and scripts

### Docs

- **ARCHITECTURE.md**: system architecture overview with component diagram and data flows
- **SECURITY_MODEL.md**: threat model, authentication chain, RLS, and sandbox isolation
- **RBAC_MATRIX.md**: 5 roles × 10 resources permission matrix
- **ENTERPRISE_READINESS.md**: Phase 1 enterprise readiness assessment — 12 capability areas with evidence
- **STRESS_TEST_REPORT.md**: MiniMaxi Anthropic API mode stress test results
- **Expanded DEPLOYMENT.md**: environment variable reference, Prometheus metrics table, backup/restore procedures, horizontal scaling guidelines, security hardening checklist, and rollback strategy
