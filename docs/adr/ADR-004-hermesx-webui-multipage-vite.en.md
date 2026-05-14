# ADR-004: hermesx-webui Vite Multi-Page Architecture

## Decision Info

| Field | Value |
|-------|-------|
| Number | ADR-004 |
| Title | Vite Multi-Page: User Portal (index.html) + Admin Console (admin.html) |
| Status | Accepted |
| Date | 2026-05-08 |
| Owner | architect |
| Related Requirement | docs/artifacts/2026-05-08-hermesx-webui/prd.md |

## Background and Constraints

- The PRD requires Admin Console (`/admin/`) and User Agent Portal (`/`) to coexist in the same repository.
- The challenge session architect recommended Vite multi-page over single SPA + route guards for the following reasons:
  - The two portals have different authentication mechanisms (user key vs admin key)
  - Independent bundles prevent admin dependencies from contaminating the user bundle
  - Aligns with the existing backend `/static/` serving pattern (`index.html` + `admin.html`)
- Non-goal: Not splitting into two separate npm projects (maintenance cost too high).

## Alternatives

### A: Single SPA + Route Guards (Rejected)

All pages in the same Vue application, distinguishing permissions via `/admin/*` routes and route guards.

**Rejected because:**
- Admin and User bundles cannot be separated; admin dependencies (e.g., large Audit Log tables) contaminate the user bundle
- Route guards only guard at the frontend, no bundle-level isolation
- Single SPA requires runtime detection of "currently admin or user", high coupling

### B: Vite Multi-Page (**Adopted**)

Two independent HTML entry points:

```
webui/
├── index.html          ← User Portal entry (built as /index.html)
├── admin.html          ← Admin Console entry (built as /admin.html or /admin/index.html)
├── src/
│   ├── user/
│   │   ├── main.ts     ← User Portal app instance
│   │   ├── App.vue
│   │   └── router.ts
│   ├── admin/
│   │   ├── main.ts     ← Admin Console app instance
│   │   ├── App.vue
│   │   └── router.ts
│   ├── shared/         ← Code shared between both entries
│   │   ├── api/        ← useApi.ts, useSse.ts
│   │   ├── stores/     ← auth.ts (Pinia, instantiated separately)
│   │   ├── types/      ← TypeScript types
│   │   └── components/ ← Shared UI components
│   └── pages/          ← Page components (split into user/ and admin/ subdirectories)
└── vite.config.ts      ← build.rollupOptions.input configured for two entries
```

Key vite.config.ts configuration:
```typescript
build: {
  rollupOptions: {
    input: {
      main: resolve(__dirname, 'index.html'),
      admin: resolve(__dirname, 'admin.html'),
    },
  },
}
```

**Advantages:**
- Independent bundles — User Portal loads no Admin code
- Natural permission isolation: admin.html loads the admin app instance, never mixed with user
- Clear nginx routing: `/admin` → `admin.html`, `/` → `index.html`
- Fully compatible with the existing backend `/static/` serving pattern

## Decision Outcome

**Adopting Option B: Vite Multi-Page.**

Nginx routing configuration:
```nginx
location /admin {
    try_files $uri $uri/ /admin.html;
}
location / {
    try_files $uri $uri/ /index.html;
}
```

Directory migration plan (Phase 0):
1. Reorganize existing `src/` content into `src/shared/` + `src/user/` + `src/admin/`
2. Create `admin.html` entry point
3. Update `vite.config.ts` to multi-page mode

## Follow-up Actions

| Action | Owner | Completion Criteria |
|--------|-------|---------------------|
| Reorganize directory structure, configure vite.config.ts | frontend-engineer | Phase 0 |
| Update nginx.conf (SSE configuration see ADR-006) | frontend-engineer | Phase 0 |
| Verify `npm run build` produces two HTML files | qa-engineer | Phase 0 acceptance |
