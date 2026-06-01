# ADR-007: Plugin Catalog And Manifest Governance

## Status

Accepted

## Date

2026-06-01

## Context

HermesX is a Go + React multi-tenant runtime control plane. NousResearch Hermes Agent `v2026.5.29.2` is a Python/uv release whose only functional fix is packaging: bundled `plugin.yaml` / `plugin.yml` files were missing from wheel and sdist artifacts, which made installed plugins undiscoverable.

HermesX already has plugin discovery under `internal/plugins`, but plugin distribution is not yet governed as a first-class release concern. If HermesX starts shipping bundled official plugins, missing manifests would create the same failure mode: code ships, discovery returns no plugin, and the runtime loses capabilities silently.

## Decision

HermesX will treat plugin manifests as release-critical metadata.

1. Bundled plugins must include `plugin.yaml` or `plugin.yml`.
2. Plugin manifest validation must be available as a package-level guard before bundled plugins become part of release artifacts.
3. Future official plugin catalog work must store manifest metadata, trust level, credential requirements, scopes, and tenant enablement state separately from executable plugin code.
4. User and project plugins remain supported, but official bundled plugins require stricter validation and release evidence.

## Alternatives Considered

### Do nothing until bundled plugins exist

Rejected. The upstream release shows that manifest packaging defects are cheap to introduce and expensive to diagnose after distribution.

### Copy Hermes Agent's Python packaging fix

Rejected. HermesX does not use Python wheel/sdist as its primary release mechanism. The transferable lesson is the release invariant, not the `pyproject.toml` syntax.

### Make plugins optional best-effort metadata

Rejected. Optional metadata is reasonable for ad-hoc local scripts, but not for official plugins in a multi-tenant control plane.

## Consequences

- First implementation adds reusable Go validation for bundled plugin trees.
- Release and CI workflows can later call this validation against official plugin roots.
- MCP catalog, skill bundles, and provider pluginization can build on a consistent manifest contract.
- Existing user/project plugin behavior remains backward compatible.

## Follow-Up Work

1. Add official plugin root once the first HermesX bundled plugin is introduced.
2. Add release packaging checks for Docker/image/archive outputs.
3. Design Admin API and store schema for tenant-scoped plugin catalog enablement.
4. Extend manifest schema with trust, scopes, credential requirements, and sandbox requirements.
