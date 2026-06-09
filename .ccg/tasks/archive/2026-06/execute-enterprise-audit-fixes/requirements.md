# Requirements

## Input Plan

- `.ccg/tasks/archive/2026-06/plan-enterprise-audit-fixes/plan.md`
- `docs/artifacts/2026-06-09-enterprise-agent-platform-audit/harness-audit-report.md`

## Objective

Batch execute the locally actionable enterprise hardening items:

- Capture release/data-isolation/OIDC-local evidence into a release artifact.
- Add Admin Governance UI for existing evolution governance backend APIs.
- Replace GDPR AlertEvents export's unlimited single query with a bounded paginated collection path.
- Clarify readiness and limiter policy documentation based on current evidence.

## Constraints

- Do not touch unrelated local user changes, including the current `AGENTS.md` modification.
- External Keycloak/Auth0, K8s, and DR evidence may be recorded as unverified if required infrastructure is unavailable.
- Do not use sub-agents unless explicitly requested by the user, because the available sub-agent tool requires explicit delegation permission.
- External CCG wrapper is unavailable at `~/.claude/bin/codeagent-wrapper`; record that review gap.
