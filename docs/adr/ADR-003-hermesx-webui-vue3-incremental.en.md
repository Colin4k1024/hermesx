# ADR-003: hermesx-webui Technology Stack — Vue 3 Incremental Evolution vs React 18 Rewrite

## Decision Info

| Field | Value |
|-------|-------|
| Number | ADR-003 |
| Title | hermesx-webui Frontend Stack: Vue 3 Incremental Evolution |
| Status | Accepted |
| Date | 2026-05-08 |
| Owner | tech-lead |
| Related Requirement | docs/artifacts/2026-05-08-hermesx-webui/prd.md |

## Background and Constraints

- The original PRD technology scope specified React 18 + Zustand + TanStack Query + Tailwind.
- During the `/team-plan` challenge session, it was discovered that the `webui/` directory already contained a Vue 3 + Pinia + Naive UI + Vue Router v4 project (~484 lines) with:
  - A working auth store (using sessionStorage, fetching tenant_id from `/v1/me` response, not injecting into headers erroneously)
  - useApi composable (Bearer token + X-Hermes-User-Id, auto-redirect on 401/403)
  - AppShell layout, ChatPage, MemoriesPage, SkillsPage, AdminTenantsPage scaffolding
  - Vue Router v4 route guard structure
- The PRD was written without knowledge of this existing directory; the React 18 selection was based on a "start from scratch" assumption that no longer holds.
- Non-goal: This ADR does not affect the backend Go implementation.

## Alternatives

### A: Vue 3 Incremental Evolution (**Adopted**)

Reuse the existing `webui/` foundation with the following adjustments:

| Change | Details |
|--------|---------|
| Retain | Vue 3 + Pinia + Vue Router v4 + Naive UI + Vite + TypeScript |
| Add | `@tanstack/vue-query` v5 (replaces raw fetch state management in stores) |
| Add | Tailwind CSS v4 (supplements Naive UI for custom style areas) |
| Rename | `acpToken` → `adminApiKey` in auth store (ACP is an editor protocol, not an admin key) |
| Structure | Vite multi-page mode (see ADR-004) |

**Advantages:**
- Saves 2–3 days of infrastructure rebuilding
- Existing auth/API layer already addresses security concerns raised in the challenge session (sessionStorage ✓, tenant_id from response ✓)
- Naive UI DataTable/Form/Modal provides better coverage for enterprise-grade Admin Console than bare Tailwind components
- `@tanstack/vue-query` v5 API is mostly compatible with React Query, low learning curve
- Vue 3 Composition API aligns conceptually with React hooks, low switching cost

**Risks:**
- Naive UI occasionally has incomplete TypeScript types (can be handled locally with `// @ts-expect-error`)
- `@tanstack/vue-query` community is slightly smaller than React Query (documentation is complete, acceptable)

### B: React 18 Rewrite

**Rejected because:**
- 484 lines of scaffolding code must be entirely discarded, all pages rebuilt from scratch
- The PRD specification was based on a false premise ("starting from scratch") that is invalidated by the discovered existing code
- Tailwind requires building DataTable, Modal, Form, and other components from scratch, adding 1–2 weeks of work
- Both approaches deliver identical end-user product experience at feature parity

## Decision Outcome

**Adopting Option A: Vue 3 Incremental Evolution.**

Final technology stack:
```
Vue 3 + Pinia + Vue Router v4 + Naive UI
+ @tanstack/vue-query v5
+ Tailwind CSS v4
+ Vite 6 (multi-page mode)
+ TypeScript strict
```

Impact scope:
- `webui/package.json`: add `@tanstack/vue-query`, `tailwindcss`
- `webui/src/stores/auth.ts`: rename `acpToken` field to `adminApiKey`, update related references
- `webui/src/composables/useApi.ts`: admin requests switch from `acpToken` to `adminApiKey`
- Compatibility: existing useApi / auth patterns are not broken, only extended

## Enterprise Governance Notes

- Application tier: internal tooling (T4), no mandatory enterprise framework constraints
- No enterprise frontend framework whitepaper restrictions

## Follow-up Actions

| Action | Owner | Completion Criteria |
|--------|-------|---------------------|
| Update auth.ts: acpToken → adminApiKey | frontend-engineer | Phase 0 complete |
| Install @tanstack/vue-query + tailwindcss | frontend-engineer | Phase 0 complete |
| Configure Vite multi-page (see ADR-004) | frontend-engineer | Phase 0 complete |
