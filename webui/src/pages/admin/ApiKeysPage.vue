<template>
  <div>
    <!-- Page header -->
    <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:20px">
      <h2 style="color:#e6edf3;font-size:20px;font-weight:600;margin:0">API Keys</h2>
      <n-button type="primary" :disabled="!selectedTenantId" @click="openCreate">New Key</n-button>
    </div>

    <!-- Tenant selector -->
    <div style="background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px;margin-bottom:20px">
      <div style="color:#7d8590;font-size:13px;margin-bottom:8px">Select Tenant</div>
      <n-select
        v-model:value="selectedTenantId"
        :options="tenantOptions"
        placeholder="Choose a tenant..."
        style="max-width:360px"
        @update:value="fetchKeys"
      />
    </div>

    <!-- One-time key reveal -->
    <n-alert v-if="newKeyValue" type="success" closable @close="newKeyValue = ''" style="margin-bottom:16px">
      <div style="margin-bottom:6px;font-weight:600">Key created — copy it now, it will not be shown again:</div>
      <div style="display:flex;align-items:center;gap:8px">
        <code style="font-family:monospace;font-size:13px;word-break:break-all;flex:1">{{ newKeyValue }}</code>
        <n-button size="small" @click="copyKey(newKeyValue)">Copy</n-button>
      </div>
    </n-alert>

    <!-- Loading -->
    <div v-if="loading" style="display:flex;justify-content:center;padding:60px">
      <n-spin size="large" />
    </div>

    <!-- Error -->
    <n-alert v-else-if="error" type="error" style="margin-bottom:16px">{{ error }}</n-alert>

    <!-- Placeholder when no tenant -->
    <div v-else-if="!selectedTenantId" style="display:flex;justify-content:center;padding:60px">
      <n-empty description="Select a tenant to view API keys" />
    </div>

    <!-- Empty -->
    <div v-else-if="keys.length === 0" style="display:flex;justify-content:center;padding:60px">
      <n-empty description="No API keys for this tenant" />
    </div>

    <!-- Table -->
    <div v-else style="background:#161b22;border-radius:8px;overflow:hidden;border:1px solid #30363d">
      <n-data-table
        :columns="columns"
        :data="keys"
        :bordered="false"
        :bottom-bordered="false"
        size="small"
      />
    </div>

    <!-- Create modal -->
    <n-modal v-model:show="showCreate" preset="card" title="New API Key" style="width:440px;background:#161b22;border:1px solid #30363d">
      <n-alert v-if="createError" type="error" style="margin-bottom:16px">{{ createError }}</n-alert>
      <n-form ref="createFormRef" :model="createForm" :rules="createRules">
        <n-form-item label="Name" path="name">
          <n-input v-model:value="createForm.name" placeholder="e.g. ci-token" />
        </n-form-item>
        <n-form-item label="Roles (comma-separated)" path="rolesRaw">
          <n-input v-model:value="createForm.rolesRaw" placeholder="user" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showCreate = false">Cancel</n-button>
          <n-button type="primary" :loading="creating" @click="submitCreate">Create</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, h, onMounted } from 'vue'
