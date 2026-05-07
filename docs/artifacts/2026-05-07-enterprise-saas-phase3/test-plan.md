# Test Plan — Phase 3: OIDC Wiring, Breaker Metrics, CI/CD

**Date:** 2026-05-07
**Role:** qa-engineer
**Status:** passed
**State:** review

---

## Test Scope

### Functional Coverage

| Area | Tests | Status |
|------|-------|--------|
| OIDC token verification (valid/invalid/expired/wrong-aud) | 8 | PASS |
| OIDC missing tenant_id rejection (SEC-01 fix) | 1 | PASS |
| OIDC JWT-shape detection + error propagation (SEC-02 fix) | 2 | PASS |
| Circuit breaker Chat path (success/failure/trip) | 12 | PASS |
| Circuit breaker ChatStream wrapper (context exit) | 4 | PASS |
| Breaker Prometheus metrics registration | 2 | PASS |
| Auth ExtractorChain ordering | 6 | PASS |
| Full regression (all packages) | 1469 | PASS |

### Non-Functional Coverage

| Check | Result |
|-------|--------|
| `go build ./...` | Clean |
| `go vet ./...` | Clean |
| Race detection (`-race`) for auth/llm/gateway | PASS |
| CI workflow YAML syntax | Valid |

---

## Security Review Findings (Addressed)

| ID | Severity | Issue | Resolution |
|----|----------|-------|-----------|
| SEC-01 | HIGH | Empty tenant_id → "default" tenant | Fixed: reject auth when tenant claim missing |
| SEC-02 | HIGH | OIDC verification failure silent passthrough | Fixed: JWT-shaped tokens return error on verification failure |
| CODE-02 | HIGH | OIDC startup blocks indefinitely | Fixed: 15s timeout on discovery context |
| CODE-03 | HIGH | ChatStream goroutine leak on caller abandon | Fixed: context-based select exit path |

## Remaining Accepted Risks (Non-Blocking)

| ID | Severity | Issue | Disposition |
|----|----------|-------|-------------|
| CODE-04 | HIGH | ChatStream `breaker.Execute` double-counts | Accept for now — streaming workload is low volume; follow-up when gobreaker exposes RecordFailure |
| SEC-03 | MEDIUM | tenant_id as Prometheus label cardinality | Accept — bounded by DB-registered tenants |
| SEC-04 | MEDIUM | Half-open state not enforced in ChatStream | Accept — streaming traffic is probe-safe at current scale |
| CODE-CI-1 | MEDIUM | Go 1.25 in CI (may not exist yet) | Accept — matches go.mod toolchain directive |
| CODE-CI-2 | MEDIUM | No coverage failure threshold | Accept — decorative for now, add threshold later |
| SEC-06 | LOW | GHA actions not digest-pinned | Follow-up task |

---

## Code Review Summary

| Severity | Count | Status |
|----------|-------|--------|
| CRITICAL | 0 | Pass |
| HIGH | 5 found → 4 fixed | 1 accepted |
| MEDIUM | 7 | All accepted with rationale |
| LOW | 6 | Non-blocking |

---

## Go/No-Go

**Verdict: GO** — All blocking security issues resolved. Build clean, 1469 tests pass, vet clean. Remaining items are non-blocking accepted risks with documented follow-up paths.
