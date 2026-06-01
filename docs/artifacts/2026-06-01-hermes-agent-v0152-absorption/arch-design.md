# Arch Design: Hermes Agent v0.15.2 Capability Absorption

> Owner: architect / backend-engineer  
> Date: 2026-06-01  
> Status: phase-1-implemented

## Architecture Position

Hermes Agent and HermesX now solve different problems:

- Hermes Agent: Python personal/remote agent with broad local integrations.
- HermesX: Go runtime control plane for governed, multi-tenant agent execution.

The integration strategy is therefore capability absorption, not source-code merge.

## Phase 1 Design

`internal/plugins` gains a reusable validation function:

```text
ValidateBundledPluginManifests(root)
  -> []PluginManifestIssue
```

The validator:

- accepts a bundled plugin root;
- allows category directories when nested plugin manifests exist;
- reports implementation directories with no `plugin.yaml` / `plugin.yml`;
- parses manifests and requires `name`;
- treats a missing root as no-op so current deployments remain compatible.

## MCP Catalog Boundary

MCP catalog should be a control-plane object, not an agent prompt convention.

Minimum fields:

- catalog item ID, name, version, source URL;
- trust tier and review status;
- required credentials and scopes;
- tenant enablement policy;
- sandbox/egress requirements;
- installation and rollback state.

Phase 2 implementation adds:

- `internal/mcpcatalog`: catalog item model, tenant item policy, validation, and in-memory store.
- `internal/api/admin/mcp_catalog.go`: Admin endpoints for item management and tenant enablement.
- `internal/api/server.go`: default empty in-memory catalog wiring when no persistent catalog store is provided.

The current store is intentionally in-memory. The interface boundary is already explicit so PostgreSQL/MySQL persistence can replace it without changing handlers.

## Future Skill Bundle Boundary

Skill bundles should live above individual skills:

```text
bundle -> ordered skill refs -> tenant policy -> session load request
```

Bundles should not duplicate skill content. They should reference canonical skills and record compatibility constraints.

## Non-Functional Requirements

| Area | Requirement |
| --- | --- |
| Backward compatibility | Existing user/project plugins continue loading as before. |
| Multi-tenancy | Official catalog enablement must be tenant-scoped. |
| Auditability | Catalog install, enable, disable, and rollback actions require audit events. |
| Security | External plugin or MCP installation must pass source and manifest checks before enablement. |
| Release reliability | Bundled plugin manifests must be included in every distribution artifact that contains plugin code. |
