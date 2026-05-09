# UI Implementation Plan — Hermes Web UI

**Version:** 0.1  
**Date:** 2026-04-30  
**Owner:** frontend-engineer  
**Slug:** 2026-04-30-hermes-webui

---

## 1. Directory Structure (`webui/src/`)

```
webui/
├── index.html
├── vite.config.ts
├── tsconfig.json
├── package.json
└── src/
    ├── main.ts                  # app entry: createApp, pinia, router
    ├── App.vue                  # root: <RouterView />
    ├── router/
    │   └── index.ts             # route definitions + auth guard
    ├── stores/
    │   ├── auth.ts              # apiKey, userId, acpToken, isAdmin, connected
    │   ├── chat.ts              # sessions, currentSession, messages, loading
    │   ├── memory.ts            # memories list
    │   └── skill.ts             # skills list, selectedSkill content
    ├── composables/
    │   └── useApi.ts            # central fetch wrapper with auth headers
    ├── pages/
    │   ├── ConnectPage.vue      # /connect
    │   ├── ChatPage.vue         # /chat
    │   ├── MemoriesPage.vue     # /memories
    │   ├── SkillsPage.vue       # /skills
    │   └── admin/
    │       ├── AdminSkillsPage.vue   # /admin/skills
    │       ├── AdminTenantsPage.vue  # /admin/tenants
    │       └── AdminKeysPage.vue     # /admin/keys
    ├── components/
    │   ├── layout/
    │   │   ├── AppLayout.vue         # shell: top nav + <slot />
    │   │   └── AdminLayout.vue       # extends AppLayout, admin nav tab
    │   ├── connect/
    │   │   └── ConnectForm.vue
    │   ├── chat/
    │   │   ├── SessionSidebar.vue    # session list + new-session button
    │   │   ├── MessageList.vue       # scrollable message area
    │   │   ├── MessageItem.vue       # single bubble (role, content, timestamp)
    │   │   └── ChatInput.vue         # textarea + send button + loading state
    │   ├── memory/
    │   │   ├── MemoryTable.vue       # list table with delete action
    │   │   └── MemoryDeleteConfirm.vue
    │   ├── skill/
    │   │   ├── SkillList.vue         # sidebar skill list (read-only)
    │   │   └── SkillContentViewer.vue # right pane, code/markdown render
    │   └── admin/
    │       ├── SkillUploadModal.vue
    │       ├── TenantTable.vue
    │       ├── TenantCreateModal.vue
    │       ├── KeyTable.vue
    │       └── KeyCreateModal.vue
    └── utils/
        └── errors.ts            # normalizeApiError(e): { message, status }
```

---

## 2. Pinia Store Schemas

### `stores/auth.ts`

```ts
interface AuthState {
  apiKey: string           // persisted: sessionStorage['hermes_api_key']
  userId: string           // persisted: sessionStorage['hermes_user_id']
  acpToken: string         // persisted: sessionStorage['hermes_acp_token']
  isAdmin: boolean         // derived: acpToken.length > 0
  connected: boolean       // true after /connect form submit succeeds
}

// Actions
connect(apiKey: string, userId: string, acpToken?: string): Promise<void>
  // Validates by calling GET /v1/me with supplied credentials.
  // On success: writes sessionStorage, sets connected = true.
  // On failure: throws, caller shows error state.

disconnect(): void
  // Clears sessionStorage and resets all state fields.
```

**sessionStorage keys:** `hermes_api_key`, `hermes_user_id`, `hermes_acp_token`  
**Rehydration:** called in `main.ts` before mount — reads sessionStorage, sets store state without network call.

---

### `stores/chat.ts`

```ts
interface Session {
  id: string
  started_at: string
  ended_at: string | null
  message_count: number
}

interface Message {
  role: 'user' | 'assistant' | 'system'
  content: string
  timestamp: string
}

interface ChatState {
  sessions: Session[]
  currentSessionId: string | null
  messages: Message[]
  // 4-state per data-fetching operation
  sessionsLoading: boolean
  sessionsError: string | null
  messagesLoading: boolean
  messagesError: string | null
  sendLoading: boolean
  sendError: string | null
}

// Actions
fetchSessions(): Promise<void>          // GET /v1/sessions
selectSession(id: string): Promise<void>// GET /v1/sessions/{id}/messages
sendMessage(content: string): Promise<void>
  // POST /v1/chat/completions with X-Hermes-Session-Id
  // Optimistically appends user message, then appends assistant response.
clearSession(id: string): Promise<void> // DELETE /v1/mock-sessions/{id}
newSession(): void                      // set currentSessionId = null, messages = []
```

---

### `stores/memory.ts`

```ts
interface MemoryEntry {
  key: string
  content: string
}

interface MemoryState {
  memories: MemoryEntry[]
  loading: boolean
  error: string | null
  deleteLoading: Set<string>   // tracks which keys are mid-delete
}

// Actions
fetchMemories(): Promise<void>           // GET /v1/memories
deleteMemory(key: string): Promise<void> // DELETE /v1/memories/{key}
```