import {
  NDataTable, NButton, NModal, NForm, NFormItem, NInput,
  NSelect, NPopconfirm, NSpin, NAlert, NEmpty, NSpace,
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import { useApi } from '@shared/composables/useApi'
import type {
  TenantItem, TenantListResponse,
  ApiKeyItem, ApiKeyListResponse, ApiKeyCreateResponse,
} from '@shared/types/index'

const api = useApi()

// Tenants
const tenants = ref<TenantItem[]>([])
const selectedTenantId = ref<string | null>(null)
const tenantOptions = computed(() =>
  tenants.value.map(t => ({ label: t.name, value: t.id }))
)

// Keys
const keys = ref<ApiKeyItem[]>([])
const loading = ref(false)
const error = ref('')
const newKeyValue = ref('')

// Create modal
const showCreate = ref(false)
const creating = ref(false)
const createError = ref('')
const createFormRef = ref<FormInst | null>(null)
const createForm = ref({ name: '', rolesRaw: 'user' })
const createRules: FormRules = {
  name: [{ required: true, message: 'Name is required', trigger: 'blur' }],
}

async function fetchTenants() {
  try {
    const data = await api.get<TenantListResponse>('/v1/tenants', { asAdmin: true })
    tenants.value = data.tenants ?? []
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

async function fetchKeys(tenantId: string | null) {
  if (!tenantId) { keys.value = []; return }
  loading.value = true
  error.value = ''
  try {
    const data = await api.get<ApiKeyListResponse>(`/admin/v1/tenants/${tenantId}/api-keys`, { asAdmin: true })
    keys.value = data.api_keys ?? []
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

function openCreate() {
  createForm.value = { name: '', rolesRaw: 'user' }
  createError.value = ''
  showCreate.value = true
}

async function submitCreate() {
  try {
    await createFormRef.value?.validate()
  } catch {
    return
  }
  if (!selectedTenantId.value) return
  creating.value = true
  createError.value = ''
  try {
    const roles = createForm.value.rolesRaw.split(',').map(r => r.trim()).filter(Boolean)
    const data = await api.post<ApiKeyCreateResponse>(
      `/admin/v1/tenants/${selectedTenantId.value}/api-keys`,
      { name: createForm.value.name, roles },
      { asAdmin: true }
    )
    newKeyValue.value = data.key
    showCreate.value = false
    await fetchKeys(selectedTenantId.value)
  } catch (e: unknown) {
    createError.value = e instanceof Error ? e.message : String(e)
  } finally {
    creating.value = false
  }
}

async function rotateKey(keyId: string) {
  if (!selectedTenantId.value) return
  error.value = ''
  try {
    const data = await api.post<ApiKeyCreateResponse>(
      `/admin/v1/tenants/${selectedTenantId.value}/api-keys/${keyId}/rotate`,
      {},
      { asAdmin: true }
    )
    newKeyValue.value = data.key
    await fetchKeys(selectedTenantId.value)
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

async function revokeKey(keyId: string) {
  if (!selectedTenantId.value) return
  error.value = ''
  try {
    await api.del(`/admin/v1/tenants/${selectedTenantId.value}/api-keys/${keyId}`, { asAdmin: true })
    await fetchKeys(selectedTenantId.value)
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

function copyKey(val: string) {
  navigator.clipboard.writeText(val).catch(() => {})
}

const columns: DataTableColumns<ApiKeyItem> = [
  {
    title: 'Name',
    key: 'name',
    render: (row) => h('span', { style: 'color:#e6edf3' }, row.name),
  },
  {
    title: 'Prefix',
    key: 'prefix',
    render: (row) => h('code', { style: 'color:#7d8590;font-size:12px;font-family:monospace' }, row.prefix),
  },
  {
    title: 'Roles',
    key: 'roles',
    render: (row) => h('span', { style: 'color:#7d8590;font-size:12px' }, (row.roles ?? []).join(', ')),
  },
  {
    title: 'Revoked',
    key: 'revoked_at',
    render: (row) => h('span', {
      style: `font-size:12px;color:${row.revoked_at ? '#f85149' : '#3fb950'}`,
    }, row.revoked_at ? 'Yes' : 'No'),
  },
  {
    title: 'Created At',
    key: 'created_at',
    render: (row) => h('span', { style: 'color:#7d8590;font-size:13px' }, row.created_at.slice(0, 10)),
  },
  {
    title: 'Actions',
    key: 'actions',
    render: (row) =>
      h(NSpace, {}, {
        default: () => [
          h(NPopconfirm, { onPositiveClick: () => rotateKey(row.id) }, {
            trigger: () => h(NButton, { size: 'small', ghost: true }, { default: () => 'Rotate' }),
            default: () => 'Rotate this key? The old key will be invalidated.',
          }),
          h(NPopconfirm, { onPositiveClick: () => revokeKey(row.id) }, {
            trigger: () => h(NButton, { size: 'small', type: 'error', ghost: true }, { default: () => 'Revoke' }),
            default: () => 'Revoke this API key?',
          }),
        ],
      }),
  },
]

onMounted(async () => {
  await fetchTenants()
})
</script>
