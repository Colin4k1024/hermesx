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
  disconnectUser: () => void
  disconnectAdmin: () => void
  rehydrateUser: () => void
  rehydrateAdmin: () => void
  isAdmin: () => boolean
}

// Restore admin or user auth state eagerly at store creation time so that
// AuthGuard sees connected=true on the very first synchronous render after
// a full-page reload (e.g. when the browser navigates directly to a deep URL).
function getInitialState() {
  const adminKey = sessionStorage.getItem('hx_admin_key')
  const adminTenantId = sessionStorage.getItem('hx_admin_tenant_id')
  if (adminKey && adminTenantId) {
    return {
      adminApiKey: adminKey,
      tenantId: adminTenantId,
      roles: ['admin'],
      connected: true,
    }
  }
  const userKey = sessionStorage.getItem('hx_user_key')
  const userId = sessionStorage.getItem('hx_user_id')
  const userTenantId = sessionStorage.getItem('hx_tenant_id')
  if (userKey && userId) {
    return {
      userApiKey: userKey,
      userId,
      tenantId: userTenantId ?? '',
      roles: ['user'],
      connected: true,
    }
  }
  return {}
}

export const useAuthStore = create<AuthState>((set, get) => ({
  userApiKey: '',
  userId: '',
  adminApiKey: '',
  tenantId: '',
  roles: [],
  connected: false,
  // Override defaults with anything persisted in sessionStorage.
  ...getInitialState(),

  connectUser: async (key, userId) => {
    try {
      const res = await fetch('/v1/me', {
        headers: { Authorization: `Bearer ${key}`, 'X-Hermes-User-Id': userId },
      })
      if (!res.ok) return false
      const data = await res.json()
      set({
        userApiKey: key,
        userId,
        tenantId: data.tenant_id ?? '',
        roles: data.roles ?? [],
        connected: true,
      })
      sessionStorage.setItem('hx_user_key', key)
      sessionStorage.setItem('hx_user_id', userId)
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
      sessionStorage.setItem('hx_admin_key', key)
      sessionStorage.setItem('hx_admin_tenant_id', data.tenant_id ?? '')
      return true
    } catch {
      return false
    }
  },

  disconnectUser: () => {
    set({ userApiKey: '', userId: '', tenantId: '', roles: [], connected: false })
    sessionStorage.removeItem('hx_user_key')
    sessionStorage.removeItem('hx_user_id')
    sessionStorage.removeItem('hx_tenant_id')
  },

  disconnectAdmin: () => {
    set({ adminApiKey: '', tenantId: '', roles: [], connected: false })
    sessionStorage.removeItem('hx_admin_key')
    sessionStorage.removeItem('hx_admin_tenant_id')
  },

  rehydrateUser: () => {
    const userKey = sessionStorage.getItem('hx_user_key')
    const userId = sessionStorage.getItem('hx_user_id')
    const tenantId = sessionStorage.getItem('hx_tenant_id')
    if (userKey && userId) set({ userApiKey: userKey, userId, tenantId: tenantId ?? '', connected: true, roles: ['user'] })
  },

  rehydrateAdmin: () => {
    const adminKey = sessionStorage.getItem('hx_admin_key')
    const tenantId = sessionStorage.getItem('hx_admin_tenant_id')
    if (adminKey && tenantId) set({ adminApiKey: adminKey, tenantId, connected: true, roles: ['admin'] })
  },

  isAdmin: () => get().roles.includes('admin'),
}))
