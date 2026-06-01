# Delivery Plan: Hermes Agent v0.15.2 Capability Absorption

> Owner: tech-lead  
> Date: 2026-06-01  
> Status: phase-1-complete

## Phase 1: Foundation Guardrails

| Task | Owner | Output | Status |
| --- | --- | --- | --- |
| Capture ADR for plugin manifest governance | architect | `docs/adr/ADR-007-plugin-catalog-and-manifest-governance.md` | done |
| Add bundled plugin manifest validator | backend-engineer | `internal/plugins/manifest_validation.go` | done |
| Add focused unit tests | qa-engineer | `internal/plugins/manifest_validation_test.go` | done |

## Phase 2: Extension Catalog

| Task | Owner | Output | Status |
| --- | --- | --- | --- |
| Design MCP catalog schema | architect/backend-engineer | `docs/artifacts/2026-06-01-hermes-agent-v0152-absorption/api-contract.md` | done |
| Add Admin catalog endpoints | backend-engineer | `/admin/v1/mcp-catalog/*` | done |
| Add tenant enablement policy | backend-engineer | `internal/mcpcatalog` in-memory store + tests | done |
| Add WebUI catalog management | frontend-engineer | Admin page | planned |

## Phase 3: Skill Bundles

| Task | Owner | Output | Status |
| --- | --- | --- | --- |
| Define bundle manifest schema | architect | ADR/API contract | **done** |
| Add bundle loader to skills layer | backend-engineer | `internal/skills/bundle.go` + 9 tests | **done** |
| Add tenant-safe bundle sync | backend-engineer | `Provisioner.SyncBundle` + 7 tests | **done** |
| Add QA scenarios for shadowing and rollback | qa-engineer | test matrix | **done** |

## Phase 4: Safety And Secrets

| Task | Owner | Output | Status |
| --- | --- | --- | --- |
| Consolidate threat patterns | backend-engineer/security | `internal/safety/threatpatterns` | **done** |
| Wire scanner to tool output, memory recall, skill install, workflow agent tasks | backend-engineer | interceptors + tests | **done** |
| Define `SecretSource` abstraction | architect/backend-engineer | ADR + interface | **done** |
| Add Bitwarden provider as first external source | backend-engineer | provider + tests | **done** |

## Risk Controls

- Keep Phase 1 behavior read-only and test-only.
- Do not add official plugin loading semantics before tenant policy exists.
- Treat user-installed plugins and official bundled plugins as different trust tiers.
- Do not migrate upstream Python code directly into Go packages.