---

### `stores/skill.ts`

```ts
interface SkillItem {
  name: string
  description: string
  version: string
  source: string
  user_modified: boolean
}

interface SkillState {
  skills: SkillItem[]
  selectedSkillName: string | null
  selectedSkillContent: string | null
  // 4-state
  listLoading: boolean
  listError: string | null
  contentLoading: boolean
  contentError: string | null
  uploadLoading: boolean
  uploadError: string | null
}

// Actions
fetchSkills(): Promise<void>                           // GET /v1/skills
fetchSkillContent(name: string): Promise<void>         // GET /v1/skills/{name}
uploadSkill(name: string, file: File): Promise<void>   // PUT /v1/skills/{name}
deleteSkill(name: string): Promise<void>               // DELETE /v1/skills/{name}
```

---

## 3. API Composable — `composables/useApi.ts`

```ts
interface RequestOptions extends RequestInit {
  sessionId?: string   // adds X-Hermes-Session-Id header
  asAdmin?: boolean    // uses acpToken instead of apiKey for Authorization
}

function useApi() {
  function request<T>(path: string, options?: RequestOptions): Promise<T>
  // Resolves headers:
  //   Authorization: Bearer {acpToken}  if asAdmin=true
  //   Authorization: Bearer {apiKey}    otherwise
  //   X-Hermes-User-Id: {userId}        always
  //   X-Hermes-Session-Id: {sessionId}  when provided
  // On 401/403: calls authStore.disconnect(), redirects to /connect
  // On non-2xx: throws { message, status } via normalizeApiError

  // Convenience wrappers
  function get<T>(path: string, options?: RequestOptions): Promise<T>
  function post<T>(path: string, body: unknown, options?: RequestOptions): Promise<T>
  function put<T>(path: string, body: unknown, options?: RequestOptions): Promise<T>
  function del(path: string, options?: RequestOptions): Promise<void>
  // put() with multipart: accepts FormData directly, omits Content-Type to let browser set boundary
}
```

**Base URL resolution:**
- Dev: Vite proxy rewrites `/v1/` → `http://localhost:8080/v1/` (see §7)
- Prod: relative `/v1/` served by Nginx reverse proxy in front of the Go backend

---

## 4. Component Breakdown by Page

### `/connect` — `ConnectPage.vue`

| Component | Responsibility |
|-----------|---------------|
| `ConnectForm` | Fields: API Key (password input), User ID (text), ACP Token (optional, password). Submit calls `authStore.connect()`. Shows inline error on failure. Redirects to `/chat` on success. |

States: idle / submitting (button disabled + spinner) / error (inline message) / success (redirect).

---

### `/chat` — `ChatPage.vue`

| Component | Responsibility |
|-----------|---------------|
| `SessionSidebar` | Lists sessions from `chatStore.sessions`. Highlights active. "New Chat" button. Delete session per row. Loading skeleton + empty "No sessions yet" + error retry. |
| `MessageList` | Renders `chatStore.messages`. Auto-scrolls to bottom on new message. Empty state: "Send a message to start." Error state: inline banner. |
| `MessageItem` | Single message bubble: role badge, content (pre-wrap), timestamp. |
| `ChatInput` | Textarea (Enter = send, Shift+Enter = newline). Send button disabled when `sendLoading`. Spinner on `sendLoading`. Error toast on `sendError`. |

States per component: loading (skeleton) / empty / error (retry/dismiss) / success (data rendered).

---

### `/memories` — `MemoriesPage.vue`

| Component | Responsibility |
|-----------|---------------|
| `MemoryTable` | n-data-table with columns: Key, Content (truncated), Actions. Loading skeleton. Empty: "No memories stored." Error: full-page error + retry button. |
| `MemoryDeleteConfirm` | n-popconfirm inline on each row. Disables row delete button while `deleteLoading.has(key)`. |

---

### `/skills` — `SkillsPage.vue` (read-only)

| Component | Responsibility |
|-----------|---------------|
| `SkillList` | Left sidebar. Lists skills. Active highlight. Loading skeleton. Empty + error states. |
| `SkillContentViewer` | Right pane. Displays raw YAML/Markdown in `<pre>` or n-code. Loading spinner when `contentLoading`. Empty: "Select a skill to view." Error: inline. |

---

### `/admin/skills` — `AdminSkillsPage.vue`

Extends `SkillsPage` layout. Adds:

| Component | Responsibility |
|-----------|---------------|
| `SkillUploadModal` | n-upload (single file, `.md`/`.yaml`). Field for skill name. `PUT /v1/skills/{name}` with `asAdmin: true`. Loading + error + success states. |
| Delete button in `SkillList` | Calls `skillStore.deleteSkill(name)` with `asAdmin: true`. Confirm via n-popconfirm. |

