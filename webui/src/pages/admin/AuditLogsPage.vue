<template>
  <div>
    <h2 style="color:#e6edf3;font-size:20px;font-weight:600;margin:0 0 20px">Audit Logs</h2>

    <!-- Filter row -->
    <div style="background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px;margin-bottom:20px">
      <n-space align="center">
        <n-input
          v-model:value="filterTenantId"
          placeholder="Filter by Tenant ID..."
          style="width:280px"
          clearable
        />
        <n-button type="primary" @click="doSearch">Search</n-button>
        <n-button @click="clearFilter">Clear</n-button>
      </n-space>
    </div>

    <!-- Loading -->
    <div v-if="loading" style="display:flex;justify-content:center;padding:60px">
      <n-spin size="large" />
    </div>

    <!-- Error -->
    <n-alert v-else-if="error" type="error" style="margin-bottom:16px">{{ error }}</n-alert>

    <!-- Empty -->
    <div v-else-if="logs.length === 0" style="display:flex;justify-content:center;padding:60px">
      <n-empty description="No audit logs found" />
    </div>

    <!-- Table + pagination -->
    <template v-else>
      <div style="background:#161b22;border-radius:8px;overflow:hidden;border:1px solid #30363d;margin-bottom:16px">
        <n-data-table
          :columns="columns"
          :data="logs"
          :bordered="false"
          :bottom-bordered="false"
          size="small"
          :row-props="rowProps"
        />
      </div>
      <div style="display:flex;justify-content:flex-end">
        <n-pagination
          v-model:page="currentPage"
          :page-count="totalPages"
          :page-size="pageSize"
          @update:page="onPageChange"
        />
      </div>
    </template>

    <!-- Detail modal -->
    <n-modal v-model:show="showDetail" preset="card" title="Audit Log Detail" style="width:600px;background:#161b22;border:1px solid #30363d">
      <pre v-if="selectedLog" style="color:#e6edf3;font-size:12px;font-family:monospace;white-space:pre-wrap;word-break:break-all;max-height:400px;overflow-y:auto;background:#0d1117;padding:12px;border-radius:6px;border:1px solid #30363d">{{ JSON.stringify(selectedLog, null, 2) }}</pre>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showDetail = false">Close</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, h, onMounted } from 'vue'
import {
  NDataTable, NButton, NInput, NPagination,
  NSpin, NAlert, NEmpty, NSpace, NModal,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useApi } from '@shared/composables/useApi'
import type { AuditLogItem, AuditLogListResponse } from '@shared/types/index'

const api = useApi()

const logs = ref<AuditLogItem[]>([])
const total = ref(0)
const loading = ref(false)
const error = ref('')
const filterTenantId = ref('')
const currentPage = ref(1)
const pageSize = 50

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

const showDetail = ref(false)
const selectedLog = ref<AuditLogItem | null>(null)

async function fetchLogs() {
  loading.value = true
  error.value = ''
  try {
    const params = new URLSearchParams()
    if (filterTenantId.value.trim()) params.set('tenant_id', filterTenantId.value.trim())
    params.set('page', String(currentPage.value))
    params.set('page_size', String(pageSize))
    const data = await api.get<AuditLogListResponse>(`/admin/v1/audit-logs?${params.toString()}`, { asAdmin: true })
    logs.value = data.logs ?? []
    total.value = data.total ?? 0
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

function doSearch() {
  currentPage.value = 1
  fetchLogs()
}

function clearFilter() {
  filterTenantId.value = ''
  currentPage.value = 1
  fetchLogs()
}

function onPageChange(page: number) {
  currentPage.value = page
  fetchLogs()
}

function rowProps(row: AuditLogItem) {
  return {
    style: 'cursor:pointer',
    onClick: () => {
      selectedLog.value = row
      showDetail.value = true
    },
  }
}

function truncate(val: string | null, len = 8): string {
  if (!val) return '-'
  return val.length > len ? val.slice(0, len) + '…' : val
}

const columns: DataTableColumns<AuditLogItem> = [
  {
    title: 'ID',
    key: 'id',
    width: 60,
    render: (row) => h('span', { style: 'color:#7d8590;font-size:12px' }, String(row.id)),
  },
  {
    title: 'Tenant ID',
    key: 'tenant_id',
    render: (row) => h('code', { style: 'color:#7d8590;font-size:12px;font-family:monospace' }, truncate(row.tenant_id)),
  },
  {
    title: 'User ID',
    key: 'user_id',
    render: (row) => h('code', { style: 'color:#7d8590;font-size:12px;font-family:monospace' }, truncate(row.user_id)),
  },
  {
    title: 'Action',
    key: 'action',
    render: (row) => h('span', { style: 'color:#e6edf3;font-size:13px' }, row.action),
  },
  {
    title: 'Status',
    key: 'status_code',
    width: 80,
    render: (row) => {
      const code = row.status_code
      const color = !code ? '#7d8590' : code < 300 ? '#3fb950' : code < 500 ? '#d29922' : '#f85149'
      return h('span', { style: `color:${color};font-size:13px;font-weight:500` }, code ? String(code) : '-')
    },
  },
  {
    title: 'Created At',
    key: 'created_at',
    render: (row) => h('span', { style: 'color:#7d8590;font-size:12px' }, row.created_at.slice(0, 19).replace('T', ' ')),
  },
]

onMounted(fetchLogs)
</script>
