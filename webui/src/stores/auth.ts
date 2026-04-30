import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const KEYS = {
  apiKey: 'hermes_api_key',
  userId: 'hermes_user_id',
  acpToken: 'hermes_acp_token',
  tenantId: 'hermes_tenant_id',
}

export const useAuthStore = defineStore('auth', () => {
  const apiKey = ref('')
  const userId = ref('')
  const acpToken = ref('')
  const tenantId = ref('')
  const connected = ref(false)

  const isAdmin = computed(() => acpToken.value.length > 0)

  function rehydrate() {
    apiKey.value = sessionStorage.getItem(KEYS.apiKey) ?? ''
    userId.value = sessionStorage.getItem(KEYS.userId) ?? ''
    acpToken.value = sessionStorage.getItem(KEYS.acpToken) ?? ''
    tenantId.value = sessionStorage.getItem(KEYS.tenantId) ?? ''
    connected.value = apiKey.value.length > 0 && userId.value.length > 0
  }

  async function connect(key: string, uid: string, acp = '') {
    const res = await fetch('/v1/me', {
      headers: {
        Authorization: `Bearer ${key}`,
        'X-Hermes-User-Id': uid,
      },
    })

    if (!res.ok) {
      const msg = res.status === 401 ? 'Invalid API Key' : `Connection failed (${res.status})`
      throw new Error(msg)
    }

    const data = await res.json()

    apiKey.value = key
    userId.value = uid
    acpToken.value = acp
    tenantId.value = data.tenant_id ?? ''
    connected.value = true

    sessionStorage.setItem(KEYS.apiKey, key)
    sessionStorage.setItem(KEYS.userId, uid)
    sessionStorage.setItem(KEYS.acpToken, acp)
    sessionStorage.setItem(KEYS.tenantId, tenantId.value)
  }

  function disconnect() {
    apiKey.value = ''
    userId.value = ''
    acpToken.value = ''
    tenantId.value = ''
    connected.value = false
    Object.values(KEYS).forEach((k) => sessionStorage.removeItem(k))
  }

  return { apiKey, userId, acpToken, tenantId, connected, isAdmin, rehydrate, connect, disconnect }
})
