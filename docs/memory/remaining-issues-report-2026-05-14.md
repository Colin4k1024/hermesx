# Remaining Issues Report - 2026-05-14

**Scope**: current `main` after commit `cec6583` (`fix: stabilize bootstrap security and release workflow`)  
**Basis**: `docs/memory/project-context.md`, `docs/memory/backlog.md`, current route/spec code, and source TODO scan  
**Status**: no known blocking Go/WebUI build failure. Remaining work is release validation, API contract cleanup, security hardening, tests, performance debt, and backlog hygiene.

---

## Executive Summary

The v2.2.0 stabilization patch closed the previously critical WebUI/bootstrap leftovers:

- Bootstrap IP rate limiting is implemented at application and Nginx layers.
- Bootstrap cross-replica idempotency is implemented through `bootstrap_state`.
- PostgreSQL API key scopes are persisted and read back.
- Chat sessions now have readable titles in the WebUI.

The remaining issues are not one single blocker. They fall into four groups:

1. **Release readiness**: full Docker/K8s smoke has not been recorded after the latest stabilization commit.
2. **API contract drift**: embedded OpenAPI still carries old Hermes/v1.3.0 metadata and misses/incorrectly lists several live routes.
3. **Security/test/performance debt**: action digest pinning, `HasScope` legacy semantics decision, pgxmock/store tests, Curator dedup complexity.
4. **Backlog hygiene**: several old backlog items are now completed or contradicted by newer session records and should be closed or rewritten.

---

## P1 - Release Readiness

### 1. v2.2.0 full deployment smoke is not yet recorded

**Owner**: qa-engineer + devops-engineer  
**Evidence**:

- `docs/memory/project-context.md` lists v2.2.0 as active.
- The next step is still "Go test/vet + WebUI typecheck/build + Docker/K8s smoke".
- Local verification already covered Go and WebUI, but there is no post-`cec6583` Docker/K8s smoke record.

**Reasoning**: the latest change touched DB migrations, bootstrap security path, Nginx configs, WebUI session rendering, and release workflow. Unit/build checks are necessary but do not prove container startup, migrations, bootstrap, and ingress behavior.

**Impact**: a release tag could pass source-level checks but fail in container/K8s due to migration order, Nginx limit config, static asset packaging, or env wiring.

**Recommended action**:

- Run Docker build.
- Run compose or Kind smoke with PostgreSQL and, if supported, MySQL.
- Validate:
  - `/health/live`
  - `/health/ready`
  - `POST /admin/v1/bootstrap` success once, then idempotent behavior
  - repeated bootstrap attempts return rate limited
  - WebUI chat creates sessions with titles

---

## P1 - API Contract Drift

### 2. Embedded OpenAPI spec is stale and partially inaccurate

**Owner**: backend-engineer + writer  
**Evidence**:

- `internal/api/openapi.go` still reports title `"Hermes Agent API"`, version `"1.3.0"`, and contact `"Hermes Team"`.
- The same file lists `/v1/health/live`, `/v1/health/ready`, and `/v1/metrics`.
- Runtime routes in `internal/api/server.go` expose `/health/live`, `/health/ready`, and `/metrics` instead.
- Runtime routes include `POST /v1/agent/chat`, `POST /v1/gdpr/cleanup-minio`, and `/admin/v1/*` admin routes that the embedded OpenAPI spec does not fully document.

**Reasoning**: `docs/api-reference.md` has been partially corrected, but `/v1/openapi` is the machine-readable contract. Client generators, API tests, and external integrators will consume the embedded spec, not the prose docs.

**Impact**: SDK generation and contract tests may target wrong paths or outdated branding/versioning.

**Recommended action**:

- Update OpenAPI info metadata to HermesX/v2.2.0.
- Replace incorrect health/metrics paths.
- Add missing paths:
  - `POST /v1/agent/chat`
  - `POST /v1/gdpr/cleanup-minio`
  - `GET/POST /admin/v1/bootstrap`
  - `/admin/v1/tenants/{id}/api-keys`
  - `/admin/v1/pricing-rules`
  - `/admin/v1/audit-logs`
- Add regression tests that compare registered server routes against OpenAPI paths.

---

## P2 - Security And Policy Decisions

