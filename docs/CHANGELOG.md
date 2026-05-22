# Changelog

All notable changes to HermesX are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Admin Usage Aggregation API** — `GET /admin/v1/usage?tenant_id=&granularity=daily|monthly&from=&to=` 端点，按租户聚合 Token 用量及成本
- **K8s Job 沙箱模式** — `SANDBOX_MODE=k8s-job` 通过 Kubernetes Job API 执行工具代码，无需特权容器或 DinD，兼容 GKE Autopilot / EKS Fargate
- **预置可观测性栈** — Grafana Dashboard JSON（7 面板）、Prometheus 告警规则（5 条）、OTel Collector 配置、`docker-compose.observability.yml` 一键部署
- **Redis/MinIO 备份脚本** — `scripts/redis-backup.sh`（BGSAVE + S3）、`scripts/minio-backup.sh`（mc mirror）、`scripts/dr-test.sh`（灾难恢复验证）
- **Eino 0.9 Agent 主链** — 接入 native provider model、AgenticMessage blocks、TurnLoop checkpoint resume、PG/MySQL checkpoint store；`/v1/agent/chat` 支持 `include_agentic_blocks` 调试输出

### Fixed

- **API Key 生成安全性** — `generateRawKey()` 从 panic 改为返回 `(string, error)`，`rand.Read` 失败时返回 HTTP 500 而非进程崩溃
- **Agent Chat 失败持久化** — `/v1/agent/chat` 仅在 agent 成功后写入 user/assistant turn；运行失败或流式错误不会留下半写消息、重复 resume user turn 或 token 统计脏状态
- **Agent Chat 同 session 并发** — 同一 `tenant/session` 的请求在 handler 层串行执行，避免消息、checkpoint 和 token 统计双写
- **Workflow Agent 默认路径** — `agent_task` 默认 executor 从旧 AIAgent 切到 `EinoAgentExecutor`，与 API 共用 `RunConversationTurnLoopSafe` 主链

---

## [2.3.0] - 2026-05-20

SaaS Cron Scheduler — 分布式定时任务执行引擎。

### Added

- **SaaS Cron Scheduler** — 基于 gocron + Redis 分布式锁的多 Pod 定时任务执行引擎
  - `internal/scheduler/` 包：`SaasScheduler`、`syncOnce`、`execute`、`execWithTenant`
  - PG 轮询同步：每 30s 从 `cron_jobs` 表读取启用任务，自动注册/更新/移除 gocron 调度
  - 幂等执行：`ON CONFLICT (cron_job_id, scheduled_at) DO NOTHING` 防止重复执行
  - Redis 分布式锁：`redislock.WithTries(1)` 无重试竞争，失败 Pod 跳过
  - 执行结果投递：`ResultDeliverer` 接口将结果推送回用户来源平台（Telegram/Discord/Slack 等）
  - 生命周期上下文：Scheduler 持有独立 `ctx/cancelFunc`，gocron 任务通过 `baseCtx()` 获取实时上下文
- **cron_job_runs 表** — 执行记录持久化，含 `status`、`duration_ms`、`result`、`error`、`pod_id`
- **RLS 写入策略（Migration 105）** — `cron_job_runs` 表的 INSERT/UPDATE/DELETE 策略，通过 `current_setting('app.current_tenant')` 校验
- **SECURITY DEFINER 函数（Migration 106）** — `scheduler_cleanup_stale_runs()` 跨租户清理超时运行记录

### Fixed

- **Executor RLS 绕过** — 所有 scheduler 写操作通过 `execWithTenant()` 在事务内设置 `SET LOCAL app.current_tenant`，兼容 FORCE RLS
- **Shutdown 时序** — `cronScheduler.Stop()` 在 `syncCancel()` 之前执行，确保运行中任务先排空再取消上下文
- **gocron 任务上下文过期** — 使用 `s.baseCtx()` 闭包替代同步时捕获的上下文，防止 fire 时上下文已过期
- **CronJobStore 错误类型** — `Get`/`Update`/`Delete` 返回统一的 `store.ErrNotFound` 而非 `fmt.Errorf`

### Changed

- `cleanupStaleRuns` 从直接 SQL UPDATE 改为调用 `scheduler_cleanup_stale_runs($1)` SECURITY DEFINER 函数

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
