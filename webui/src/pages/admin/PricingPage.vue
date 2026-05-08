<template>
  <div>
    <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:20px">
      <h2 style="color:#e6edf3;font-size:20px;font-weight:600;margin:0">Pricing Rules</h2>
      <n-button type="primary" @click="openAdd">Add Rule</n-button>
    </div>

    <!-- Loading -->
    <div v-if="loading" style="display:flex;justify-content:center;padding:60px">
      <n-spin size="large" />
    </div>

    <!-- Error -->
    <n-alert v-else-if="error" type="error" style="margin-bottom:16px">{{ error }}</n-alert>

    <!-- Empty -->
    <div v-else-if="rules.length === 0" style="display:flex;justify-content:center;padding:60px">
      <n-empty description="No pricing rules configured" />
    </div>

    <!-- Table -->
    <div v-else style="background:#161b22;border-radius:8px;overflow:hidden;border:1px solid #30363d">
      <n-data-table
        :columns="columns"
        :data="rules"
        :bordered="false"
        :bottom-bordered="false"
        size="small"
      />
    </div>

    <!-- Upsert modal -->
    <n-modal v-model:show="showUpsert" preset="card" :title="editingKey ? 'Edit Rule' : 'Add Rule'" style="width:480px;background:#161b22;border:1px solid #30363d">
      <n-alert v-if="upsertError" type="error" style="margin-bottom:16px">{{ upsertError }}</n-alert>
      <n-form ref="upsertFormRef" :model="upsertForm" :rules="upsertRules">
        <n-form-item label="Model Key" path="model_key">
          <n-input v-model:value="upsertForm.model_key" :disabled="!!editingKey" placeholder="e.g. claude-3-5-sonnet" />
        </n-form-item>
        <n-form-item label="Input per 1K tokens ($)" path="input_per_1k">
          <n-input-number v-model:value="upsertForm.input_per_1k" :min="0" :step="0.0001" :precision="6" style="width:100%" />
        </n-form-item>
        <n-form-item label="Output per 1K tokens ($)" path="output_per_1k">
          <n-input-number v-model:value="upsertForm.output_per_1k" :min="0" :step="0.0001" :precision="6" style="width:100%" />
        </n-form-item>
        <n-form-item label="Cache Read per 1K tokens ($)" path="cache_read_per_1k">
          <n-input-number v-model:value="upsertForm.cache_read_per_1k" :min="0" :step="0.0001" :precision="6" style="width:100%" />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showUpsert = false">Cancel</n-button>
          <n-button type="primary" :loading="upserting" @click="submitUpsert">Save</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import {
  NDataTable, NButton, NModal, NForm, NFormItem, NInput, NInputNumber,
  NPopconfirm, NSpin, NAlert, NEmpty, NSpace,
} from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'
import { useApi } from '@shared/composables/useApi'
import type { PricingRule, PricingRuleListResponse } from '@shared/types/index'

const api = useApi()

const rules = ref<PricingRule[]>([])
const loading = ref(false)
const error = ref('')

const showUpsert = ref(false)
const upserting = ref(false)
const upsertError = ref('')
const editingKey = ref('')
const upsertFormRef = ref<FormInst | null>(null)
const upsertForm = ref({
  model_key: '',
  input_per_1k: 0,
  output_per_1k: 0,
  cache_read_per_1k: 0,
})

const upsertRules: FormRules = {
  model_key: [{ required: true, message: 'Model key is required', trigger: 'blur' }],
}

async function fetchRules() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.get<PricingRuleListResponse>('/admin/v1/pricing-rules', { asAdmin: true })
    rules.value = data.rules ?? []
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

function openAdd() {
  editingKey.value = ''
  upsertForm.value = { model_key: '', input_per_1k: 0, output_per_1k: 0, cache_read_per_1k: 0 }
  upsertError.value = ''
  showUpsert.value = true
}

function openEdit(rule: PricingRule) {
  editingKey.value = rule.model_key
  upsertForm.value = {
    model_key: rule.model_key,
    input_per_1k: rule.input_per_1k,
    output_per_1k: rule.output_per_1k,
    cache_read_per_1k: rule.cache_read_per_1k,
  }
  upsertError.value = ''
  showUpsert.value = true
}

async function submitUpsert() {
  try {
    await upsertFormRef.value?.validate()
  } catch {
    return
  }
  upserting.value = true
  upsertError.value = ''
  const key = editingKey.value || upsertForm.value.model_key
  try {
    await api.put(
      `/admin/v1/pricing-rules/${encodeURIComponent(key)}`,
      {
        input_per_1k: upsertForm.value.input_per_1k,
        output_per_1k: upsertForm.value.output_per_1k,
        cache_read_per_1k: upsertForm.value.cache_read_per_1k,
      },
      { asAdmin: true }
    )
    showUpsert.value = false
    await fetchRules()
  } catch (e: unknown) {
    upsertError.value = e instanceof Error ? e.message : String(e)
  } finally {
    upserting.value = false
  }
}

async function deleteRule(modelKey: string) {
  error.value = ''
  try {
    await api.del(`/admin/v1/pricing-rules/${encodeURIComponent(modelKey)}`, { asAdmin: true })
    await fetchRules()
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

const columns: DataTableColumns<PricingRule> = [
  {
    title: 'Model Key',
    key: 'model_key',
    render: (row) => h('code', { style: 'color:#e6edf3;font-size:13px;font-family:monospace' }, row.model_key),
  },
  {
    title: 'Input / 1K',
    key: 'input_per_1k',
    render: (row) => h('span', { style: 'color:#7d8590;font-size:13px' }, `$${row.input_per_1k}`),
  },
  {
    title: 'Output / 1K',
    key: 'output_per_1k',
    render: (row) => h('span', { style: 'color:#7d8590;font-size:13px' }, `$${row.output_per_1k}`),
  },
  {
    title: 'Cache Read / 1K',
    key: 'cache_read_per_1k',
    render: (row) => h('span', { style: 'color:#7d8590;font-size:13px' }, `$${row.cache_read_per_1k}`),
  },
  {
    title: 'Updated At',
    key: 'updated_at',
    render: (row) => h('span', { style: 'color:#7d8590;font-size:12px' }, row.updated_at.slice(0, 10)),
  },
  {
    title: 'Actions',
    key: 'actions',
    render: (row) =>
      h(NSpace, {}, {
        default: () => [
          h(NButton, { size: 'small', ghost: true, onClick: () => openEdit(row) }, { default: () => 'Edit' }),
          h(NPopconfirm, { onPositiveClick: () => deleteRule(row.model_key) }, {
            trigger: () => h(NButton, { size: 'small', type: 'error', ghost: true }, { default: () => 'Delete' }),
            default: () => `Delete rule for ${row.model_key}?`,
          }),
        ],
      }),
  },
]

onMounted(fetchRules)
</script>
