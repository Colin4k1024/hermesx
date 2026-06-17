# Changelog

All notable changes to HermesX are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - v2.4.0-dev

Release state: these entries describe current-branch changes after the latest released baseline (`v2.3.0`). They are not stable release commitments until a `v2.4.0` release is cut.

### Added

- **Supply-chain artifacts** — Dependabot config for Go/npm/GitHub Actions/Docker, CodeQL security analysis, CycloneDX SBOM workflow, and release artifact provenance attestation.
- **Egress denied-event audit** — `SecureTransport` denied decisions now persist as `audit_logs` entries with action `egress.denied`; `/admin/v1/egress/blocked-log` returns those persisted audit events.
- **Helm production hardening controls** — chart supports existing Secret references for sensitive env vars, configurable ServiceAccount, pod/container security context, read-only root filesystem with `/tmp` emptyDir, and optional NetworkPolicy.
- **Maintainer and security response docs** — added CODEOWNERS, MAINTAINERS, and explicit security response targets.
- **Admin Usage Aggregation API** — `GET /admin/v1/usage?tenant_id=&granularity=daily|monthly&from=&to=` endpoint for per-tenant token usage and cost aggregation
- **K8s Job Sandbox Mode** — `SANDBOX_MODE=k8s-job` executes tool code via Kubernetes Job API, no privileged containers or DinD required, compatible with GKE Autopilot / EKS Fargate
- **Pre-built Observability Stack** — Grafana dashboard JSON (7 panels), Prometheus alert rules (5), OTel Collector config, one-click deploy via `docker-compose.observability.yml`
- **Redis/MinIO Backup Scripts** — `scripts/redis-backup.sh` (BGSAVE + S3), `scripts/minio-backup.sh` (mc mirror), `scripts/dr-test.sh` (disaster recovery verification)
- **Eino 0.9 Agent Main Path** — native provider model, AgenticMessage blocks, TurnLoop checkpoint resume, and PG/MySQL checkpoint stores; `/v1/agent/chat` supports `include_agentic_blocks` debug output

### Breaking

- **SaaS-only product surface** — the public entry points are now `hermesx saas-api`, HTTP APIs, embedded WebUI, and SaaS Docker/Helm/K8s deployments; former local assistant commands, the gateway runtime, quickstart stack, separate frontend container, and non-SaaS Dockerfiles are no longer supported interfaces
- **Release artifact consolidation** — Release/CI artifacts now build only Linux SaaS service binaries; `Dockerfile.saas` is the only published image path, defaults to `saas-api`, and bundles `webui/dist` into `/static`
- **Code execution no longer falls back to host execution** — `execute_code` returns an error when `SANDBOX_MODE` is unset; production should use `SANDBOX_MODE=k8s-job`, while `local` mode requires explicit local SaaS development opt-in outside production

### Fixed

- **API Key Generation Safety** — `generateRawKey()` changed from panic to returning `(string, error)`; `rand.Read` failure now returns HTTP 500 instead of crashing the process
- **Agent Chat Failure Persistence** — `/v1/agent/chat` now persists user/assistant turns only after a successful agent run; runtime and streaming failures no longer leave half-written messages, duplicate resume user turns, or dirty token counters
- **Agent Chat Same-Session Concurrency** — requests for the same `tenant/session` are serialized at the handler layer to avoid duplicate message, checkpoint, and token writes
- **Workflow Agent Default Path** — `agent_task` default executor now uses `EinoAgentExecutor` instead of the legacy AIAgent path, sharing the `RunConversationTurnLoopSafe` main path with the API

### Security

