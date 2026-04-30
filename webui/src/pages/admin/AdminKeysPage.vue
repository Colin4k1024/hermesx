<template>
  <div style="padding: 24px">
    <n-page-header title="API Keys" subtitle="Manage tenant API keys">
      <template #extra>
        <n-button type="primary" size="small" @click="showCreate = true">Create API Key</n-button>
        <n-button size="small" style="margin-left: 8px" @click="load">Refresh</n-button>
      </template>
    </n-page-header>

    <n-divider />

    <n-spin v-if="loading" style="display: flex; justify-content: center; padding: 48px" />

    <n-alert v-else-if="error" type="error" :title="error">
      <n-button size="small" @click="load">Retry</n-button>
    </n-alert>

    <n-empty v-else-if="!keys.length" description="No API keys yet" style="padding: 48px" />

    <n-data-table v-else :columns="columns" :data="keys" striped />

    <!-- Create Key Modal -->
    <n-modal v-model:show="showCreate" preset="card" title="Create API Key" style="width: 440px">
      <n-form @submit.prevent="handleCreate">
        <n-form-item label="Tenant ID">
          <n-input v-model:value="form.tenantId" placeholder="UUID of tenant" :disabled="creating" />
        </n-form-item>
        <n-form-item label="Key Name">
          <n-input v-model:value="form.name" placeholder="e.g. production-key" :disabled="creating" />
        </n-form-item>
        <n-form-item label="Roles">
          <n-checkbox-group v-model:value="form.roles">
            <n-checkbox value="user" label="user" />
            <n-checkbox value="admin" label="admin" />
          </n-checkbox-group>
        </n-form-item>
        <n-alert v-if="createError" type="error" :title="createError" style="margin-bottom: 12px" />
        <n-space justify="end">
          <n-button @click="showCreate = false" :disabled="creating">Cancel</n-button>
          <n-button type="primary" :loading="creating" @click="handleCreate">Create</n-button>
        </n-space>
      </n-form>
    </n-modal>

    <!-- One-time Key Display Modal -->
    <n-modal v-model:show="showKeyResult" :closable="false" :mask-closable="false" preset="card" title="API Key Created" style="width: 480px">
      <n-alert type="warning" title="Copy this key now — it will not be shown again" />
      <n-input
        :value="createdKey"
        readonly
        style="margin-top: 16px; font-family: monospace"
        type="textarea"
        :autosize="{ minRows: 2 }"
      />
      <div style="margin-top: 16px">
        <n-checkbox v-model:checked="keyCopied">I have copied the key</n-checkbox>
      </div>
      <template #footer>
        <n-space justify="end">
          <n-button type="primary" :disabled="!keyCopied" @click="showKeyResult = false">Close</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, h, onMounted } from 'vue'
import {
  NPageHeader, NDivider, NSpin, NAlert, NEmpty, NDataTable,
  NModal, NForm, NFormItem, NInput, NButton, NSpace, NCheckbox, NCheckboxGroup,
  NPopconfirm,
  type DataTableColumns,
} from 'naive-ui'
import { useApi } from '@/composables/useApi'
import type { ApiKeyItem, ApiKeyListResponse, ApiKeyCreateResponse } from '@/types'
import { normalizeApiError } from '@/utils/errors'

const api = useApi()
const keys = ref<ApiKeyItem[]>([])
const loading = ref(false)
const error = ref<string | null>(null)

const showCreate = ref(false)
const form = reactive({ tenantId: '', name: '', roles: ['user'] as string[] })
const creating = ref(false)
const createError = ref<string | null>(null)

const showKeyResult = ref(false)
const createdKey = ref('')
const keyCopied = ref(false)

onMounted(load)

async function load() {
  loading.value = true
  error.value = null
  try {
    const data = await api.get<ApiKeyListResponse>('/v1/api-keys?limit=100', { asAdmin: true })
    keys.value = data.api_keys ?? []
  } catch (e) {
    error.value = normalizeApiError(e).message
  } finally {
    loading.value = false
  }
}

async function handleCreate() {
  if (!form.tenantId.trim() || !form.name.trim()) return
  creating.value = true
  createError.value = null
  try {
    const result = await api.post<ApiKeyCreateResponse>('/v1/api-keys', {
      tenant_id: form.tenantId.trim(),
      name: form.name.trim(),
      roles: form.roles,
    }, { asAdmin: true })

    createdKey.value = result.key
    keyCopied.value = false
    showCreate.value = false
    showKeyResult.value = true

    // Add to list without key (only prefix will be shown)
    keys.value.push({
      id: result.id,
      name: result.name,
      prefix: result.key.slice(0, 12) + '…',
      tenant_id: result.tenant_id,
      roles: result.roles,
      created_at: result.created_at,
    })
    form.tenantId = ''
    form.name = ''
    form.roles = ['user']
  } catch (e) {
    createError.value = normalizeApiError(e).message
  } finally {
    creating.value = false
  }
}

async function revokeKey(id: string) {
  try {
    await api.del(`/v1/api-keys/${id}`, { asAdmin: true })
    keys.value = keys.value.filter((k) => k.id !== id)
  } catch (e) {
    error.value = normalizeApiError(e).message
  }
}

const columns: DataTableColumns<ApiKeyItem> = [
  { title: 'Name', key: 'name' },
  { title: 'Prefix', key: 'prefix', width: 160 },
  { title: 'Tenant', key: 'tenant_id', ellipsis: true },
  { title: 'Roles', key: 'roles', render: (row) => row.roles.join(', ') },
  { title: 'Created', key: 'created_at', render: (row) => new Date(row.created_at).toLocaleString() },
  {
    title: 'Actions',
    key: 'actions',
    width: 100,
    render(row) {
      return h(
        NPopconfirm,
        { onPositiveClick: () => revokeKey(row.id) },
        {
          trigger: () => h(NButton, { size: 'small', type: 'error' }, { default: () => 'Revoke' }),
          default: () => 'Revoke this API key?',
        },
      )
    },
  },
]
</script>
