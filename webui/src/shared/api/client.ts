import { useAuthStore } from '@shared/stores/authStore'

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

interface RequestOptions {
  asAdmin?: boolean
  sessionId?: string
  signal?: AbortSignal
}

async function request<T>(method: string, path: string, body?: unknown, opts?: RequestOptions): Promise<T> {
  const state = useAuthStore.getState()
  const key = opts?.asAdmin ? state.adminApiKey : state.userApiKey
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }

  if (key) headers['Authorization'] = `Bearer ${key}`
  if (!opts?.asAdmin && state.userId) headers['X-Hermes-User-Id'] = state.userId
  if (opts?.sessionId) headers['X-Hermes-Session-Id'] = opts.sessionId

  const res = await fetch(path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
    signal: opts?.signal,
  })

  if (res.status === 401 || res.status === 403) {
    if (opts?.asAdmin) {
      useAuthStore.getState().disconnectAdmin()
    } else {
      useAuthStore.getState().disconnectUser()
    }
    throw new ApiError(res.status, 'Unauthorized')
  }

  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new ApiError(res.status, text)
  }

  if (res.status === 204) return undefined as T
  return res.json()
}

export const apiClient = {
  get: <T>(path: string, opts?: RequestOptions) => request<T>('GET', path, undefined, opts),
  post: <T>(path: string, body?: unknown, opts?: RequestOptions) => request<T>('POST', path, body, opts),
  put: <T>(path: string, body?: unknown, opts?: RequestOptions) => request<T>('PUT', path, body, opts),
  del: <T>(path: string, opts?: RequestOptions) => request<T>('DELETE', path, undefined, opts),
}
