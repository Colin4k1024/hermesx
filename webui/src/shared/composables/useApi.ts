import { useAuthStore } from '@shared/stores/auth'
import { ApiError } from '@shared/utils/errors'
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
    const headers: Record<string, string> = { ...(init.headers as Record<string, string>) }

    const key = asAdmin ? auth.adminApiKey : auth.userApiKey
    if (key) headers['Authorization'] = `Bearer ${key}`
    if (!asAdmin && auth.userId) headers['X-Hermes-User-Id'] = auth.userId
    if (sessionId) headers['X-Hermes-Session-Id'] = sessionId
    if (init.body && typeof init.body === 'string') headers['Content-Type'] = 'application/json'

    const res = await fetch(path, { ...init, headers })

    if (res.status === 401 || res.status === 403) {
      if (asAdmin) { auth.disconnectAdmin(); void router.push('/login') }
      else { auth.disconnectUser(); void router.push('/login') }
      throw new ApiError('Session expired — please reconnect', res.status)
    }

    if (!res.ok) {
      const ct = res.headers.get('content-type') ?? ''
      if (ct.includes('text/html')) throw new ApiError('Request timed out — please retry', res.status)
      let msg = `Request failed (${res.status})`
      try {
        const body = await res.json() as { error?: string; message?: string }
        msg = body.error ?? body.message ?? msg
      } catch { /* ignore */ }
      throw new ApiError(msg, res.status)
    }

    if (res.status === 204) return undefined as unknown as T
    return res.json() as Promise<T>
  }

  const get = <T>(path: string, opts?: Omit<RequestOptions, 'method'>) =>
    request<T>(path, { ...opts, method: 'GET' })
  const post = <T>(path: string, body: unknown, opts?: Omit<RequestOptions, 'method' | 'body'>) =>
    request<T>(path, { ...opts, method: 'POST', body: JSON.stringify(body) })
  const put = <T>(path: string, body: unknown, opts?: Omit<RequestOptions, 'method' | 'body'>) =>
    request<T>(path, { ...opts, method: 'PUT', body: JSON.stringify(body) })
  const del = <T = void>(path: string, opts?: Omit<RequestOptions, 'method'>) =>
    request<T>(path, { ...opts, method: 'DELETE' })

  return { request, get, post, put, del }
}
