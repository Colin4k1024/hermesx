# UI Design Specification: Three-Panel Workspace Layout

| Field | Value |
|-------|-------|
| Status | draft |
| Owner | frontend-engineer |
| Date | 2026-07-06 |
| Related | prd.md, arch-design.md, delivery-plan.md |

## 1. Layout Specification

### 1.1 Container Structure

The workspace uses Ant Design 6's native `Splitter` component to create a resizable three-panel layout. The top-level route `/workspace` replaces the existing `/chat` as the primary user-facing view.

```
WorkspaceLayout (Splitter, direction="horizontal")
  +-- Panel 1: TaskSidebar
  +-- Panel 2: DialogArea
  +-- Panel 3: ResultsPanel
```

### 1.2 Panel Dimensions

| Panel | Default Width | Min Width | Max Width | Collapse Trigger |
|-------|--------------|-----------|-----------|-----------------|
| TaskSidebar | 260px | 200px | 360px | viewport < 1024px |
| DialogArea | flex (fill) | 400px | - | never collapses |
| ResultsPanel | 360px | 280px | 520px | when no artifacts |

### 1.3 Splitter Configuration

```tsx
<Splitter style={{ height: '100vh' }}>
  <Splitter.Panel
    size={sidebarCollapsed ? 0 : 260}
    min={200}
    max={360}
    collapsible
  >
    <TaskSidebar />
  </Splitter.Panel>
  <Splitter.Panel min={400}>
    <DialogArea />
  </Splitter.Panel>
  <Splitter.Panel
    size={resultsPanelCollapsed ? 0 : 360}
    min={280}
    max={520}
    collapsible
  >
    <ResultsPanel />
  </Splitter.Panel>
</Splitter>
```

### 1.4 Visual Hierarchy

- Panel borders use `colorBorderSecondary` (dark: `#1f1f23`, light: `#f4f4f5`)
- Splitter drag handles: 4px hit area, 1px visual line, `colorBorder` color
- Background: panels use `colorBgContainer` (dark: `#111113`, light: `#ffffff`)
- No drop shadows between panels; clean flat separation

---

## 2. TaskSidebar Design

### 2.1 Structure

```
+----------------------------------+
|  [icon] 工作区    [+ New Task]   |  <- Header (48px)
+----------------------------------+
|  [Search input ...]              |  <- Search (40px)
+----------------------------------+
|  Today                           |  <- Group Label
|  |-  Generate Q2 report  [blue]  |  <- Task Item
|  |-  Translate contract  [green] |
|                                  |
|  This Week                       |
|  |-  Market research     [gray]  |
|  |-  PPT for meeting     [red]   |
|                                  |
|  Earlier                         |
|  |-  Budget analysis     [green] |
+----------------------------------+
```

### 2.2 Header

- Left: `Briefcase` icon (lucide) + "工作区" text, `fontSize: 15`, `fontWeight: 600`
- Right: `Plus` icon button, Tooltip "新建任务" (Cmd+N)
- Height: 48px, padding: `12px 16px`
- Border bottom: `1px solid colorBorderSecondary`

### 2.3 Search Input

- Ant Design `Input` with `Search` prefix icon
- Placeholder: "搜索任务..."
- Height: 32px, `borderRadius: 6`
- Margin: `8px 12px`
- Filters tasks by title match (client-side debounce 300ms)

### 2.4 Task List Grouping

Tasks are grouped by temporal proximity:

| Group Key | Label | Condition |
|-----------|-------|-----------|
| today | 今天 | `started_at` is today |
| thisWeek | 本周 | `started_at` within current week |
| earlier | 更早 | everything else |

Group headers: `fontSize: 12`, `fontWeight: 500`, `colorTextTertiary`, uppercase tracking, `padding: 8px 16px 4px`

### 2.5 Task Item

Each task item is 52px tall with the following layout:

```
+-------------------------------------------+
| [StatusDot]  Task title truncat...  12:34 |
|              Tool used / step info         |
+-------------------------------------------+
```

- **Title**: `fontSize: 13`, `fontWeight: 400`, max 1 line with `text-overflow: ellipsis`
- **Subtitle**: `fontSize: 11`, `colorTextTertiary`, shows last tool/step info
- **Time**: `fontSize: 11`, `colorTextTertiary`, right-aligned
- **Active state**: `colorBgElevated` background + left `3px` border in `colorPrimary`
- **Hover state**: `colorBgElevated` background
- **Padding**: `8px 16px`
- **Border radius**: `6px`

