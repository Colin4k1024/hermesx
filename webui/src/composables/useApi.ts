import { useAuthStore } from '@/stores/auth'
import { ApiError } from '@/utils/errors'
import { useRouter } from 'vue-router'

export interface RequestOptions extends RequestInit {
  sessionId?: string
  asAdmin?: boolean
}

export function useApi() {
  const auth = useAuthStore()
  const router = useRouter()

  async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const { sessionId, asAdmin, ...init } = options

    const headers: Record<string, string> = {
      ...(init.headers as Record<string, string>),
    }

    headers['Authorization'] = asAdmin
      ? `Bearer ${auth.acpToken}`
      : `Bearer ${auth.apiKey}`
    headers['X-Hermes-User-Id'] = auth.userId

    if (sessionId) {
      headers['X-Hermes-Session-Id'] = sessionId
    }

    if (init.body && typeof init.body === 'string') {
      headers['Content-Type'] = 'application/json'
    }

    const res = await fetch(path, { ...init, headers })

    if (res.status === 401 || res.status === 403) {
      auth.disconnect()
      router.push('/connect')
      throw new ApiError('Session expired — please reconnect', res.status)
    }

    if (!res.ok) {
      // Guard against HTML 502 (Nginx timeout before LLM responds)
      const ct = res.headers.get('content-type') ?? ''
      if (ct.includes('text/html')) {
        throw new ApiError('Request timed out — please retry', res.status)
      }
      let msg = `Request failed (${res.status})`
      try {
        const body = await res.json()
        msg = body.error ?? body.message ?? msg
      } catch {
        // ignore parse error, use default msg
      }
      throw new ApiError(msg, res.status)
    }

    if (res.status === 204) return undefined as T

    // Guard: only parse JSON if content-type says so
    const ct = res.headers.get('content-type') ?? ''
    if (!ct.includes('application/json')) {
      return undefined as T
    }

    return res.json() as Promise<T>
  }

  function get<T>(path: string, options?: RequestOptions) {
    return request<T>(path, { ...options, method: 'GET' })
  }

  function post<T>(path: string, body: unknown, options?: RequestOptions) {
    return request<T>(path, {
      ...options,
      method: 'POST',
      body: body instanceof FormData ? body : JSON.stringify(body),
    })
  }

  function put<T>(path: string, body: unknown, options?: RequestOptions) {
    return request<T>(path, {
      ...options,
      method: 'PUT',
      body: body instanceof FormData ? body : JSON.stringify(body),
    })
  }

  function del(path: string, options?: RequestOptions) {
    return request<void>(path, { ...options, method: 'DELETE' })
  }

  return { request, get, post, put, del }
}
