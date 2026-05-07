# Enterprise Readiness Matrix

> HermesX — Enterprise Agent Runtime / SaaS Control Plane
> Version: v1.3.0 | Last Updated: 2026-05-07

---

## 1. Multi-Tenancy

**Capability**: Full tenant isolation across all data paths  
**Status**: Done  
**Evidence**:
- 10 tables with Row-Level Security (RLS) enabled
- 37 RLS policies enforcing `app.current_tenant` session variable
- `withTenantTx()` helper sets `SET LOCAL app.current_tenant` per transaction
- TenantMiddleware derives tenant_id from AuthContext (not headers)
- 58 integration tests validating cross-tenant isolation
- Dedicated test suites: `tenant_isolation_test.go`, `rls_policy_test.go`, `cross_tenant_attack_test.go`

**Risk**: No automated RLS regression test in CI (requires live PG)  
**Next Action**: CI integration test job with PG service already wired

---

## 2. Auth / API Key / RBAC

**Capability**: Chain authentication with scoped API keys and role-based access  
**Status**: Done  
**Evidence**:
- Auth chain: Static Token → API Key (SHA-256 hashed) → JWT
- API Key supports: scopes, roles, expiry, revocation
- Roles: `super_admin`, `admin`, `owner`, `user`, `auditor`
- `HasScope()` enforces scope-based access per endpoint
- Tenant boundary enforcement: non-admin callers cannot specify foreign `tenant_id`
- `generateRawKey()` panics on `crypto/rand.Read` failure

**Risk**: JWT validation is prepared but not production-tested with real IdP  
**Next Action**: OIDC integration test with Keycloak/Auth0

---

## 3. Rate Limit / Quota

**Capability**: Per-tenant and per-user rate limiting with distributed enforcement  
**Status**: Done  
**Evidence**:
- `RateLimiter` interface with Redis sliding window (Lua atomic script)
- `DualLayerLimiter` for simultaneous tenant + user limits
- Local LRU fallback when Redis unavailable
- Per-tenant override via `TenantLimitFn`
- Prometheus counter: `hermes_rate_limit_rejected_total`
- Pressure tested: 100 concurrent × 5 minutes, accuracy > 95%

**Risk**: No per-endpoint granularity (all requests share one bucket)  
**Next Action**: Endpoint-aware rate limiting (P2)

---

## 4. Metering / Billing

**Capability**: Token-level usage recording with aggregation queries  
**Status**: Done  
**Evidence**:
- `UsageRecorder` with async batch persistence (buffered channel + periodic flush)
- `usage_records` table: tenant_id, user_id, session_id, model, input/output tokens, cost
- `UsageV2Handler`: `GET /v1/usage` with from/to/granularity (hour/day/month)
- Per-tenant aggregation queries with time bucketing
- Migrations v62-64 for schema + indexes

**Risk**: No billing system integration (recording only, no invoicing)  
**Next Action**: Stripe/billing webhook integration (P2)

---

## 5. Audit / Compliance

**Capability**: Immutable audit trail for all state-changing operations  
**Status**: Done  
**Evidence**:
- `audit_logs` table with RLS
- All POST/PUT/DELETE operations generate audit entries
- Fields: actor, action, resource_type, resource_id, metadata, tenant_id, timestamp
- `AuditLogStore` with Create/List/filtering
- `GET /v1/audit-logs` gated behind `auditor` role
- Execution receipts provide tool-level audit trail

**Risk**: No tamper-proof guarantee (append-only but not cryptographically chained)  
**Next Action**: Optional hash-chain verification for regulated environments (P2)

---

## 6. GDPR / Data Lifecycle

**Capability**: Full data export and deletion per tenant  
**Status**: Done  
**Evidence**:
- `GET /v1/gdpr/export` — exports all tenant data as JSON
- `DELETE /v1/gdpr/delete` — transactional deletion of sessions, messages, memories, api_keys, audit_logs, cron_jobs
- Deletion cascades through all related tables in single transaction
- Admin-only access control

**Risk**: No data retention policy engine (manual deletion only)  
**Next Action**: Automated retention sweep based on tenant configuration (P2)

---

## 7. Observability

**Capability**: Full-stack metrics, tracing, and structured logging  
**Status**: Done  
**Evidence**:
- Prometheus metrics: 11 custom business metrics (HTTP, LLM, tools, rate limiting, sessions, store)
- OpenTelemetry tracing: HTTP → middleware → store → LLM full chain
- PGX tracer for database query spans
- OTel Collector config: traces → Jaeger, metrics → Prometheus
- Structured JSON logging via `slog`
- Memory limiter (512MB) on collector
- Alert rule examples in deployment guide

