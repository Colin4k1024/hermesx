import { create } from 'zustand'

interface AuthState {
  userApiKey: string
  userId: string
  adminApiKey: string
  tenantId: string
  roles: string[]
  connected: boolean

  connectUser: (key: string, userId: string) => Promise<boolean>
  connectAdmin: (key: string) => Promise<boolean>
  loginWithPassword: (username: string, password: string, tenantId: string) => Promise<boolean>
  register: (username: string, password: string, displayName: string, tenantId?: string, newTenantName?: string) => Promise<boolean>
  disconnectUser: () => void
  disconnectAdmin: () => void
  isAdmin: () => boolean
}

export const useAuthStore = create<AuthState>((set, get) => ({
  userApiKey: '',
  userId: '',
  adminApiKey: '',
  tenantId: '',
  roles: [],
  connected: false,

  connectUser: async (key, userId) => {
    try {
      const res = await fetch('/v1/me', {
        headers: { Authorization: `Bearer ${key}`, 'X-Hermes-User-Id': userId },
      })
      if (!res.ok) return false
      const data = await res.json()
      const identity = data.identity ?? userId
      set({
        userApiKey: key,
        userId: identity,
        tenantId: data.tenant_id ?? '',
        roles: data.roles ?? [],
        connected: true,
      })
      sessionStorage.setItem('hx_user_id', identity)
      sessionStorage.setItem('hx_tenant_id', data.tenant_id ?? '')
      return true
    } catch {
      return false
    }
  },

  connectAdmin: async (key) => {
    try {
      const res = await fetch('/v1/me', {
        headers: { Authorization: `Bearer ${key}` },
      })
      if (!res.ok) return false
      const data = await res.json()
      if (!data.roles?.includes('admin')) return false
      set({
        adminApiKey: key,
        tenantId: data.tenant_id ?? '',
        roles: data.roles ?? [],
        connected: true,
      })
      sessionStorage.setItem('hx_admin_tenant_id', data.tenant_id ?? '')
      return true
    } catch {
      return false
    }
  },

  loginWithPassword: async (username, password, tenantId) => {
    try {
      const res = await fetch('/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ username, password, tenant_id: tenantId }),
      })
      if (!res.ok) return false
      const data = await res.json()
      // After successful login, verify session with /v1/me using cookie auth.
      const meRes = await fetch('/v1/me', { credentials: 'include' })
      if (!meRes.ok) return false
      const meData = await meRes.json()
      set({
        userId: data.user_id ?? username,
        tenantId: data.tenant_id ?? tenantId,
        roles: meData.roles ?? ['user'],
        connected: true,
      })
      sessionStorage.setItem('hx_user_id', data.user_id ?? username)
      sessionStorage.setItem('hx_tenant_id', data.tenant_id ?? tenantId)
      return true
    } catch {
      return false
    }
  },

  register: async (username, password, displayName, tenantId, newTenantName) => {
    try {
      const res = await fetch('/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          username,
          password,
          display_name: displayName,
          ...(tenantId ? { tenant_id: tenantId } : {}),
          ...(newTenantName ? { new_tenant_name: newTenantName } : {}),
        }),
      })
      return res.ok
    } catch {
      return false
    }
  },

  disconnectUser: () => {
    set({ userApiKey: '', userId: '', tenantId: '', roles: [], connected: false })
    sessionStorage.removeItem('hx_user_id')
    sessionStorage.removeItem('hx_tenant_id')
  },

  disconnectAdmin: () => {
    set({ adminApiKey: '', tenantId: '', roles: [], connected: false })
    sessionStorage.removeItem('hx_admin_tenant_id')
  },

  isAdmin: () => get().roles.includes('admin'),
}))
