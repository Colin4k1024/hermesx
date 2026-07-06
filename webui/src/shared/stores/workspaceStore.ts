import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { Session, ChatMessage } from '@shared/types'

/* ------------------------------------------------------------------ */
/*  Types                                                             */
/* ------------------------------------------------------------------ */

export interface DisplayMessage extends ChatMessage {
  id: string
}

export interface TaskSession {
  id: string
  title: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  createdAt: string
  updatedAt: string
  messages: DisplayMessage[]
  artifacts: Artifact[]
}

export interface Artifact {
  id: string
  filename: string
  mimeType: string
  sizeBytes: number
  createdAt: string
  downloadUrl: string
  previewUrl?: string
}

export interface PlanStep {
  id: string
  title: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  startedAt?: number
  completedAt?: number
}

/* ------------------------------------------------------------------ */
/*  State & Actions                                                   */
/* ------------------------------------------------------------------ */

interface WorkspaceState {
  // Session management
  activeSessionId: string | null
  sessions: Map<string, TaskSession>

  // Streaming tracking
  streamingSessions: Set<string>

  // Plan steps per session
  planSteps: Map<string, PlanStep[]>

  // Panel state
  sidebarCollapsed: boolean
  resultsPanelCollapsed: boolean
  resultsPanelActiveTab: 'artifacts' | 'files'

  // Search
  searchQuery: string

  // Actions
  setSessions: (sessions: Session[]) => void
  upsertSession: (session: TaskSession) => void
  switchSession: (id: string | null) => void
  deleteSession: (id: string) => void
  updateSessionStatus: (id: string, status: TaskSession['status']) => void
  addMessage: (sessionId: string, msg: DisplayMessage) => void
  updateLastMessage: (sessionId: string, content: string) => void
  addArtifact: (sessionId: string, artifact: Artifact) => void
  addStreamingSession: (sessionId: string) => void
  removeStreamingSession: (sessionId: string) => void

  // Plan actions
  addPlanSteps: (sessionId: string, steps: PlanStep[]) => void
  updatePlanStep: (sessionId: string, stepId: string, status: PlanStep['status']) => void
  clearPlanSteps: (sessionId: string) => void

  // Panel actions
  toggleSidebar: () => void
  toggleResultsPanel: () => void
  setResultsPanelTab: (tab: 'artifacts' | 'files') => void
  setSearchQuery: (q: string) => void
}

/* ------------------------------------------------------------------ */
/*  Selectors                                                         */
/* ------------------------------------------------------------------ */

export const selectActiveSession = (state: WorkspaceState) =>
  state.activeSessionId ? state.sessions.get(state.activeSessionId) ?? null : null

export const selectFilteredSessions = (state: WorkspaceState) => {
  const q = state.searchQuery.toLowerCase()
  const all = Array.from(state.sessions.values())
  if (!q) return all
  return all.filter((s) => s.title.toLowerCase().includes(q))
}

export const selectGroupedSessions = (state: WorkspaceState) => {
  const sessions = selectFilteredSessions(state)
  const now = new Date()
  const startOfDay = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const startOfWeek = new Date(startOfDay)
  startOfWeek.setDate(startOfWeek.getDate() - startOfWeek.getDay())

  return {
    today: sessions.filter((s) => new Date(s.createdAt) >= startOfDay),
    thisWeek: sessions.filter((s) => {
      const d = new Date(s.createdAt)
      return d >= startOfWeek && d < startOfDay
    }),
    earlier: sessions.filter((s) => new Date(s.createdAt) < startOfWeek),
  }
}

export const selectIsStreaming = (sessionId: string) => (state: WorkspaceState) =>
  state.streamingSessions.has(sessionId)

/* ------------------------------------------------------------------ */
/*  Store                                                             */
/* ------------------------------------------------------------------ */