**Risk**: No pre-built Grafana dashboards  
**Next Action**: Dashboard JSON templates (P1)

---

## 8. Sandbox Isolation

**Capability**: Isolated code execution with resource limits  
**Status**: Done  
**Evidence**:
- Local sandbox: process isolation with timeout, output truncation (50KB), env stripping
- Docker sandbox: `--network=none`, `--memory`, `--cpus` limits
- Per-tenant `SandboxPolicy` (JSONB on tenants table): enabled, max_timeout, allowed_tools, restrict_network
- `AllowedTools` enforcement: non-whitelisted tools rejected
- Max tool calls limit (default 50)
- Skill metadata `sandbox: required` triggers Docker execution

**Risk**: Docker sandbox requires Docker-in-Docker or socket mount in containerized deploys  
**Next Action**: gVisor/Firecracker evaluation for production (P2)

---

## 9. Backup / Disaster Recovery

**Capability**: Automated backup with point-in-time restore capability  
**Status**: Done  
**Evidence**:
- `scripts/backup/backup.sh`: pg_dump + gzip, 7-day retention, configurable output dir
- `scripts/backup/restore.sh`: single-transaction restore with post-migration
- `deploy/pitr/` templates for WAL archiving
- Production compose includes volume persistence for PG/Redis

**Risk**: No automated DR drill in CI; RPO depends on backup frequency  
**Next Action**: Automated weekly restore verification (P1)

---

## 10. CI / Security

**Capability**: Automated build, test, security scan pipeline  
**Status**: Done  
**Evidence**:
- `.github/workflows/ci.yml`: build + vet + test + coverage + race detection + Docker push
- `.github/workflows/security.yml`: govulncheck + gosec + trivy (weekly + PR)
- Integration test job with PG/Redis/MinIO services
- Build matrix: linux/darwin × amd64/arm64 + windows/amd64
- 21 test packages, all passing

**Risk**: No DAST or container runtime scanning  
**Next Action**: Add OWASP ZAP scan against deployed instance (P2)

---

## 11. Known Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| JWT/OIDC not production-tested | Medium | Auth chain works; needs IdP integration test |
| No billing integration | Low | Usage recording is complete; billing is business logic |
| Docker sandbox in containers | Medium | Local sandbox is always available as fallback |
| Single PG writer (no read replicas) | Low | Sufficient for < 500 req/s; PgBouncer for connection pooling |
| No Grafana dashboards | Low | Metrics exposed; dashboards are configuration |

---

## 12. Roadmap

### v1.4.0 (P1 — Next)
- [ ] OIDC integration test with real IdP
- [ ] Grafana dashboard templates
- [ ] Automated DR verification in CI
- [ ] Endpoint-aware rate limiting

### v2.0.0 (P2 — Future)
- [ ] Billing/invoicing webhook
- [ ] Data retention policy engine
- [ ] gVisor sandbox backend
- [ ] Read replica support
- [ ] Multi-region deployment guide

---

## Store Interface Coverage

The `Store` interface covers all core SaaS state objects:

| Sub-Store | Operations | RLS |
|-----------|-----------|-----|
| Sessions | Create, Get, List, Delete, AppendMessage, ListMessages | Yes |
| Tenants | Create, Get, List, Update, Delete | Yes |
| APIKeys | Create, Get, List, Revoke, GetByHash | Yes |
| AuditLogs | Create, List | Yes |
| Memories | Set, Get, List, Delete | Yes |
| UserProfiles | Get, Set, Delete | Yes |
| CronJobs | Create, Get, List, Update, Delete | Yes |
| Roles | Assign, Revoke, List | Yes |
| ExecutionReceipts | Create, Get, List, GetByIdempotencyID | Yes |

---

## API Surface

22 documented endpoints across:
- Health (3): `/v1/health`, `/v1/health/live`, `/v1/health/ready`
- Chat (1): `/v1/chat/completions` (OpenAI-compatible)
- Sessions (1): `/v1/sessions`
- Tenants (1): `/v1/tenants`
- API Keys (1): `/v1/api-keys`
- Audit (1): `/v1/audit-logs`
- Execution Receipts (1): `/v1/execution-receipts`
- Usage (1): `/v1/usage`
- GDPR (2): `/v1/gdpr/export`, `/v1/gdpr/delete`
- Metrics (1): `/v1/metrics`
- OpenAPI (1): `/v1/openapi`
- Admin (4): `/admin/v1/tenants`, `/admin/v1/sandbox-policy`, etc.
- Me (1): `/v1/me`

Full OpenAPI 3.0.3 spec available at `GET /v1/openapi`.
