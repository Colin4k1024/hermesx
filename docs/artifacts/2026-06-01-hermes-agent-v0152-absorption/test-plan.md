# Test Plan: Hermes Agent v0.15.2 Capability Absorption

> Owner: qa-engineer  
> Date: 2026-06-01  
> Status: phase-1-pass

## Phase 1 Test Matrix

| ID | Scenario | Expected Result |
| --- | --- | --- |
| T01 | Nested plugin category with valid child manifest | no validation issue |
| T02 | Plugin implementation directory without manifest | one missing-manifest issue |
| T03 | Manifest exists but lacks `name` | one invalid-manifest issue |
| T04 | Bundled plugin root does not exist | no-op, no issue |

## Command

```bash
go test ./internal/plugins -count=1
```

## Result

```text
ok  	github.com/Colin4k1024/hermesx/internal/plugins	0.730s
```

## Future Test Additions

- Release archive/image check proves bundled manifests are present after packaging.
- Skill bundle loading preserves tenant isolation and user-modified skill shadowing semantics.
- Threat-pattern scanner blocks or annotates tool output, memory recall, skill install, and workflow agent task injection attempts.

## Phase 2 Test Matrix

| ID | Scenario | Expected Result |
| --- | --- | --- |
| T05 | MCP catalog item upsert, list, and tenant enablement through Admin routes | 200 responses and audit records |
| T06 | Invalid stdio catalog item without command | 400 response |
| T07 | Catalog route when store is not configured directly on AdminHandler | 503 response |
| T08 | API server default catalog wiring | `internal/api` package tests pass |

## Phase 2 Result

```text
ok  	github.com/Colin4k1024/hermesx/internal/plugins	0.650s
ok  	github.com/Colin4k1024/hermesx/internal/mcpcatalog	0.967s
ok  	github.com/Colin4k1024/hermesx/internal/api/admin	1.650s
ok  	github.com/Colin4k1024/hermesx/internal/api	1.599s
```