### 2.6 Status Badges

| Status | Color | Visual | Token |
|--------|-------|--------|-------|
| running | blue | pulsing dot (CSS animation) | `colorPrimary` |
| completed | green | solid dot | `colorSuccess` |
| failed | red | solid dot | `colorError` |
| pending | gray | hollow dot (ring only) | `colorTextTertiary` |

Dot size: 8px diameter, vertically centered against title. The running state uses a `pulse` keyframe animation (scale 1.0 -> 1.4 -> 1.0, opacity 1.0 -> 0.6 -> 1.0) with 1.5s duration.

### 2.7 Empty State

When no tasks exist:
- Illustration: `ClipboardList` icon at 48px, `colorTextTertiary`
- Text: "还没有任务，点击上方按钮创建"
- CTA: Ghost button "新建任务"

### 2.8 Future: Folder Organization

Reserved slot below search for a folder selector dropdown. Not implemented in V1, but the layout must accommodate insertion without breaking existing structure.

---

## 3. DialogArea Design

### 3.1 Structure

```
+------------------------------------------------+
|  [StatusDot] Task Title        [Collapse btn]  |  <- Top Bar (48px)
+------------------------------------------------+
|                                                |
|  [PlanSteps - collapsible]                     |  <- Plan Section
|                                                |
|  [User message bubble]                         |  <- Messages
|                                                |
|  [Assistant message bubble]                    |
|                                                |
|  [User message bubble]                         |
|                                                |
|  [Assistant message + streaming cursor]        |
|                                                |
+------------------------------------------------+
|  [TextArea input]                 [Send/Stop]  |  <- InputBar (auto-height)
+------------------------------------------------+
```

### 3.2 Top Bar

- Left: Status dot (same badge system as sidebar) + Task title
- Title: `fontSize: 14`, `fontWeight: 500`, `colorText`
- Right: `PanelRightClose` icon button to toggle results panel
- Height: 48px, padding: `0 16px`
- Border bottom: `1px solid colorBorderSecondary`
- When no task selected: show "选择或创建一个任务开始" centered placeholder

### 3.3 PlanSteps Component

Displayed as a collapsible card at the top of the message area when the current task has plan data.

```
+--------------------------------------------+
|  [v] 执行计划  (3/5 completed)             |  <- Collapse header
+--------------------------------------------+
|  [check] 分析需求文档           2s         |
|  [check] 提取关键数据           5s         |
|  [check] 生成表格结构           3s         |
|  [spin]  填充数据并格式化       12s...     |  <- active step
|  [circle] 导出为 xlsx 文件                 |  <- pending
+--------------------------------------------+
```

**Step Card Layout:**

| Element | Spec |
|---------|------|
| Icon (left) | 20px, CircleCheck (green) / Loader (blue, spinning) / Circle (gray) |
| Title | `fontSize: 13`, `fontWeight: 400` |
| Duration (right) | `fontSize: 11`, `colorTextTertiary`, monospace |
| Row height | 36px |
| Padding | `8px 12px` |

**Card Container:**
- Background: `colorBgElevated`
- Border: `1px solid colorBorderSecondary`
- Border radius: `8px`
- Margin: `12px 16px`
- Collapse animation: 200ms ease-out (matches `TRANSITION_NORMAL`)

### 3.4 Message List

Reuses the existing chat bubble pattern from `Chat.tsx` with these refinements:

**User Messages:**
- Align: right
- Background: `colorPrimary` (`#6366f1` dark / `#4f46e5` light)
- Text color: white
- Max width: 70%
- Border radius: `12px 12px 4px 12px`

**Assistant Messages:**
- Align: left
- Background: `colorBgElevated`
- Border: `1px solid colorBorder`
- Text color: `colorText`
- Max width: 80%
- Border radius: `12px 12px 12px 4px`
- Supports markdown rendering (future: code blocks, tables)

**Streaming Indicator:**
- Block cursor (blinking `|`) appended to last assistant message
- Typing indicator: 3 dots animation when waiting for first token

**Message Spacing:**
- Gap between messages: 12px
- Container padding: `16px 24px`
- Scroll: `overflow-y: auto`, smooth scroll to bottom on new message

### 3.5 InputBar

Reuses the existing input pattern with visual refinements:

