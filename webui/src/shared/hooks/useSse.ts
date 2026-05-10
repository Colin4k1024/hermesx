import { useCallback, useRef, useState } from 'react'
import { useAuthStore } from '@shared/stores/authStore'

interface SseOptions {
  sessionId?: string
  onToken: (token: string) => void
  onDone: (sessionId: string | null) => void
  onError: (msg: string) => void
}

export function useSse() {
  const [loading, setLoading] = useState(false)
  const abortRef = useRef<AbortController | null>(null)

  const stream = useCallback(async (model: string, messages: Array<{ role: string; content: string }>, opts: SseOptions) => {
    abortRef.current?.abort()
    abortRef.current = new AbortController()
    setLoading(true)

    const { userApiKey, userId, disconnectUser } = useAuthStore.getState()
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    if (userApiKey) headers['Authorization'] = `Bearer ${userApiKey}`
    if (userId) headers['X-Hermes-User-Id'] = userId
    if (opts.sessionId) headers['X-Hermes-Session-Id'] = opts.sessionId

    try {
      const res = await fetch('/v1/chat/completions', {
        method: 'POST',
        headers,
        body: JSON.stringify({ model, messages, stream: true }),
        signal: abortRef.current.signal,
      })

      if (res.status === 401 || res.status === 403) {
        disconnectUser()
        opts.onError('Session expired')
        return
      }
      if (!res.ok || !res.body) {
        opts.onError(`Chat failed (${res.status})`)
        return
      }

      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let buf = ''
      let newSessionId: string | null = null

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buf += decoder.decode(value, { stream: true })
        const lines = buf.split('\n')
        buf = lines.pop() ?? ''
        for (const line of lines) {
          if (!line.startsWith('data: ')) continue
          const raw = line.slice(6).trim()
          if (raw === '[DONE]') {
            await reader.cancel()
            opts.onDone(newSessionId)
            return
          }
          try {
            const chunk = JSON.parse(raw)
            if (chunk.id && !newSessionId) newSessionId = chunk.id
            const delta = chunk.choices?.[0]?.delta?.content
            if (delta) opts.onToken(delta)
            if (chunk.choices?.[0]?.finish_reason === 'stop') {
              await reader.cancel()
              opts.onDone(newSessionId)
              return
            }
          } catch {
            // skip malformed SSE lines
          }
        }
      }
      opts.onDone(newSessionId)
    } catch (e) {
      if (e instanceof DOMException && e.name === 'AbortError') return
      opts.onError(e instanceof Error ? e.message : 'Stream error')
    } finally {
      setLoading(false)
    }
  }, [])

  const abort = useCallback(() => {
    abortRef.current?.abort()
    setLoading(false)
  }, [])

  return { loading, stream, abort }
}