### 3. GitHub Actions are not digest-pinned

**Owner**: devops-engineer  
**Evidence**:

- `docs/memory/project-context.md` tracks "GHA actions not digest-pinned" as deferred security work.
- `.github/workflows/release.yml` still uses version tags: `actions/checkout@v4`, `actions/setup-go@v5`, `softprops/action-gh-release@v2`.

**Reasoning**: tag-based actions are mutable references. This is acceptable for velocity, but weaker for release supply-chain guarantees.

**Impact**: compromised or retagged actions could affect release artifacts.

**Recommended action**:

- Pin third-party actions to full commit SHA.
- Keep comments showing the human-readable upstream version.
- Add periodic review automation to refresh pins.

### 4. `HasScope` empty-scope behavior conflicts with backlog policy

**Owner**: tech-lead + backend-engineer + security-reviewer  
**Evidence**:

- `docs/memory/backlog.md` still lists "HasScope empty scopes 放行修复" as P3 and says empty scopes should strictly reject no-scope requests.
- `internal/auth/context.go` currently allows legacy empty-scope keys for all non-admin scopes and rejects only `admin`.

**Reasoning**: current code is a compatibility policy, not a pure strict-scope policy. The latest PG scopes fix makes newly created admin keys work, but does not settle whether legacy keys should retain read/write behavior.

**Impact**: if strict least-privilege is required, current behavior is too permissive. If compatibility is required, backlog wording is wrong and should be rewritten as an accepted risk.

**Recommended action**:

- Make an explicit decision:
  - **Option A**: keep compatibility, document legacy non-admin scope behavior and close backlog item as accepted.
  - **Option B**: strict mode, add a config flag such as `HERMES_STRICT_SCOPES=true`, then migrate existing keys.

---

## P2 - Test Coverage

### 5. PostgreSQL store unit tests still need pgxmock or equivalent

**Owner**: backend-engineer  
**Evidence**:

- `docs/memory/project-context.md` lists "store/pg unit tests - pgxmock introduction" as a v2.2.0 candidate.
- `docs/memory/backlog.md` tracks pgxmock introduction as P3 testing debt.

**Reasoning**: this sprint changed PostgreSQL API key scope persistence and bootstrap state migration. Current Go tests pass, but targeted mock tests would catch SQL shape, scan, and error mapping regressions faster than only broader integration paths.

**Impact**: store-layer regressions can slip until integration or deployment smoke.

**Recommended action**:

- Add pgxmock tests for:
  - API key create/get/list with `scopes`
  - bootstrap state atomic claim
  - rotation transaction tenant context

### 6. MySQL adapter still needs CI-level integration guardrails

**Owner**: backend-engineer + qa-engineer  
**Evidence**:

- Session 004 records successful local Kind MySQL E2E.
- Older v2.1 infra docs still list missing MySQL integration tests and migration versioning risk.
- `docs/memory/project-context.md` still contains a stale line saying MySQL full implementation is large work.

**Reasoning**: local E2E reduced product risk, but CI still needs a durable MySQL service matrix or Testcontainers coverage, especially after adding bootstrap state and API key scopes.

**Impact**: MySQL-specific SQL or migration drift could reappear without being caught on every PR.

**Recommended action**:

- Add CI MySQL service or Testcontainers suite for core store flows.
- Reconcile stale project-context wording with Session 004.
- Decide whether MySQL migration versioning is required before production MySQL support.

---

## P3 - Performance And Reliability Debt

### 7. Curator dedup is still quadratic

**Owner**: backend-engineer  
**Evidence**:

- `docs/memory/backlog.md` tracks "Curator O(n²) dedup 优化".
- `internal/agent/curator.go` compares each entry against all unique entries during dedup.

**Reasoning**: the default `MaxMemories` is 100, so this is not urgent at current scale. It becomes visible if memory limits or user memory volume grows.

**Impact**: high-memory tenants may see slow curator runs.

**Recommended action**:

- Keep exact-key dedup in a map.
- Use normalized content hash or token-set signature buckets before similarity comparison.
- Add benchmark coverage at 100, 1,000, and 10,000 memories.

### 8. RustFS SDK compatibility remains unrecorded

