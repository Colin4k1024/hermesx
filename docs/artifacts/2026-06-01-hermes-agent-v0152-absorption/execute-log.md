# Execute Log: Hermes Agent v0.15.2 Capability Absorption

> Owner: backend-engineer  
> Date: 2026-06-01  
> Status: phase-4-complete

## Completed

- Created ADR-007 for plugin catalog and manifest governance.
- Created PRD, delivery plan, architecture design, and test plan artifacts.
- Added bundled plugin manifest validator in `internal/plugins`.
- Added unit tests for valid nested plugin trees, missing manifests, invalid manifests, and missing roots.
- Ran focused plugin package tests successfully.
- Added `internal/mcpcatalog` with governed catalog item and tenant policy models.
- Added in-memory MCP catalog store with validation and unit tests.
- Added Admin endpoints for catalog item list/get/upsert and tenant item enablement.
- Wired `APIServerConfig.MCPCatalogStore`; nil config now creates an empty in-memory catalog.
- Added MCP catalog API contract artifact.
- **Phase 3 — Skill Bundles**:
  - Created `internal/skills/bundle.go` — `BundleManifest` model, `LoadBundleManifest`, `ValidateBundleManifest` (9 tests).
  - Added `SyncBundle` to `internal/skills/provisioner.go` — uploads bundle skills to tenant OSS namespace, respects `UserModified` shadowing semantics controlled by `BundlePolicy.AllowTenantOverride` (7 SyncBundle tests).
  - All 19 new bundle + sync tests pass alongside existing skills tests.

## Pending

- Add release packaging checks once HermesX has an official bundled plugin root.
- Add PostgreSQL/MySQL persistence for MCP catalog.
- Add Admin WebUI catalog management.
- **Phase 4 — Safety And Secrets** (next phase):
  - Consolidate threat patterns → `internal/safety/threatpatterns` subpackage.
  - Wire scanner to tool output, memory recall, skill install, workflow agent tasks.
  - Define `SecretSource` abstraction with Bitwarden provider.
- **Phase 4 — Safety And Secrets** ✅ **COMPLETE**:
  - Created `internal/safety/threatpatterns/bundles.go` — 7 named bundles (`PromptInjection`, `RoleHijack`, `PromptExtraction`, `DelimiterInjection`, `EncodingAttack`, `SafetyBypass`, `IndirectInjection`), plus `All()` aggregate.
  - Added `(*PatternRegistry).LoadBundle` in `internal/safety/patterns.go` — compiles regexp, skips invalid patterns with `slog.Warn`.
  - Added scan-point extension methods to `InterceptorChain`: `ScanToolOutput`, `ScanSkillContent`, `ScanMemoryContent` — respect Enforce/LogOnly/Disabled policy modes; integrate `OutputGuard` + `CanaryDetector`.
  - Created `internal/secrets/source.go` — `SecretSource` interface, `ErrNotFound` sentinel, `Chain` with first-wins `Get` and deduplicated `List`.
  - Created `internal/secrets/env/provider.go` — env-variable-backed provider with configurable prefix; treats unset or empty vars as not found.
  - Created `internal/secrets/bitwarden/client.go` — `BitwardenClient` interface + default `httpClient` (Bearer auth, configurable base URL, 10 s timeout).
  - Created `internal/secrets/bitwarden/provider.go` — `Provider` backed by `BitwardenClient`; `New` for production, `NewWithClient` for test injection.
  - All tests pass: `internal/safety`, `internal/safety/threatpatterns`, `internal/secrets`, `internal/secrets/env`, `internal/secrets/bitwarden`.

## Verification

```bash
/usr/local/go/bin/go test ./internal/plugins -count=1
```

Result: PASS.

```bash
/usr/local/go/bin/go test ./internal/mcpcatalog ./internal/api/admin ./internal/api -count=1
```

Result: PASS.

```bash
/usr/local/go/bin/go test ./internal/skills -count=1
```

Result: PASS (19 new bundle + SyncBundle tests, all green).

```bash
/usr/local/go/bin/go test ./internal/safety/... ./internal/secrets/... -count=1
```

Result: PASS — `internal/safety` ✅ `internal/safety/threatpatterns` ✅ `internal/secrets` ✅ `internal/secrets/env` ✅ `internal/secrets/bitwarden` ✅