- Container: padding `12px 16px`, border-top `1px solid colorBorderSecondary`
- TextArea: `autoSize={{ minRows: 1, maxRows: 6 }}`, `borderRadius: 8`
- Send button: `type="primary"`, `borderRadius: 8`, right of textarea
- Stop button: `danger`, replaces Send when streaming
- Keyboard: Enter sends, Shift+Enter newline
- Placeholder: "描述你的任务..." (when no active task) / "继续对话..." (when task active)

---

## 4. ResultsPanel Design

### 4.1 Structure

```
+--------------------------------------+
|  [Artifacts] | [Files]     [x close] |  <- Tab Bar (44px)
+--------------------------------------+
|                                      |
|  +--------------------------------+  |
|  | [xlsx] Q2-report.xlsx          |  |  <- File Card
|  | 142 KB  ·  12:34              |  |
|  | [Preview]  [Download]          |  |
|  +--------------------------------+  |
|                                      |
|  +--------------------------------+  |
|  | [docx] contract-draft.docx    |  |
|  | 58 KB  ·  12:31               |  |
|  | [Preview]  [Download]          |  |
|  +--------------------------------+  |
|                                      |
+--------------------------------------+
```

### 4.2 Tab Bar

- Two tabs: "产物" (default) / "文件"
- Tab style: Ant Design `Tabs` with `type="line"`, `size="small"`
- Right: `X` icon button to collapse the panel (Cmd+E)
- Height: 44px
- Border bottom: `1px solid colorBorderSecondary`

### 4.3 Artifacts Tab

Displays generated files from the current task session.

**File Card:**

```
+----------------------------------------------+
|  [FileIcon 32px]                             |
|  Filename.xlsx                               |
|  142 KB  ·  2 minutes ago                    |
|                                              |
|  [Eye Preview]  [Download DownloadCloud]     |
+----------------------------------------------+
```

| Element | Spec |
|---------|------|
| Card | `colorBgElevated` bg, `1px solid colorBorderSecondary`, `borderRadius: 8` |
| Icon | 32px, color-coded by type (xlsx=green, docx=blue, pptx=orange, pdf=red) |
| Filename | `fontSize: 13`, `fontWeight: 500`, `colorText`, truncate with ellipsis |
| Meta | `fontSize: 11`, `colorTextTertiary` |
| Actions | Two ghost buttons, `fontSize: 12`, spaced with 8px gap |
| Card padding | 12px |
| Card gap | 8px between cards |
| Container padding | 12px |

**File Type Icons (lucide):**
- `.xlsx`: `Sheet` icon, `colorSuccess`
- `.docx`: `FileText` icon, `colorPrimary`
- `.pptx`: `Presentation` icon, `#f59e0b` (warning)
- `.pdf`: `FileType` icon, `colorError`
- Other: `File` icon, `colorTextTertiary`

### 4.4 Files Tab

Displays a workspace file browser using a tree view.