---

### `/admin/tenants` — `AdminTenantsPage.vue`

| Component | Responsibility |
|-----------|---------------|
| `TenantTable` | Columns: ID, Name, Created At. Loading skeleton. Empty + error. |
| `TenantCreateModal` | Fields: Tenant ID (slug), Name. `POST /v1/tenants` with `asAdmin: true`. |

---

### `/admin/keys` — `AdminKeysPage.vue`

| Component | Responsibility |
|-----------|---------------|
| `KeyTable` | Columns: ID, Name, Prefix, Tenant, Roles, Actions (revoke). Loading / empty / error. |
| `KeyCreateModal` | Fields: Tenant ID (select from tenant list), Name, Roles (checkbox). `POST /v1/api-keys` with `asAdmin: true`. Displays raw key in a one-time modal — warns user to copy now. |

---

## 5. 4-State Rule Enforcement

Every component that fetches data must implement all four states:

| State | Implementation |
|-------|---------------|
| **loading** | `n-skeleton` for list/table; spinner icon for detail pane; disabled submit button for forms |
| **empty** | Descriptive empty message with context-appropriate call-to-action (e.g. "Upload your first skill") |
| **error** | Inline error banner with human-readable message from `normalizeApiError()` + retry button where applicable |
| **success** | Data rendered; no spinner; no error banner visible |

Transition logic lives in the store action (sets loading/error/data), not in the component. Components are pure computed renders of store state.

---

## 6. Router Guard

```ts
// router/index.ts

const routes = [
  { path: '/connect',         component: ConnectPage,       meta: { public: true } },
  { path: '/chat',            component: ChatPage },
  { path: '/memories',        component: MemoriesPage },
  { path: '/skills',          component: SkillsPage },
  { path: '/admin/skills',    component: AdminSkillsPage,   meta: { requiresAdmin: true } },
  { path: '/admin/tenants',   component: AdminTenantsPage,  meta: { requiresAdmin: true } },
  { path: '/admin/keys',      component: AdminKeysPage,     meta: { requiresAdmin: true } },
  { path: '/:pathMatch(.*)*', redirect: '/connect' },
]

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.public) return true
  if (!auth.connected) return '/connect'
  if (to.meta.requiresAdmin && !auth.isAdmin) return '/chat'
  return true
})
```

Admin nav items (sidebar links to `/admin/*`) are rendered conditionally on `authStore.isAdmin`. Non-admin users who navigate directly to an admin route are silently redirected to `/chat`.

---

## 7. Dev Proxy Config (`vite.config.ts`)

```ts
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': path.resolve(__dirname, './src') },
  },
  server: {
    port: 5173,
    proxy: {
      '/v1': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        // No rewrite — path passes through as-is: /v1/chat/completions → :8080/v1/chat/completions
      },
    },
  },
})
```

Backend must be started with `ALLOWED_ORIGINS=http://localhost:5173` or the proxy removes the CORS requirement entirely during dev (proxy request has no Origin header from browser perspective).

---

## 8. Key Implementation Risks

| Risk | Impact | Mitigation |
|------|--------|-----------|
| **Session continuity across page reload** | sessionStorage is cleared on tab close, so users must re-enter credentials. This is by design per challenge session decision. Document it explicitly in `/connect` UI. | |
| **No streaming (SSE)** | Long backend responses will block the send button for the full response duration. | Show loading spinner + disable input immediately on send. Consider per-request timeout (e.g. 60s) with user-visible countdown or cancel button in v2. |
| **ACP Token = admin signal** | If a non-admin accidentally enters a value in the ACP Token field, they get admin access. | Label the field clearly ("Admin Token — leave blank if not admin"), and visually distinguish the admin nav from normal nav. |
| **Raw API key in one-time modal** | `POST /v1/api-keys` returns `key` only once. If user closes the modal before copying, key is lost. | Force the user to check a "I have copied the key" checkbox before the modal can be closed normally; only allow force-close with a warning. |
| **Skills endpoint gated on SkillsClient** | `GET /v1/skills` returns 404 (not registered) if MinIO is not configured. | `useApi` should treat 404 on skills endpoints as "empty, not error" with a specific message: "Skills not configured on this server." |
| **Admin routes require ACP token, not API key** | `Authorization` header must switch to `acpToken` for `/v1/tenants`, `/v1/api-keys` admin operations. | Enforced via `asAdmin: true` option in `useApi()`. All admin store actions must pass this flag — document as a convention, enforce in code review. |
| **CORS in production** | Browser direct-to-backend requires correct `Access-Control-Allow-Origin`. Backend already supports `ALLOWED_ORIGINS` env var. | Document required Nginx config (or backend env) in `deployment-context.md`. |
| **Naive UI tree-shaking** | Using auto-import plugin is recommended to avoid bundling the full component library. | Add `unplugin-vue-components` + `@naive-ui/resolver` to `vite.config.ts`. |
