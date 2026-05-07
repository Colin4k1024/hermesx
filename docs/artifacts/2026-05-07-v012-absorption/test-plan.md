# Test Plan — v0.12 Upstream Absorption (Sprint 3)

## Meta

| Field | Value |
|-------|-------|
| Date | 2026-05-07 |
| Slug | v012-absorption |
| Role | qa-engineer |
| Status | final |
| State | review |

## Scope

### In Scope

| Module | Feature | Test Coverage |
|--------|---------|---------------|
| `internal/agent/curator.go` | Autonomous memory curation with heuristic + LLM dedup | 25 unit tests |
| `internal/agent/self_improve.go` | Self-improvement loop with periodic LLM review | 17 unit tests (14 original + 3 new) |
| `internal/gateway/media_dispatch.go` | Capability-aware media routing with fallback | 14 unit tests (12 original + 2 new) |
| `internal/gateway/lifecycle_hooks.go` | Priority-ordered lifecycle event hooks | 11 unit tests |

### Out of Scope

- Integration tests against real LLM providers
- E2E tests through platform adapters
- Performance/load testing under high concurrency
- Gateway Runner wiring of LifecycleHooks (deferred to integration phase)

## Test Matrix

| Category | Scenario | Type | Result |
|----------|----------|------|--------|
| Curator | Disabled curator returns empty | Unit | PASS |
| Curator | Empty store returns no-op | Unit | PASS |
| Curator | Exact key dedup detection | Unit | PASS |
| Curator | Content similarity above threshold | Unit | PASS |
| Curator | Stale entry pruning when over limit | Unit | PASS |
| Curator | LLM merge group parsing | Unit | PASS |
| Curator | LLM fallback on error | Unit | PASS |
| SelfImprover | Disabled never triggers | Unit | PASS |
| SelfImprover | Nil completer never triggers | Unit | PASS |
| SelfImprover | Correct trigger intervals | Unit | PASS |
| SelfImprover | ReviewInterval=0 no panic | Unit | PASS |
| SelfImprover | MaxInsights eviction enforced | Unit | PASS |
| SelfImprover | Prompt sanitization strips control chars | Unit | PASS |
| SelfImprover | Insights parsed from LLM response | Unit | PASS |
| SelfImprover | NONE response yields empty | Unit | PASS |
| MediaDispatcher | Image direct send | Unit | PASS |
| MediaDispatcher | Image → document fallback | Unit | PASS |
| MediaDispatcher | Image → link fallback | Unit | PASS |
| MediaDispatcher | Video direct + fallback | Unit | PASS |
| MediaDispatcher | Voice direct + fallback | Unit | PASS |
| MediaDispatcher | Missing chat_id rejected | Unit | PASS |
| MediaDispatcher | Path traversal rejected | Unit | PASS |
| MediaDispatcher | DetectMediaType (10 extensions) | Unit | PASS |
| LifecycleHooks | Register and fire | Unit | PASS |
| LifecycleHooks | Priority ordering | Unit | PASS |
| LifecycleHooks | All 6 emit methods | Unit | PASS |
| LifecycleHooks | Error propagation + continuation | Unit | PASS |
| LifecycleHooks | No hooks returns nil | Unit | PASS |

## Regression

- Full suite: **1576 tests, 33 packages, 0 failures**
- Race detector: Clean on lifecycle hooks package
- Build: `go build ./...` success

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| LifecycleHooks not wired into Runner | Medium | Deferred — hooks work standalone, integration in next sprint |
| normalizedSimilarity O(n²) for large stores | Low | Bounded by MaxMemories=100 default |
| SelfImprover insights use UnixNano key | Low | Collision extremely unlikely at expected call frequency |
| Curator LLM prompt not sanitized | Low | Only processes store.List() output (server-controlled data) |

## Verdict

**GO** — All CRITICAL and HIGH findings remediated, test coverage adequate, no blocking issues remaining.