- Ant Design `Tree` component with directory icons
- Root: `workspace/` (user's file workspace)
- File click: opens FilePreview modal
- Right-click context menu: Download, Delete, Rename (future)
- Empty: "工作区为空"

### 4.5 File Preview Modal

Opens as an Ant Design `Modal` (fullscreen on mobile, 80vw/80vh on desktop).

| Format | Preview Method |
|--------|---------------|
| `.xlsx` | Table render via SheetJS (first sheet, first 100 rows) |
| `.docx` | Rich text render via mammoth.js |
| `.pptx` | Slide thumbnails (one per page, image extraction) |
| `.pdf` | Native browser PDF embed (`<object>` or `react-pdf`) |
| `.png/.jpg` | `<img>` with zoom controls |
| `.md` | Markdown render (react-markdown) |
| Other | Download prompt, no inline preview |

**Modal Layout:**
- Header: filename + file size + download button
- Body: preview content, scrollable
- Footer: close button
- Border radius: `8px` (via Ant Design `styles.content`)

### 4.6 Empty State (Artifacts Tab)

When the active task has no generated artifacts:

```
+--------------------------------------+
|                                      |
|         [Package icon 48px]          |
|                                      |
|   任务完成后，产物将在此展示         |
|                                      |
+--------------------------------------+
```

- Icon: `Package` (lucide), 48px, `colorTextTertiary`
- Text: `fontSize: 13`, `colorTextTertiary`, centered
- Vertical centering within available space

---

## 5. State Management (workspaceStore)

### 5.1 Store Definition

```typescript
// webui/src/shared/stores/workspaceStore.ts

interface TaskSession {
  id: string
  title: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  createdAt: string
  updatedAt: string
  messages: DisplayMessage[]
  planSteps: PlanStep[]
  artifacts: Artifact[]
  sseController: AbortController | null
}

interface PlanStep {
  id: string
  title: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  startedAt?: string
  completedAt?: string
  durationMs?: number
}

interface Artifact {
  id: string
  filename: string
  mimeType: string
  sizeBytes: number
  createdAt: string
  downloadUrl: string
  previewUrl?: string
}

interface WorkspaceState {
  // Session management
  activeSessionId: string | null
  sessions: Map<string, TaskSession>

  // Panel state
  sidebarCollapsed: boolean
  resultsPanelCollapsed: boolean
  resultsPanelActiveTab: 'artifacts' | 'files'

  // Search
  searchQuery: string

  // Actions
  createSession: (title: string) => Promise<string>
  switchSession: (id: string) => void
  deleteSession: (id: string) => void
  updateSessionStatus: (id: string, status: TaskSession['status']) => void
  addMessage: (sessionId: string, msg: DisplayMessage) => void
  updateLastMessage: (sessionId: string, content: string) => void
  setPlanSteps: (sessionId: string, steps: PlanStep[]) => void
  updatePlanStep: (sessionId: string, stepId: string, patch: Partial<PlanStep>) => void
  addArtifact: (sessionId: string, artifact: Artifact) => void

  // Panel actions
  toggleSidebar: () => void
  toggleResultsPanel: () => void
  setResultsPanelTab: (tab: 'artifacts' | 'files') => void
  setSearchQuery: (q: string) => void
}
```

### 5.2 Derived State

```typescript
// Computed selectors (outside store, use Zustand subscribeWithSelector)

const selectActiveSession = (state: WorkspaceState) =>
  state.activeSessionId ? state.sessions.get(state.activeSessionId) : null

const selectFilteredSessions = (state: WorkspaceState) => {
  const q = state.searchQuery.toLowerCase()
  const all = Array.from(state.sessions.values())
  if (!q) return all
  return all.filter(s => s.title.toLowerCase().includes(q))
}

const selectGroupedSessions = (state: WorkspaceState) => {
  const sessions = selectFilteredSessions(state)
  const now = new Date()
  const startOfDay = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const startOfWeek = new Date(startOfDay)
  startOfWeek.setDate(startOfWeek.getDate() - startOfWeek.getDay())

  return {
    today: sessions.filter(s => new Date(s.createdAt) >= startOfDay),
    thisWeek: sessions.filter(s => {
      const d = new Date(s.createdAt)
      return d >= startOfWeek && d < startOfDay
    }),
    earlier: sessions.filter(s => new Date(s.createdAt) < startOfWeek),
  }
}
```

### 5.3 Persistence

- `activeSessionId` and panel states persist to `localStorage` via Zustand `persist` middleware
- Session messages are fetched on-demand from API (not persisted locally)
- Session list is fetched on mount and kept in-memory

---

## 6. Responsive Behavior

### 6.1 Breakpoint Definitions

| Breakpoint | Range | Layout Strategy |
|------------|-------|-----------------|
| Desktop XL | >= 1440px | All panels visible, generous spacing |
| Desktop | 1280-1439px | All panels visible, compact spacing |
| Tablet Landscape | 1024-1279px | Sidebar collapsed to icon strip (56px), expandable |
| Tablet Portrait | 768-1023px | Tab-based navigation |
| Mobile | < 768px | Tab-based navigation, full-screen panels |

### 6.2 Desktop Layout (>= 1280px)

```
+--------+------------------------+-----------+
|        |                        |           |
| Task   |      Dialog Area       |  Results  |
| Side   |                        |  Panel    |
| bar    |  [Plan Steps]          |           |
|        |                        | [Tabs]    |
| 260px  |  [Messages]            |           |
|        |                        | [Cards]   |
|        |  [Input Bar]           |           |
|        |                        |           |
+--------+------------------------+-----------+
  260px         flex                  360px
```

### 6.3 Tablet Landscape (1024-1279px)

```
+----+----------------------------+-----------+
|    |                            |           |
| [] |       Dialog Area          |  Results  |
| [] |                            |  Panel    |
| [] |  [Plan Steps]             |           |
| [] |                            | [Tabs]    |
| [] |  [Messages]               |           |
|    |                            | [Cards]   |
|    |  [Input Bar]              |           |
|    |                            |           |
+----+----------------------------+-----------+
 56px         flex                   360px
```

- Sidebar collapses to a vertical icon strip (56px wide)
- Shows only status dots and task icons (first letter avatar)
- Hover/click expands sidebar as an overlay (absolute positioned, z-index 100)
- Overlay has `box-shadow: 4px 0 16px rgba(0,0,0,0.12)` (dark: `rgba(0,0,0,0.4)`)
- Click outside or press Escape dismisses the overlay

### 6.4 Tablet Portrait / Mobile (< 1024px)

```
+--------------------------------------+
| [Sidebar] | [Dialog] | [Results]     |  <- Tab Bar (44px)
+--------------------------------------+
|                                      |
|      Active Tab Content              |
|      (full width, full height)       |
|                                      |
+--------------------------------------+
```

- Bottom tab bar with 3 tabs: Sidebar / Dialog / Results
- Each tab shows its panel at full viewport width
- Dialog tab is default
- Badge on Sidebar tab shows count of running tasks
- Badge on Results tab shows count of new artifacts

### 6.5 Transition Behavior

- Panel collapse/expand: `200ms` ease-in-out (CSS transition on width)
- Tab switch: immediate, no animation (mobile)
- Sidebar overlay: `150ms` slide-in from left

---

## 7. Keyboard Shortcuts

| Shortcut | Action | Context |
|----------|--------|---------|
| `Cmd+B` / `Ctrl+B` | Toggle sidebar | Global |
| `Cmd+E` / `Ctrl+E` | Toggle results panel | Global |
| `Cmd+N` / `Ctrl+N` | Create new task | Global |
| `Cmd+Up` / `Ctrl+Up` | Switch to previous task | Global |
| `Cmd+Down` / `Ctrl+Down` | Switch to next task | Global |
| `Enter` | Send message | Input focused |
| `Shift+Enter` | New line in input | Input focused |
| `Escape` | Close file preview modal / collapse sidebar overlay | Modal or overlay open |

Implementation: Register via `useEffect` with `keydown` listener on `document`. Use `useCallback` refs to avoid stale closures. Respect `e.metaKey` (Mac) / `e.ctrlKey` (Windows/Linux) via a unified `isModKey` helper.

---

## 8. Design Token Usage

All visual values reference the existing token system from `webui/src/shared/theme/tokens.ts`:

| Usage | Dark Token | Light Token |
|-------|-----------|-------------|
| Panel background | `colorBgContainer` (#111113) | `colorBgContainer` (#ffffff) |
| Elevated cards | `colorBgElevated` (#18181b) | `colorBgElevated` (#ffffff) |
| Primary borders | `colorBorder` (#27272a) | `colorBorder` (#e4e4e7) |
| Secondary borders | `colorBorderSecondary` (#1f1f23) | `colorBorderSecondary` (#f4f4f5) |
| Primary text | `colorText` (#fafafa) | `colorText` (#09090b) |
| Secondary text | `colorTextSecondary` (#a1a1aa) | `colorTextSecondary` (#71717a) |
| Tertiary text | `colorTextTertiary` (#52525b) | `colorTextTertiary` (#a1a1aa) |
| Accent / Primary | `colorPrimary` (#6366f1) | `colorPrimary` (#4f46e5) |
| Success | `colorSuccess` (#22c55e) | `colorSuccess` (#16a34a) |
| Error | `colorError` (#ef4444) | `colorError` (#dc2626) |
| Warning | `colorWarning` (#f59e0b) | `colorWarning` (#d97706) |

### New Constants (to add to `constants.ts`)

```typescript
export const WORKSPACE_SIDEBAR_WIDTH = 260
export const WORKSPACE_SIDEBAR_MIN = 200
export const WORKSPACE_SIDEBAR_MAX = 360
export const WORKSPACE_SIDEBAR_COLLAPSED = 56
export const WORKSPACE_RESULTS_WIDTH = 360
export const WORKSPACE_RESULTS_MIN = 280
export const WORKSPACE_RESULTS_MAX = 520
export const WORKSPACE_DIALOG_MIN = 400
export const WORKSPACE_TOPBAR_HEIGHT = 48
```

---

## 9. Component File Structure

```
webui/src/user/
  components/
    workspace/
      WorkspaceLayout.tsx        <- Splitter container + responsive logic
      TaskSidebar.tsx            <- Left panel
      TaskItem.tsx               <- Single task row
      TaskGroupLabel.tsx         <- "Today" / "This Week" group header
      DialogArea.tsx             <- Center panel
      PlanSteps.tsx              <- Collapsible plan card
      StepRow.tsx                <- Single step in plan
      MessageList.tsx            <- Scrollable message container
      MessageBubble.tsx          <- User/Assistant bubble
      InputBar.tsx               <- TextArea + Send/Stop
      ResultsPanel.tsx           <- Right panel
      ArtifactCard.tsx           <- File card in results
      FileTreeView.tsx           <- Workspace file browser
      FilePreviewModal.tsx       <- Multi-format preview
      StatusDot.tsx              <- Reusable status indicator
      index.ts                   <- Barrel exports
  pages/
    Workspace.tsx               <- Route entry, mounts WorkspaceLayout
```

---

## 10. Accessibility

| Requirement | Implementation |
|-------------|----------------|
| Keyboard navigation | All interactive elements reachable via Tab; task list navigable with arrow keys |
| Focus visibility | Default Ant Design focus ring; never removed |
| Screen reader | Status dot has `aria-label` ("Running" / "Completed" etc.) |
| Landmarks | Sidebar: `<nav aria-label="Task list">`, Dialog: `<main>`, Results: `<aside>` |
| Reduced motion | `@media (prefers-reduced-motion: reduce)` disables pulse animation |
| Color contrast | All text meets WCAG 2.1 AA (4.5:1 for body text, 3:1 for large text) |
| Live region | New assistant messages announced via `aria-live="polite"` |

---

## 11. Animation Specification

| Animation | Property | Duration | Easing | Trigger |
|-----------|----------|----------|--------|---------|
| Sidebar collapse | width | 200ms | ease-in-out | Toggle or breakpoint |
| Results collapse | width | 200ms | ease-in-out | Toggle or empty artifacts |
| PlanSteps expand | max-height + opacity | 200ms | ease-out | Click collapse header |
| Status pulse | transform + opacity | 1500ms | ease-in-out | Status = "running" |
| Message appear | opacity + translateY(8px) | 150ms | ease-out | New message added |
| File card hover | border-color | 150ms | ease | Mouse enter |
| Sidebar overlay slide | translateX | 150ms | ease-out | Hover/click in tablet mode |

---

## 12. Error States

| Scenario | UI Response |
|----------|-------------|
| SSE connection lost | Toast notification + reconnect attempt (3x), then error badge on task |
| Task execution failed | Status turns red, last message shows error with retry button |
| File preview unavailable | Fallback to download prompt with format info |
| Session load failed | Inline error in dialog area with retry link |
| No network | Global banner at top: "网络连接中断，正在重连..." |

---

## 13. ASCII Wireframes

### 13.1 Full Desktop (>= 1280px)

```
+==================================================================+
| [Briefcase] 工作区    [+]  |                   | [Tabs] [X]      |
+----------------------------+                   +-----------------+
| [Search...]                | [dot] Task Title  | 产物  |  文件   |
+----------------------------+-------------------+-----------------+
| Today                      |                   |                 |
| [*] Generate report  12:34 | +---------------+ | +-----------+   |
| [ ] Translate docs   12:30 | | 执行计划(3/5) | | | [xlsx]    |   |
|                            | | [v] Step 1  2s| | | report.xl |   |
| This Week                  | | [v] Step 2  5s| | | 142KB     |   |
| [ ] Research         Mon   | | [o] Step 3 ..| | | [Eye][DL] |   |
| [x] Failed task      Mon   | | [ ] Step 4   | | +-----------+   |
|                            | | [ ] Step 5   | |                 |
| Earlier                    | +---------------+ | +-----------+   |
| [v] Budget review    Jun   |                   | | [docx]    |   |
| [v] Email draft      Jun   | User: 帮我生成   | | contract  |   |
|                            | Q2报告           | | 58KB      |   |
|                            |                   | | [Eye][DL] |   |
|                            | AI: 好的，我来    | +-----------+   |
|                            | 分析数据并生成   |                 |
|                            | 报告...          |                 |
|                            |                   |                 |
|                            |                   |                 |
+----------------------------+-------------------+-----------------+
|                            | [Input area...]        [Send]      |
+----------------------------+-----------------+-----------------+
```

### 13.2 Tablet Landscape (1024-1279px)

```
+====+=============================================+================+
| [] |                                             |  产物 | 文件   |
| [] | [dot] Generate Q2 Report                    +----------------+
| [] |                                             |                |
| [] | +---------------------------------------+   | +-----------+  |
| [] | | 执行计划 (3/5)                        |   | | report.xl |  |
|    | | [v] Step 1    [v] Step 2    [o] 3... |   | | 142KB     |  |
|    | +---------------------------------------+   | | [Eye][DL] |  |
|    |                                             | +-----------+  |
|    | User: 帮我生成Q2报告                        |                |
|    |                                             |                |
|    | AI: 好的，正在处理...                        |                |
|    |                                             |                |
|    |                                             |                |
+----+---------------------------------------------+----------------+
|    | [Input area...]                      [Send] |                |
+====+=============================================+================+
 56px              flex                               360px
```

### 13.3 Mobile / Tablet Portrait (< 1024px)

```
+==========================================+
|                                          |
|  [dot] Generate Q2 Report               |
|                                          |
|  +------------------------------------+  |
|  | 执行计划 (3/5)              [v]    |  |
|  | [v] Analyze    [v] Extract  [o]... |  |
|  +------------------------------------+  |
|                                          |
|  User: 帮我生成Q2报告                    |
|                                          |
|  AI: 好的，正在分析数据并生成报告...      |
|      |                                   |
|                                          |
|  [Input area...]               [Send]   |
|                                          |
+==========================================+
| [TaskList]  | [*Dialog*]  | [Results]    |  <- Bottom tabs
+==========================================+
```

---

## 14. Integration with Existing Code

### 14.1 Reusable from Chat.tsx

| Element | Current Location | Target Component |
|---------|-----------------|------------------|
| Message rendering logic | Chat.tsx L144-169 | `MessageBubble.tsx` |
| Input + Send/Stop | Chat.tsx L175-189 | `InputBar.tsx` |
| Session list rendering | Chat.tsx L117-138 | `TaskItem.tsx` |
| SSE streaming hook | `useSse` hook | Reused as-is |
| API client | `apiClient` | Reused as-is |

### 14.2 New Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `antd` (Splitter) | ^6.0.0 | Already included, Splitter is native |
| `sheetjs` | ^0.20.0 | xlsx preview in FilePreviewModal |
| `mammoth` | ^1.8.0 | docx preview in FilePreviewModal |
| `react-pdf` | ^9.0.0 | PDF embed in FilePreviewModal |
| `react-markdown` | ^9.0.0 | Markdown rendering (if not already) |

### 14.3 API Endpoints Required

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `GET /v1/sessions` | GET | List tasks (already exists) |
| `GET /v1/sessions/:id` | GET | Load task detail (already exists) |
| `POST /v1/sessions` | POST | Create new task session |
| `GET /v1/sessions/:id/artifacts` | GET | List task artifacts (new) |
| `GET /v1/files/:path` | GET | Download/preview file (new) |
| `GET /v1/workspace/tree` | GET | File tree listing (new) |
| `DELETE /v1/sessions/:id` | DELETE | Delete task (new) |

---

## 15. Implementation Priority

| Phase | Components | Dependency |
|-------|-----------|------------|
| P0 | WorkspaceLayout, TaskSidebar, DialogArea, InputBar, MessageBubble | None |
| P1 | PlanSteps, StepRow, StatusDot, workspaceStore | P0 |
| P2 | ResultsPanel, ArtifactCard, FileTreeView | P1 + backend artifacts API |
| P3 | FilePreviewModal (xlsx/docx/pptx/pdf) | P2 + file download API |
| P4 | Responsive behavior (tablet/mobile) | P0-P2 stable |
| P5 | Keyboard shortcuts, accessibility polish | P0-P4 |

---

## 16. Open Questions

| # | Question | Impact | Owner |
|---|----------|--------|-------|
| 1 | Should task deletion require confirmation modal? | UX safety | product-manager |
| 2 | Max concurrent SSE connections per user? | Browser limit (6 per domain) | architect |
| 3 | File preview size limit before forcing download? | Performance | frontend-engineer |
| 4 | Should PlanSteps auto-expand when task starts? | UX flow | product-manager |
| 5 | Drag-and-drop file upload to workspace? | Scope V1 vs V2 | tech-lead |