- **BrowserBackend SecretResolver integration** — `BrowserBackend.Connect(ctx context.Context, tctx *ToolContext) error` new interface; credentials resolved via `SecretResolver` first, `os.Getenv` fallback; all 20 call sites updated; `browser_impl.go` `os` package dependency eliminated
- **Admin DI injectable safety components** — `APIServerConfig` now accepts optional `LeakScanner`, `CanaryDetector`, and `PolicyStore` injection points; nil-guarded, backward compatible
- **HTTP redirect bypass protection (#37)** — `agent.go CheckRedirect` hook validates redirect target via `egress.ValidateRedirectTarget`; rejects `Location` headers resolving to loopback/private/CGNAT/link-local addresses
- **RLS FORCE hardening (#27)** — migration 109: `execution_receipts` `FORCE ROW LEVEL SECURITY`; migration 110: `egress_rules` `ENABLE + FORCE RLS` + `tenant_isolation_egress_rules` policy (USING + WITH CHECK)
- **MCP SamplingHandler safety gates (#47)** — `NewSamplingHandlerWithSafety(client, interceptor)` checks input before and output after LLM calls via `SafetyInterceptor`; blocked requests return JSON-RPC error `-32000`
- **Linter compliance (#44)** — `osv_check.go` init() `OSV_ENDPOINT` public-endpoint `os.Getenv` annotated with `//nolint:forbidigo`

### Docs

- **README positioning** — reframed HermesX as an Agent-first Runtime Control Plane with audience, pillars, minimal demo, capability matrix, and explicit release-state separation
- **Agent-first architecture overview** — added `docs/AGENT_FIRST_ARCHITECTURE.md` with runtime/control-plane/workflow/governance boundaries
- **Version alignment** — aligned README, OpenAPI, and Enterprise Readiness docs around `v2.4.0-dev` current docs/API baseline and `v2.3.0` latest released baseline
- **RBAC role-count consistency** — updated docs that still described the older five-role model after platform/security/billing/ops governance roles were added

---

## [2.3.0] - 2026-05-20

Latest released baseline referenced by the public README release-state note.

SaaS Cron Scheduler — distributed scheduled task execution engine.

### Added

- **SaaS Cron Scheduler** — distributed cron job execution engine built on gocron + Redis distributed locks
  - `internal/scheduler/` package: `SaasScheduler`, `syncOnce`, `execute`, `execWithTenant`
  - PG poll-sync: every 30s fetches enabled jobs from `cron_jobs` table, auto-registers/updates/removes gocron schedules
  - Idempotent execution: `ON CONFLICT (cron_job_id, scheduled_at) DO NOTHING` prevents duplicate runs
  - Redis distributed lock: `redislock.WithTries(1)` no-retry contention, losing pods skip gracefully
  - Result delivery: `ResultDeliverer` interface pushes execution results back to user's source platform (Telegram/Discord/Slack, etc.)
  - Lifecycle context: Scheduler holds its own `ctx/cancelFunc`; gocron tasks use `baseCtx()` for live context at fire time
- **cron_job_runs table** — execution record persistence with `status`, `duration_ms`, `result`, `error`, `pod_id`
- **RLS write policies (Migration 105)** — INSERT/UPDATE/DELETE policies for `cron_job_runs` via `current_setting('app.current_tenant')`
- **SECURITY DEFINER function (Migration 106)** — `scheduler_cleanup_stale_runs()` for cross-tenant stale run cleanup

### Fixed

- **Executor RLS bypass** — all scheduler writes go through `execWithTenant()` which sets `SET LOCAL app.current_tenant` within the transaction, compatible with FORCE RLS
- **Shutdown ordering** — `cronScheduler.Stop()` executes before `syncCancel()`, ensuring running tasks drain before context cancellation
- **gocron task stale context** — uses `s.baseCtx()` closure instead of sync-time captured context, preventing expired context at fire time
- **CronJobStore error types** — `Get`/`Update`/`Delete` return unified `store.ErrNotFound` instead of `fmt.Errorf`

### Changed

- `cleanupStaleRuns` refactored from direct SQL UPDATE to calling `scheduler_cleanup_stale_runs($1)` SECURITY DEFINER function

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
  - execution receipts are created by the internal tool execution path via `DispatchWithReceipt()`
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
