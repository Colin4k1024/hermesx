# Launch Acceptance — v0.12 Upstream Absorption (Sprint 3)

## Meta

| Field | Value |
|-------|-------|
| Date | 2026-05-07 |
| Slug | v012-absorption |
| Role | qa-engineer |
| Status | final |
| State | accepted |

## Acceptance Overview

| Item | Detail |
|------|--------|
| Object | Sprint 3: Autonomous Memory Curator, Self-improvement Loop, Gateway Media Parity, Gateway Lifecycle Hooks |
| Time | 2026-05-07 |
| Roles | qa-engineer, tech-lead |
| Method | Automated test suite + code review + security review |

## Acceptance Scope

### Business

- Memory quality self-maintenance (curator dedup + pruning)
- Agent conversation quality self-assessment (self-improver)
- Unified media dispatch with platform capability detection
- Lifecycle event hooks for monitoring and extensibility

### Technical

- 4 new source files, 4 new test files
- Concurrency safety (mutexes on shared state)
- Input validation (prompt sanitization, path traversal, chat_id)
- Bounded resource usage (MaxInsights, MaxMemories)

### Non-functional

- Race-clean under `-race` flag
- No external dependency additions
- Backward compatible (no API breaking changes)

### Not in Scope

- Production deployment
- Gateway Runner integration wiring
- Real LLM provider integration tests
- Performance benchmarking

## Acceptance Evidence

| Evidence | Location | Result |
|----------|----------|--------|
| Unit tests | `go test ./... -short` | 1576 pass, 0 fail |
| Race detection (gateway) | `go test -race ./internal/gateway/ -run Lifecycle` | Clean |
| Race detection (agent) | `go test -race ./internal/agent/ -run SelfImprov` | Clean |
| Build | `go build ./...` | Success |
| Code review (round 1) | Prior session | WARNING: 4 HIGH → all fixed |
| Code review (round 2) | This session | WARNING: 2 HIGH (eviction order, Review() race) → fixed |
| Security review (round 1) | Prior session | 1 CRITICAL, 4 HIGH → all fixed |
| Security review (round 2) | This session | All primary vectors CLOSED, 3 non-blocking residuals tracked |

## Risk Judgment

### Satisfied

- [x] All CRITICAL security findings fixed (prompt injection)
- [x] All HIGH findings fixed (div-by-zero, race conditions, path traversal, unbounded growth)
- [x] Test coverage for new code adequate (67 tests across 4 features)
- [x] No regressions in existing 1509 tests
- [x] Concurrency safety verified with race detector

### Accepted Risks

| Risk | Rationale |
|------|-----------|
| LifecycleHooks not yet wired into Runner | Standalone correct; wiring is additive work |
| Curator O(n²) dedup | Bounded by MaxMemories=100; optimize later if needed |
| No integration tests with real LLM | Unit mocks cover all paths; real LLM test deferred to E2E phase |
| compress.go/curator.go not yet sanitized | Operate on server-controlled stored data, not raw user input; tracked for next sprint |
| payload.URL traversal check not yet applied | Adapters currently treat URLs as remote references; tracked for next sprint |
| Unicode bidi chars pass sanitization | LLM not affected by rendering tricks; low practical risk |

### Blocking Items

None.

## Launch Conclusion

| Item | Decision |
|------|----------|
| Allow merge | **YES** |
| Preconditions | CI green on main branch |
| Observation focus | Memory store growth patterns after curator activation |
| Confirmed by | qa-engineer (automated), tech-lead (review) |

## Go / No-Go

**GO** — Sprint 3 deliverables are ready for merge to main.