**Owner**: devops-engineer + qa-engineer  
**Evidence**:

- `docs/memory/project-context.md` still marks RustFS SDK compatibility as unverified.
- Session 004 validated MinIO in the K8s path, not a RustFS endpoint.

**Reasoning**: the code uses an S3-compatible ObjectStore abstraction, but S3-compatible systems often differ on multipart upload, presigned URLs, metadata, and error behavior.

**Impact**: production deployments that switch from MinIO to RustFS may fail in object operations even if local MinIO tests pass.

**Recommended action**:

- Run object store compatibility smoke against RustFS:
  - bucket create
  - put/get/delete object
  - list prefix
  - presigned URL
  - multipart path if used

---

## P3 - Product / UX Candidates

### 9. Admin tenant-level usage aggregation

**Owner**: product-manager + frontend-engineer + backend-engineer  
**Evidence**:

- `docs/memory/project-context.md` lists "Admin Dashboard tenant-level usage aggregation" as a v2.2.0 UX candidate.

**Reasoning**: user-side usage exists, but admin-level tenant aggregation is a different workflow: comparing tenants, cost, rate-limit pressure, and model consumption.

**Impact**: SaaS operators still lack a compact operational dashboard.

**Recommended action**:

- Confirm product scope first:
  - tenant summary cards
  - model cost by tenant
  - time range filters
  - export CSV

### 10. GDPR self-service export UI

**Owner**: product-manager + frontend-engineer  
**Evidence**:

- `docs/memory/backlog.md` lists GDPR self-service export UI as a P4 candidate.
- Backend GDPR export/delete endpoints exist.

**Reasoning**: backend capability exists, but end users/admins cannot self-serve without API-level operation.

**Impact**: compliance workflows remain operator-assisted.

**Recommended action**:

- Leave as P4 until compliance need is confirmed.
- If confirmed, add a guarded admin/user UI with clear confirmation states.

---

## P3 - Feature Stub

### 11. Skill hub sync is a stub

**Owner**: backend-engineer / product-manager  
**Evidence**:

- `internal/tools/skills_manifest.go` has `SyncFromHub(hubURL string)` implemented as a log-only stub with TODO.

**Reasoning**: this is only a problem if remote skill hub sync is in active product scope. It is currently not blocking core SaaS/WebUI flows.

**Impact**: any UI/CLI path that claims hub sync support would silently do nothing.

**Recommended action**:

- Either remove/hide the feature from exposed surfaces, or implement:
  - signed index fetch
  - checksum validation
  - manifest-aware sync
  - user-modified skill preservation

---

## Backlog Hygiene Needed

Several old backlog entries should be closed or rewritten because newer records show they are completed:

| Item | Current Evidence | Action |
|------|------------------|--------|
| LifecycleHooks to Gateway Runner | Project context says v2.0.0 hardening wired it | Mark closed |
| SelfImprover to Agent loop | Project context says v2.0.0 hardening wired it | Mark closed |
| prompt sanitization consistency | Project context says sanitizeForPrompt extracted and applied | Mark closed |
| payload.URL traversal fix | Project context says v2.0.0 hardening fixed it | Mark closed |
| Session raw ID titles | v2.2.0 stabilization added session titles | Mark closed |
| Bootstrap IP limit / TOCTOU | v2.2.0 stabilization closed both | Already marked closed |
| API Reference endpoint alignment | Prose docs improved, embedded OpenAPI still stale | Keep open, narrow scope to `/v1/openapi` |
| Admin UI pricing rules | WebUI already has PricingPage in v2.1.0-webui records | Verify and close or restate as advanced pricing UX |
| Usage dashboard | User UsagePage exists; admin tenant aggregation remains | Restate as admin tenant-level aggregation |

---

## Recommended Next Batch

1. **API contract cleanup**: fix embedded OpenAPI metadata/routes, then add route/spec regression tests.
2. **Release smoke**: run Docker/K8s validation against the latest commit.
3. **Security sweep**: digest-pin workflows and decide strict-vs-legacy `HasScope` policy.
4. **Store tests**: add pgxmock and MySQL CI coverage for API key scopes and bootstrap state.
5. **Backlog cleanup**: update `docs/memory/backlog.md` so old completed items do not keep steering future work.

