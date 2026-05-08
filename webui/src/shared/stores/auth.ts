import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const userApiKey = ref('')
  const userId = ref('')
  const adminApiKey = ref('')
  const tenantId = ref('')
  const connected = ref(false)
  // Store verified roles separately; key presence alone is insufficient for isAdmin.
  const adminRoles = ref<string[]>([])

  const isAdmin = computed(() => adminRoles.value.includes('admin'))

  // Rehydrate non-sensitive fields only (keys are never written to sessionStorage).
  function rehydrateUser() {
    userId.value = sessionStorage.getItem('hx_user_id') ?? ''
    tenantId.value = sessionStorage.getItem('hx_tenant_id') ?? ''
    // If userId exists the user must re-enter their key — no key in storage.
    connected.value = false
  }

  function rehydrateAdmin() {
    tenantId.value = sessionStorage.getItem('hx_admin_tenant_id') ?? ''
    adminRoles.value = []
    connected.value = false
  }

  async function connectUser(key: string, uid: string) {
    const res = await fetch('/v1/me', {
      headers: { Authorization: `Bearer ${key}`, 'X-Hermes-User-Id': uid },
    })
    if (!res.ok) throw new Error(res.status === 401 ? 'Invalid API Key' : `Connection failed (${res.status})`)
    const data = await res.json() as { tenant_id?: string }
    userApiKey.value = key
    userId.value = uid
    tenantId.value = data.tenant_id ?? ''
    connected.value = true
    // Only persist non-sensitive metadata; the key stays in memory only.
    sessionStorage.setItem('hx_user_id', uid)
    sessionStorage.setItem('hx_tenant_id', tenantId.value)
  }

  async function connectAdmin(key: string) {
    const res = await fetch('/v1/me', { headers: { Authorization: `Bearer ${key}` } })
    if (!res.ok) throw new Error(res.status === 401 ? 'Invalid Admin Key' : `Connection failed (${res.status})`)
    const data = await res.json() as { tenant_id?: string; roles?: string[] }
    const roles: string[] = data.roles ?? []
    if (!roles.includes('admin')) throw new Error('This key does not have admin access')
    adminApiKey.value = key
    adminRoles.value = roles
    tenantId.value = data.tenant_id ?? ''
    connected.value = true
    // Only persist non-sensitive metadata; the key stays in memory only.
    sessionStorage.setItem('hx_admin_tenant_id', tenantId.value)
  }

  function disconnectUser() {
    userApiKey.value = ''
    userId.value = ''
    tenantId.value = ''
    connected.value = false
    ;['hx_user_id', 'hx_tenant_id'].forEach(k => sessionStorage.removeItem(k))
  }

  function disconnectAdmin() {
    adminApiKey.value = ''
    adminRoles.value = []
    tenantId.value = ''
    connected.value = false
    ;['hx_admin_tenant_id'].forEach(k => sessionStorage.removeItem(k))
  }

  return { userApiKey, userId, adminApiKey, tenantId, connected, isAdmin, rehydrateUser, rehydrateAdmin, connectUser, connectAdmin, disconnectUser, disconnectAdmin }
})
