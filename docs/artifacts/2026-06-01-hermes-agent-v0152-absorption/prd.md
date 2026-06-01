# PRD: Hermes Agent v0.15.2 Capability Absorption

> Owner: tech-lead  
> Date: 2026-06-01  
> Status: executing  
> Source: NousResearch/hermes-agent `v2026.5.29.2`

## Goal

Absorb the parts of Hermes Agent `v0.15.2` that strengthen HermesX as an enterprise runtime control plane, without copying Python runtime code into the Go platform.

## Problem

The upstream release fixed a small but important production failure: bundled plugin manifests were not shipped in distributed artifacts, causing plugins to disappear after install. HermesX already has plugin discovery, skills, MCP, safety gates, workflow, scheduler, and multi-tenant controls, but it lacks a formal release invariant for bundled plugin metadata and a roadmap for catalog-governed extensions.

## Scope

### In Scope

- Plugin manifest governance and validation.
- MCP catalog planning.
- Skill bundle planning.
- Threat-pattern consolidation planning.
- Secret source abstraction planning.

### Out Of Scope

- Direct Python code migration.
- Replacing the Go runtime with upstream Hermes Agent internals.
- TUI/Ink porting.
- Consumer plugins such as Spotify or Google Meet as core features.

## Success Criteria

- A persisted ADR records the plugin manifest governance decision.
- A delivery plan ranks absorption work by risk and value.
- The first guardrail for bundled plugin manifests exists in Go with tests.
- Follow-up work is explicit enough for backend, frontend, QA, and DevOps handoff.

## Key Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| R1 | Detect bundled plugin implementation directories missing `plugin.yaml` / `plugin.yml`. | P0 |
| R2 | Validate manifest parseability and required `name`. | P0 |
| R3 | Keep current user/project plugin discovery behavior backward compatible. | P0 |
| R4 | Define next-step architecture for MCP catalog and skill bundles. | P1 |
| R5 | Identify threat-pattern and external secret-source work as separate implementation tracks. | P1 |

## Evidence

- Upstream `v2026.5.29.2` release changed only packaging/version files and added a regression test for plugin manifests.
- HermesX `internal/plugins` discovers plugins by reading `plugin.yaml`.
- HermesX has no official bundled plugin root yet, so the safe first step is reusable validation rather than runtime behavior changes.
