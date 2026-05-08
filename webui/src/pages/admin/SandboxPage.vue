<template>
  <div>
    <h2 style="color:#e6edf3;font-size:20px;font-weight:600;margin:0 0 20px">Sandbox Policy</h2>

    <!-- Tenant selector -->
    <div style="background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px;margin-bottom:20px">
      <div style="color:#7d8590;font-size:13px;margin-bottom:8px">Select Tenant</div>
      <n-select
        v-model:value="selectedTenantId"
        :options="tenantOptions"
        placeholder="Choose a tenant..."
        style="max-width:360px"
        @update:value="onTenantChange"
      />
    </div>

    <!-- No tenant selected -->
    <div v-if="!selectedTenantId" style="display:flex;justify-content:center;padding:60px">
      <n-empty description="Select a tenant to view sandbox policy" />
    </div>

    <!-- Loading -->
    <div v-else-if="loading" style="display:flex;justify-content:center;padding:60px">
      <n-spin size="large" />
    </div>

    <!-- Error (non-404) -->
    <n-alert v-else-if="error" type="error" style="margin-bottom:16px">{{ error }}</n-alert>

    <!-- Policy editor -->
    <template v-else>
      <n-alert v-if="saveSuccess" type="success" closable @close="saveSuccess = false" style="margin-bottom:16px">
        Policy saved successfully.
      </n-alert>

      <div style="background:#161b22;border:1px solid #30363d;border-radius:8px;padding:20px">
        <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:12px">
          <div>
            <div style="color:#e6edf3;font-size:14px;font-weight:500">Policy JSON</div>
            <div style="color:#7d8590;font-size:12px;margin-top:2px">
              {{ noPolicySet ? 'No policy set — enter JSON to create one' : `Last updated: ${lastUpdated}` }}
            </div>
          </div>
          <n-space>
            <n-popconfirm v-if="!noPolicySet" @positive-click="clearPolicy">
              <template #trigger>
                <n-button type="error" ghost>Clear Policy</n-button>
              </template>
              Delete the sandbox policy for this tenant?
            </n-popconfirm>
            <n-button type="primary" :loading="saving" @click="savePolicy">Save Policy</n-button>
          </n-space>
        </div>

        <n-input
          v-model:value="policyText"
          type="textarea"
          :rows="16"
          placeholder='{"allow":[],"deny":[]}'
          style="font-family:monospace;font-size:13px"
        />

        <n-alert v-if="saveError" type="error" style="margin-top:12px">{{ saveError }}</n-alert>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  NSelect, NInput, NButton, NPopconfirm,
  NSpin, NAlert, NEmpty, NSpace,
} from 'naive-ui'
import { useApi } from '@shared/composables/useApi'
import type { TenantItem, TenantListResponse, SandboxPolicy } from '@shared/types/index'

const api = useApi()

const tenants = ref<TenantItem[]>([])
const selectedTenantId = ref<string | null>(null)
const tenantOptions = computed(() =>
  tenants.value.map(t => ({ label: t.name, value: t.id }))
)

const loading = ref(false)
const error = ref('')
const policyText = ref('')
const noPolicySet = ref(false)
const lastUpdated = ref('')

const saving = ref(false)
const saveError = ref('')
const saveSuccess = ref(false)

async function fetchTenants() {
  try {
    const data = await api.get<TenantListResponse>('/v1/tenants', { asAdmin: true })
    tenants.value = data.tenants ?? []
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

async function fetchPolicy(tenantId: string) {
  loading.value = true
  error.value = ''
  noPolicySet.value = false
  policyText.value = ''
  lastUpdated.value = ''
  try {
    const data = await api.get<SandboxPolicy>(`/admin/v1/tenants/${tenantId}/sandbox-policy`, { asAdmin: true })
    policyText.value = typeof data.policy === 'string'
      ? data.policy
      : JSON.stringify(data.policy, null, 2)
    lastUpdated.value = data.updated_at ? data.updated_at.slice(0, 10) : ''
    noPolicySet.value = false
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    if (msg.includes('404') || msg.toLowerCase().includes('not found')) {
      noPolicySet.value = true
      policyText.value = ''
    } else {
      error.value = msg
    }
  } finally {
    loading.value = false
  }
}

function onTenantChange(id: string | null) {
  if (id) fetchPolicy(id)
  else {
    policyText.value = ''
    noPolicySet.value = false
    error.value = ''
  }
}

async function savePolicy() {
  if (!selectedTenantId.value) return
  saving.value = true
  saveError.value = ''
  saveSuccess.value = false
  try {
    await api.post(
      `/admin/v1/tenants/${selectedTenantId.value}/sandbox-policy`,
      { policy: policyText.value },
      { asAdmin: true }
    )
    saveSuccess.value = true
    await fetchPolicy(selectedTenantId.value)
  } catch (e: unknown) {
    saveError.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

async function clearPolicy() {
  if (!selectedTenantId.value) return
  saveError.value = ''
  saveSuccess.value = false
  try {
    await api.del(`/admin/v1/tenants/${selectedTenantId.value}/sandbox-policy`, { asAdmin: true })
    noPolicySet.value = true
    policyText.value = ''
    lastUpdated.value = ''
  } catch (e: unknown) {
    saveError.value = e instanceof Error ? e.message : String(e)
  }
}

onMounted(fetchTenants)
</script>
