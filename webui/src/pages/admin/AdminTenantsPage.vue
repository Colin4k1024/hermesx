<template>
  <div style="padding: 24px">
    <n-page-header title="Tenants" subtitle="Manage tenants">
      <template #extra>
        <n-button type="primary" size="small" @click="showCreate = true">Create Tenant</n-button>
        <n-button size="small" style="margin-left: 8px" @click="load">Refresh</n-button>
      </template>
    </n-page-header>

    <n-divider />

    <n-spin v-if="loading" style="display: flex; justify-content: center; padding: 48px" />

    <n-alert v-else-if="error" type="error" :title="error">
      <n-button size="small" @click="load">Retry</n-button>
    </n-alert>

    <n-empty v-else-if="!tenants.length" description="No tenants yet" style="padding: 48px" />

    <n-data-table v-else :columns="columns" :data="tenants" striped />

    <!-- Create Tenant Modal -->
    <n-modal v-model:show="showCreate" preset="card" title="Create Tenant" style="width: 400px">
      <n-form @submit.prevent="handleCreate">
        <n-form-item label="Tenant Name">
          <n-input v-model:value="createName" placeholder="e.g. my-team" :disabled="creating" />
        </n-form-item>
        <n-alert v-if="createError" type="error" :title="createError" style="margin-bottom: 12px" />
        <n-space justify="end">
          <n-button @click="showCreate = false" :disabled="creating">Cancel</n-button>
          <n-button type="primary" :loading="creating" @click="handleCreate">Create</n-button>
        </n-space>
      </n-form>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  NPageHeader, NDivider, NSpin, NAlert, NEmpty, NDataTable,
  NModal, NForm, NFormItem, NInput, NButton, NSpace,
  type DataTableColumns,
} from 'naive-ui'
import { useApi } from '@/composables/useApi'
import type { TenantItem, TenantListResponse } from '@/types'
import { normalizeApiError } from '@/utils/errors'

const api = useApi()
const tenants = ref<TenantItem[]>([])
const loading = ref(false)
const error = ref<string | null>(null)

const showCreate = ref(false)
const createName = ref('')
const creating = ref(false)
const createError = ref<string | null>(null)

onMounted(load)

async function load() {
  loading.value = true
  error.value = null
  try {
    const data = await api.get<TenantListResponse>('/v1/tenants?limit=100', { asAdmin: true })
    tenants.value = data.tenants ?? []
  } catch (e) {
    error.value = normalizeApiError(e).message
  } finally {
    loading.value = false
  }
}

async function handleCreate() {
  if (!createName.value.trim()) return
  creating.value = true
  createError.value = null
  try {
    const created = await api.post<TenantItem>('/v1/tenants', { name: createName.value.trim() }, { asAdmin: true })
    tenants.value.push(created)
    showCreate.value = false
    createName.value = ''
  } catch (e) {
    createError.value = normalizeApiError(e).message
  } finally {
    creating.value = false
  }
}

const columns: DataTableColumns<TenantItem> = [
  { title: 'ID', key: 'id', width: 280 },
  { title: 'Name', key: 'name' },
  { title: 'Created', key: 'created_at', render: (row) => new Date(row.created_at).toLocaleString() },
]
</script>
