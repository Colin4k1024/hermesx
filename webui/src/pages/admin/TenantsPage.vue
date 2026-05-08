<template>
  <div>
    <!-- Page header -->
    <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:20px">
      <h2 style="color:#e6edf3;font-size:20px;font-weight:600;margin:0">Tenants</h2>
      <n-button type="primary" @click="openCreate">New Tenant</n-button>
    </div>

    <!-- Loading -->
    <div v-if="loading" style="display:flex;justify-content:center;padding:60px">
      <n-spin size="large" />
    </div>

    <!-- Error -->
    <n-alert v-else-if="error" type="error" style="margin-bottom:16px">{{ error }}</n-alert>

    <!-- Empty -->
    <div v-else-if="tenants.length === 0" style="display:flex;justify-content:center;padding:60px">
      <n-empty description="No tenants found" />
    </div>

    <!-- Table -->
    <div v-else style="background:#161b22;border-radius:8px;overflow:hidden;border:1px solid #30363d">
      <n-data-table
        :columns="columns"
        :data="tenants"
        :bordered="false"
        :bottom-bordered="false"
        size="small"
      />
    </div>

    <!-- Create modal -->
    <n-modal v-model:show="showCreate" preset="card" title="New Tenant" style="width:440px;background:#161b22;border:1px solid #30363d">
      <n-alert v-if="createError" type="error" style="margin-bottom:16px">{{ createError }}</n-alert>
      <n-form ref="createFormRef" :model="createForm" :rules="createRules">
        <n-form-item label="Name" path="name">
          <n-input v-model:value="createForm.name" placeholder="e.g. acme-corp" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showCreate = false">Cancel</n-button>
          <n-button type="primary" :loading="creating" @click="submitCreate">Create</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- New key reveal modal -->
    <n-modal v-model:show="showKeyReveal" preset="card" title="Tenant Created" style="width:480px;background:#161b22;border:1px solid #30363d">
      <n-alert type="success" style="margin-bottom:0">
        Tenant created successfully. No API key was returned at creation time — use the API Keys page to generate keys for this tenant.
      </n-alert>
      <template #footer>
        <n-space justify="end">
          <n-button type="primary" @click="showKeyReveal = false">Close</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import {
  NDataTable, NButton, NModal, NForm, NFormItem, NInput,
  NPopconfirm, NSpin, NAlert, NEmpty, NSpace,
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import { useApi } from '@shared/composables/useApi'
import type { TenantItem, TenantListResponse } from '@shared/types/index'

const api = useApi()

const tenants = ref<TenantItem[]>([])
const loading = ref(false)
const error = ref('')

const showCreate = ref(false)
const creating = ref(false)
const createError = ref('')
const showKeyReveal = ref(false)
const createFormRef = ref<FormInst | null>(null)
const createForm = ref({ name: '' })

const createRules: FormRules = {
  name: [{ required: true, message: 'Name is required', trigger: 'blur' }],
}

async function fetchTenants() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.get<TenantListResponse>('/v1/tenants', { asAdmin: true })
    tenants.value = data.tenants ?? []
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

function openCreate() {
  createForm.value = { name: '' }
  createError.value = ''
  showCreate.value = true
}

async function submitCreate() {
  try {
    await createFormRef.value?.validate()
  } catch {
    return
  }
  creating.value = true
  createError.value = ''
  try {
    await api.post('/v1/tenants', { name: createForm.value.name }, { asAdmin: true })
    showCreate.value = false
    showKeyReveal.value = true
    await fetchTenants()
  } catch (e: unknown) {
    createError.value = e instanceof Error ? e.message : String(e)
  } finally {
    creating.value = false
  }
}

async function deleteTenant(id: string) {
  try {
    await api.del(`/v1/tenants/${id}`, { asAdmin: true })
    await fetchTenants()
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

const columns: DataTableColumns<TenantItem> = [
  {
    title: 'Name',
    key: 'name',
    render: (row) => h('span', { style: 'color:#e6edf3' }, row.name),
  },
  {
    title: 'ID',
    key: 'id',
    render: (row) => h('span', { style: 'color:#7d8590;font-family:monospace;font-size:12px' }, row.id.slice(0, 8) + '...'),
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
      h(NPopconfirm, {
        onPositiveClick: () => deleteTenant(row.id),
      }, {
        trigger: () => h(NButton, { size: 'small', type: 'error', ghost: true }, { default: () => 'Delete' }),
        default: () => 'Delete this tenant?',
      }),
  },
]

onMounted(fetchTenants)
</script>