export const useWorkspaceStore = create<WorkspaceState>()(
  persist(
    (set, get) => ({
      // Initial state
      activeSessionId: null,
      sessions: new Map(),
      streamingSessions: new Set(),
      planSteps: new Map(),
      sidebarCollapsed: false,
      resultsPanelCollapsed: false,
      resultsPanelActiveTab: 'artifacts',
      searchQuery: '',

      // Session actions
      setSessions: (sessions) => {
        const map = new Map<string, TaskSession>()
        for (const s of sessions) {
          const existing = get().sessions.get(s.id)
          map.set(s.id, existing ?? {
            id: s.id,
            title: s.title ?? s.id.slice(0, 12),
            status: 'completed',
            createdAt: s.started_at,
            updatedAt: s.ended_at ?? s.started_at,
            messages: [],
            artifacts: [],
          })
        }
        set({ sessions: map })
      },

      upsertSession: (session) =>
        set((state) => {
          const next = new Map(state.sessions)
          next.set(session.id, session)
          return { sessions: next }
        }),

      switchSession: (id) => set({ activeSessionId: id }),

      deleteSession: (id) =>
        set((state) => {
          const next = new Map(state.sessions)
          next.delete(id)
          return {
            sessions: next,
            activeSessionId: state.activeSessionId === id ? null : state.activeSessionId,
          }
        }),

      updateSessionStatus: (id, status) =>
        set((state) => {
          const s = state.sessions.get(id)
          if (!s) return {}
          const next = new Map(state.sessions)
          next.set(id, { ...s, status, updatedAt: new Date().toISOString() })
          return { sessions: next }
        }),

      addMessage: (sessionId, msg) =>
        set((state) => {
          const s = state.sessions.get(sessionId)
          if (!s) return {}
          const next = new Map(state.sessions)
          next.set(sessionId, { ...s, messages: [...s.messages, msg], updatedAt: new Date().toISOString() })
          return { sessions: next }
        }),

      updateLastMessage: (sessionId, content) =>
        set((state) => {
          const s = state.sessions.get(sessionId)
          if (!s || s.messages.length === 0) return {}
          const msgs = [...s.messages]
          const last = msgs[msgs.length - 1]
          if (!last) return {}
          msgs[msgs.length - 1] = { ...last, content }
          const next = new Map(state.sessions)
          next.set(sessionId, { ...s, messages: msgs, updatedAt: new Date().toISOString() })
          return { sessions: next }
        }),

      addArtifact: (sessionId, artifact) =>
        set((state) => {
          const s = state.sessions.get(sessionId)
          if (!s) return {}
          const next = new Map(state.sessions)
          next.set(sessionId, {
            ...s,
            artifacts: [...s.artifacts, artifact],
            updatedAt: new Date().toISOString(),
          })
          return { sessions: next }
        }),

      addStreamingSession: (sessionId) =>
        set((state) => {
          const next = new Set(state.streamingSessions)
          next.add(sessionId)
          return { streamingSessions: next }
        }),

      removeStreamingSession: (sessionId) =>
        set((state) => {
          const next = new Set(state.streamingSessions)
          next.delete(sessionId)
          return { streamingSessions: next }
        }),

      // Plan actions
      addPlanSteps: (sessionId, steps) =>
        set((state) => {
          const existing = state.planSteps.get(sessionId) ?? []
          const existingIds = new Set(existing.map((s) => s.id))
          const merged = [...existing]
          for (const step of steps) {
            if (!existingIds.has(step.id)) {
              merged.push(step)
            }
          }
          const next = new Map(state.planSteps)
          next.set(sessionId, merged)
          return { planSteps: next }
        }),

      updatePlanStep: (sessionId, stepId, status) =>
        set((state) => {
          const steps = state.planSteps.get(sessionId)
          if (!steps) return {}
          const now = Date.now()
          const updated = steps.map((s) => {
            if (s.id !== stepId) return s
            const patch: Partial<PlanStep> = { status }
            if (status === 'running' && !s.startedAt) patch.startedAt = now
            if (status === 'completed' || status === 'failed') patch.completedAt = now
            return { ...s, ...patch }
          })
          const next = new Map(state.planSteps)
          next.set(sessionId, updated)
          return { planSteps: next }
        }),

      clearPlanSteps: (sessionId) =>
        set((state) => {
          const next = new Map(state.planSteps)
          next.delete(sessionId)
          return { planSteps: next }
        }),

      // Panel actions
      toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),

      toggleResultsPanel: () => set((s) => ({ resultsPanelCollapsed: !s.resultsPanelCollapsed })),

      setResultsPanelTab: (tab) => set({ resultsPanelActiveTab: tab }),

      setSearchQuery: (q) => set({ searchQuery: q }),
    }),
    {
      name: 'hx-workspace',
      // Only persist panel preferences and active session id
      partialize: (state) => ({
        activeSessionId: state.activeSessionId,
        sidebarCollapsed: state.sidebarCollapsed,
        resultsPanelCollapsed: state.resultsPanelCollapsed,
        resultsPanelActiveTab: state.resultsPanelActiveTab,
      }),
      // Map cannot be serialized by default; we store sessions in-memory only
    },
  ),
)
