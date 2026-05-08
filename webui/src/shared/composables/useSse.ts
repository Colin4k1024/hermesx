import { ref } from 'vue'
import { useAuthStore } from '@shared/stores/auth'

export interface SseOptions {
  sessionId?: string
  onToken: (token: string) => void
  onDone: (sessionId: string | null) => void
  onError: (err: string) => void
}

export function useSse() {
  const auth = useAuthStore()
  const loading = ref(false)
  let abortCtrl: AbortController | null = null

  async function stream(model: string, messages: { role: string; content: string }[], opts: SseOptions) {
    if (abortCtrl) abortCtrl.abort()
    abortCtrl = new AbortController()
    loading.value = true

    try {
      const res = await fetch('/v1/chat/completions', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${auth.userApiKey}`,
          'X-Hermes-User-Id': auth.userId,
          ...(opts.sessionId ? { 'X-Hermes-Session-Id': opts.sessionId } : {}),
        },
        body: JSON.stringify({ model, messages, stream: true }),
        signal: abortCtrl.signal,
      })

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
          if (raw === '[DONE]') { opts.onDone(newSessionId); return }

          try {
            const chunk = JSON.parse(raw) as {
              id?: string
              choices?: Array<{ delta?: { content?: string }; finish_reason?: string }>
            }
            if (chunk.id) newSessionId = chunk.id
            const delta = chunk.choices?.[0]?.delta?.content
            if (delta) opts.onToken(delta)
            if (chunk.choices?.[0]?.finish_reason === 'stop') { opts.onDone(newSessionId); return }
          } catch { /* malformed chunk, skip */ }
        }
      }
      opts.onDone(newSessionId)
    } catch (e) {
      if (e instanceof DOMException && e.name === 'AbortError') return
      opts.onError(e instanceof Error ? e.message : 'Stream error')
    } finally {
      loading.value = false
    }
  }

  function abort() { abortCtrl?.abort(); loading.value = false }

  return { loading, stream, abort }
}
