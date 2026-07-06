import { useCallback, useEffect, useRef, useState } from 'react'
import { useAuthStore } from '@shared/stores/authStore'
import { useWorkspaceStore } from '@shared/stores/workspaceStore'

/* ------------------------------------------------------------------ */
/*  Types                                                             */
/* ------------------------------------------------------------------ */

export type StreamStatus = 'idle' | 'streaming' | 'error' | 'completed'

export interface StreamState {
  status: StreamStatus
  buffer: string
  error: string | null
}

interface SSEController {
  abortController: AbortController
  buffer: string
  status: StreamStatus
  error: string | null
}

interface StartStreamOpts {
  sessionId: string
  message: string
  model?: string
  onToolResult?: (data: unknown) => void
  onPlanStart?: (data: { steps: Array<{ id: string; title: string }> }) => void
  onPlanStepUpdate?: (data: { step_id: string; status: string }) => void
}

/* ------------------------------------------------------------------ */
/*  Hook                                                              */
/* ------------------------------------------------------------------ */

/**
 * Manages multiple concurrent SSE streams, one per session.
 * Each session gets its own AbortController, event buffer, and status.
 * Switching active sessions does not kill background streams.
 */
export function useSseManager() {
  const streamsRef = useRef<Map<string, SSEController>>(new Map())
  // Counter state used solely to trigger re-renders when stream states change.
  const [, setRenderTick] = useState(0)

  const notifyUpdate = useCallback(() => {
    setRenderTick((n) => n + 1)
  }, [])

  // Cleanup all streams on unmount
  useEffect(() => {
    return () => {
      streamsRef.current.forEach((ctrl) => ctrl.abortController.abort())
      streamsRef.current.clear()
    }
  }, [])

  /**
   * Start a new SSE stream for the given session.
   * If a stream is already running for that session, it is aborted first.
   */
  const startStream = useCallback(
    async ({ sessionId, message, model = 'default', onToolResult, onPlanStart, onPlanStepUpdate }: StartStreamOpts) => {
      const store = useWorkspaceStore.getState()

      // Abort any existing stream for this session
      const existing = streamsRef.current.get(sessionId)
      if (existing) {
        existing.abortController.abort()
        streamsRef.current.delete(sessionId)
      }

      // Create new controller
      const controller: SSEController = {
        abortController: new AbortController(),
        buffer: '',
        status: 'streaming',
        error: null,
      }
      streamsRef.current.set(sessionId, controller)

      // Update store: mark session as streaming
      store.addStreamingSession(sessionId)
      store.updateSessionStatus(sessionId, 'running')
      notifyUpdate()

      // Prepare messages from session history
      const session = store.sessions.get(sessionId)
      const historyMessages = session?.messages ?? []
      const apiMessages = historyMessages
        .filter((m) => m.content) // skip empty assistant placeholders
        .map(({ role, content }) => ({ role, content }))

      // Ensure we have the latest user message
      const lastMsg = apiMessages[apiMessages.length - 1]
      if (!apiMessages.length || lastMsg?.content !== message) {
        apiMessages.push({ role: 'user', content: message })
      }

      // Prepare request
      const { userApiKey, userId, disconnectUser } = useAuthStore.getState()
      const headers: Record<string, string> = { 'Content-Type': 'application/json' }
      if (userApiKey) headers['Authorization'] = `Bearer ${userApiKey}`
      if (userId) headers['X-Hermes-User-Id'] = userId
      headers['X-Hermes-Session-Id'] = sessionId

      try {
        const res = await fetch('/v1/chat/completions', {
          method: 'POST',
          headers,
          body: JSON.stringify({ model, messages: apiMessages, stream: true }),
          signal: controller.abortController.signal,
        })

        if (res.status === 401 || res.status === 403) {
          disconnectUser()
          controller.status = 'error'
          controller.error = '会话已过期，请重新登录'
          store.removeStreamingSession(sessionId)
          store.updateSessionStatus(sessionId, 'failed')
          notifyUpdate()
          return
        }

        if (res.status === 429) {
          controller.status = 'error'
          controller.error = '并发流数量已达上限，请稍后重试'
          store.removeStreamingSession(sessionId)
          store.updateSessionStatus(sessionId, 'failed')
          notifyUpdate()
          return
        }

        if (!res.ok || !res.body) {
          controller.status = 'error'
          controller.error = `请求失败 (${res.status})`
          store.removeStreamingSession(sessionId)
          store.updateSessionStatus(sessionId, 'failed')
          notifyUpdate()
          return
        }

        const reader = res.body.getReader()
        const decoder = new TextDecoder()
        let buf = ''
        let lastEventType = ''

        while (true) {
          const { done, value } = await reader.read()
          if (done) break
          buf += decoder.decode(value, { stream: true })
          const lines = buf.split('\n')
          buf = lines.pop() ?? ''
          for (const line of lines) {
            // Track SSE event types
            if (line.startsWith('event: ')) {
              lastEventType = line.slice(7).trim()
              continue
            }
            if (!line.startsWith('data: ')) continue
            const raw = line.slice(6).trim()
            if (raw === '[DONE]') {
              await reader.cancel()
              controller.status = 'completed'
              store.removeStreamingSession(sessionId)
              store.updateSessionStatus(sessionId, 'completed')
              notifyUpdate()
              return
            }
            try {
              const chunk = JSON.parse(raw)

              // Handle tool_result events
              if (lastEventType === 'tool_result') {
                onToolResult?.(chunk)
                lastEventType = ''
                continue
              }
              // Handle plan_start events
              if (lastEventType === 'plan_start') {
                onPlanStart?.(chunk)
                lastEventType = ''
                continue
              }
              // Handle plan_step_update events
              if (lastEventType === 'plan_step_update') {
                onPlanStepUpdate?.(chunk)
                lastEventType = ''
                continue
              }
              lastEventType = ''

              const delta = chunk.choices?.[0]?.delta?.content
              if (delta) {
                controller.buffer += delta
                // Update the store's last message for this session
                store.updateLastMessage(sessionId, controller.buffer)
                notifyUpdate()
              }
              if (chunk.choices?.[0]?.finish_reason === 'stop') {
                await reader.cancel()
                controller.status = 'completed'
                store.removeStreamingSession(sessionId)
                store.updateSessionStatus(sessionId, 'completed')
                notifyUpdate()
                return
              }
            } catch {
              // skip malformed SSE lines
            }
          }
        }
        // Stream ended without explicit [DONE]
        controller.status = 'completed'
        store.removeStreamingSession(sessionId)
        store.updateSessionStatus(sessionId, 'completed')
        notifyUpdate()
      } catch (e) {
        if (e instanceof DOMException && e.name === 'AbortError') {
          // Intentional abort - clean up silently
          store.removeStreamingSession(sessionId)
          streamsRef.current.delete(sessionId)
          notifyUpdate()
          return
        }
        controller.status = 'error'
        controller.error = e instanceof Error ? e.message : '网络错误，请重试'
        store.removeStreamingSession(sessionId)
        store.updateSessionStatus(sessionId, 'failed')
        notifyUpdate()
      }
    },
    [notifyUpdate],
  )

  /**
   * Abort a specific session's stream.
   */
  const stopStream = useCallback(
    (sessionId: string) => {
      const controller = streamsRef.current.get(sessionId)
      if (controller) {
        controller.abortController.abort()
        streamsRef.current.delete(sessionId)
      }
      const store = useWorkspaceStore.getState()
      store.removeStreamingSession(sessionId)
      notifyUpdate()
    },
    [notifyUpdate],
  )

  /**
   * Get the current streaming state for a session.
   */
  const getStreamState = useCallback(
    (sessionId: string): StreamState => {
      const controller = streamsRef.current.get(sessionId)
      if (!controller) return { status: 'idle', buffer: '', error: null }
      return {
        status: controller.status,
        buffer: controller.buffer,
        error: controller.error,
      }
    },
    [],
  )

  /**
   * Check if a session is currently streaming.
   */
  const isStreaming = useCallback(
    (sessionId: string): boolean => {
      const controller = streamsRef.current.get(sessionId)
      return controller?.status === 'streaming'
    },
    [],
  )

  return {
    startStream,
    stopStream,
    getStreamState,
    isStreaming,
  }
}
