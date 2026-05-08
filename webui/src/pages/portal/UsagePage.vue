<template>
  <div style="padding:24px;height:100%;overflow-y:auto;box-sizing:border-box">
    <h1 style="color:#e6edf3;font-size:20px;font-weight:600;margin:0 0 24px">Identity &amp; Usage</h1>

    <!-- Loading -->
    <div v-if="loading" style="display:flex;justify-content:center;padding:48px 0">
      <n-spin size="medium" />
    </div>

    <!-- Error -->
    <n-alert v-else-if="error" type="error" :title="error" style="margin-bottom:16px" />

    <!-- Content -->
    <div v-else style="display:flex;flex-direction:column;gap:20px;max-width:720px">
      <!-- Identity card -->
      <div style="background:#161b22;border:1px solid #30363d;border-radius:8px;padding:20px">
        <div style="color:#7d8590;font-size:12px;text-transform:uppercase;letter-spacing:0.8px;font-weight:600;margin-bottom:16px">Identity</div>

        <div style="display:grid;grid-template-columns:140px 1fr;gap:10px 16px;align-items:start">
          <div style="color:#7d8590;font-size:13px;padding-top:2px">Tenant ID</div>
          <div style="color:#e6edf3;font-size:13px;font-family:monospace;word-break:break-all">{{ meData?.tenant_id ?? '—' }}</div>

          <div style="color:#7d8590;font-size:13px;padding-top:2px">Identity</div>
          <div style="color:#e6edf3;font-size:13px;font-family:monospace;word-break:break-all">{{ meData?.identity ?? '—' }}</div>

          <div style="color:#7d8590;font-size:13px;padding-top:2px">Auth Method</div>
          <div style="color:#e6edf3;font-size:13px">{{ meData?.auth_method ?? '—' }}</div>

          <div style="color:#7d8590;font-size:13px;padding-top:4px">Roles</div>
          <div style="display:flex;flex-wrap:wrap;gap:6px">
            <n-tag
              v-for="role in meData?.roles ?? []"
              :key="role"
              size="small"
              style="background:#21262d;color:#58a6ff;border-color:#30363d"
            >
              {{ role }}
            </n-tag>
            <span v-if="!meData?.roles?.length" style="color:#7d8590;font-size:13px">—</span>
          </div>
        </div>
      </div>

      <!-- Usage card -->
      <div style="background:#161b22;border:1px solid #30363d;border-radius:8px;padding:20px">
        <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:20px">
          <div style="color:#7d8590;font-size:12px;text-transform:uppercase;letter-spacing:0.8px;font-weight:600">Usage</div>
          <div v-if="usageData?.period" style="color:#7d8590;font-size:12px;font-family:monospace">{{ usageData.period }}</div>
        </div>

        <div style="display:grid;grid-template-columns:repeat(4,1fr);gap:16px">
          <div v-for="stat in stats" :key="stat.label" style="text-align:center">
            <div style="color:#e6edf3;font-size:24px;font-weight:700;font-variant-numeric:tabular-nums">
              {{ stat.value }}
            </div>
            <div style="color:#7d8590;font-size:11px;margin-top:4px;text-transform:uppercase;letter-spacing:0.5px">
              {{ stat.label }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { NSpin, NAlert, NTag } from 'naive-ui'
import { useApi } from '@shared/composables/useApi'
import type { MeResponse, UsageResponse } from '@shared/types/index'

const { get } = useApi()

const meData = ref<MeResponse | null>(null)
const usageData = ref<UsageResponse | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

function formatNumber(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

const stats = computed(() => [
  { label: 'Input Tokens', value: formatNumber(usageData.value?.input_tokens ?? 0) },
  { label: 'Output Tokens', value: formatNumber(usageData.value?.output_tokens ?? 0) },
  { label: 'Total Tokens', value: formatNumber(usageData.value?.total_tokens ?? 0) },
  { label: 'Est. Cost USD', value: usageData.value ? `$${usageData.value.estimated_cost_usd.toFixed(4)}` : '$0.0000' },
])

async function loadData() {
  loading.value = true
  error.value = null
  try {
    const [me, usage] = await Promise.all([
      get<MeResponse>('/v1/me'),
      get<UsageResponse>('/v1/usage'),
    ])
    meData.value = me ?? null
    usageData.value = usage ?? null
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load data'
  } finally {
    loading.value = false
  }
}

onMounted(() => loadData())
</script>
