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
  // Session management — plain objects for stable references
  activeSessionId: string | null
  sessions: Record<string, TaskSession>

  // Streaming tracking — plain object for stable references
  streaming: Record<string, boolean>

  // Plan steps per session
  planSteps: Record<string, PlanStep[]>

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

export const selectActiveSession = (state: WorkspaceState): TaskSession | null =>
  state.activeSessionId ? state.sessions[state.activeSessionId] ?? null : null

export const selectFilteredSessions = (state: WorkspaceState): TaskSession[] => {
  const q = state.searchQuery.toLowerCase()
  const all = Object.values(state.sessions)
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
  !!state.streaming[sessionId]

/* ------------------------------------------------------------------ */
/*  Store                                                             */
/* ------------------------------------------------------------------ */

export const useWorkspaceStore = create<WorkspaceState>()(
  persist(
    (set, get) => ({
      // Initial state
      activeSessionId: null,
      sessions: {},
      streaming: {},
      planSteps: {},
      sidebarCollapsed: false,
      resultsPanelCollapsed: false,
      resultsPanelActiveTab: 'artifacts',
      searchQuery: '',

      // Session actions
      setSessions: (sessions) => {
        const existing = get().sessions
        const next: Record<string, TaskSession> = {}
        for (const s of sessions) {
          next[s.id] = existing[s.id] ?? {
            id: s.id,
            title: s.title ?? s.id.slice(0, 12),
            status: 'completed',
            createdAt: s.started_at,
            updatedAt: s.ended_at ?? s.started_at,
            messages: [],
            artifacts: [],
          }
        }
        set({ sessions: next })
      },

      upsertSession: (session) =>
        set((state) => ({
          sessions: { ...state.sessions, [session.id]: session },
        })),

      switchSession: (id) => set({ activeSessionId: id }),

      deleteSession: (id) =>
        set((state) => {
          const { [id]: _, ...rest } = state.sessions
          return {
            sessions: rest,
            activeSessionId: state.activeSessionId === id ? null : state.activeSessionId,
          }
        }),

      updateSessionStatus: (id, status) =>
        set((state) => {
          const s = state.sessions[id]
          if (!s) return {}
          return {
            sessions: {
              ...state.sessions,
              [id]: { ...s, status, updatedAt: new Date().toISOString() },
            },
          }
        }),

      addMessage: (sessionId, msg) =>
        set((state) => {
          const s = state.sessions[sessionId]
          if (!s) return {}
          return {
            sessions: {
              ...state.sessions,
              [sessionId]: {
                ...s,
                messages: [...s.messages, msg],
                updatedAt: new Date().toISOString(),
              },
            },
          }
        }),

      updateLastMessage: (sessionId, content) =>
        set((state) => {
          const s = state.sessions[sessionId]
          if (!s || s.messages.length === 0) return {}
          const msgs = [...s.messages]
          const last = msgs[msgs.length - 1]
          if (!last) return {}
          msgs[msgs.length - 1] = { ...last, content }
          return {
            sessions: {
              ...state.sessions,
              [sessionId]: { ...s, messages: msgs, updatedAt: new Date().toISOString() },
            },
          }
        }),

      addArtifact: (sessionId, artifact) =>
        set((state) => {
          const s = state.sessions[sessionId]
          if (!s) return {}
          return {
            sessions: {
              ...state.sessions,
              [sessionId]: {
                ...s,
                artifacts: [...s.artifacts, artifact],
                updatedAt: new Date().toISOString(),
              },
            },
          }
        }),

      addStreamingSession: (sessionId) =>
        set((state) => ({
          streaming: { ...state.streaming, [sessionId]: true },
        })),

      removeStreamingSession: (sessionId) =>
        set((state) => {
          const { [sessionId]: _, ...rest } = state.streaming
          return { streaming: rest }
        }),

      // Plan actions
      addPlanSteps: (sessionId, steps) =>
        set((state) => {
          const existing = state.planSteps[sessionId] ?? []
          const existingIds = new Set(existing.map((s) => s.id))
          const merged = [...existing]
          for (const step of steps) {
            if (!existingIds.has(step.id)) {
              merged.push(step)
            }
          }
          return { planSteps: { ...state.planSteps, [sessionId]: merged } }
        }),

      updatePlanStep: (sessionId, stepId, status) =>
        set((state) => {
          const steps = state.planSteps[sessionId]
          if (!steps) return {}
          const now = Date.now()
          const updated = steps.map((s) => {
            if (s.id !== stepId) return s
            const patch: Partial<PlanStep> = { status }
            if (status === 'running' && !s.startedAt) patch.startedAt = now
            if (status === 'completed' || status === 'failed') patch.completedAt = now
            return { ...s, ...patch }
          })
          return { planSteps: { ...state.planSteps, [sessionId]: updated } }
        }),

      clearPlanSteps: (sessionId) =>
        set((state) => {
          const { [sessionId]: _, ...rest } = state.planSteps
          return { planSteps: rest }
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
    },
  ),
)
