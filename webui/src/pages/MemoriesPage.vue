<template>
  <div style="padding: 24px">
    <n-page-header title="Memories" subtitle="Agent memories about you">
      <template #extra>
        <n-button size="small" @click="load">Refresh</n-button>
      </template>
    </n-page-header>

    <n-divider />

    <n-spin v-if="mem.loading" style="display: flex; justify-content: center; padding: 48px" />

    <n-alert v-else-if="mem.error" type="error" :title="mem.error">
      <n-button size="small" @click="load">Retry</n-button>
    </n-alert>

    <n-empty v-else-if="!mem.memories.length" description="No memories stored" style="padding: 48px" />

    <n-data-table
      v-else
      :columns="columns"
      :data="mem.memories"
      :pagination="{ pageSize: 20 }"
      striped
    />
  </div>
</template>

<script setup lang="ts">
import { h, onMounted } from 'vue'
import {
  NPageHeader, NDivider, NSpin, NAlert, NEmpty, NDataTable,
  NButton, NPopconfirm, NSpace,
  type DataTableColumns,
} from 'naive-ui'
import { useMemoryStore } from '@/stores/memory'
import { useApi } from '@/composables/useApi'
import type { MemoryListResponse, MemoryEntry } from '@/types'
import { normalizeApiError } from '@/utils/errors'

const mem = useMemoryStore()
const api = useApi()

onMounted(load)

async function load() {
  mem.loading = true
  mem.error = null
  try {
    const data = await api.get<MemoryListResponse>('/v1/memories')
    mem.memories = data.memories ?? []
  } catch (e) {
    mem.error = normalizeApiError(e).message
  } finally {
    mem.loading = false
  }
}

async function deleteMemory(key: string) {
  mem.deleteLoading.add(key)
  try {
    await api.del(`/v1/memories/${encodeURIComponent(key)}`)
    mem.memories = mem.memories.filter((m) => m.key !== key)
  } catch (e) {
    mem.error = normalizeApiError(e).message
  } finally {
    mem.deleteLoading.delete(key)
  }
}

const columns: DataTableColumns<MemoryEntry> = [
  { title: 'Key', key: 'key', width: 200, ellipsis: true },
  { title: 'Content', key: 'content', ellipsis: true },
  {
    title: 'Actions',
    key: 'actions',
    width: 100,
    render(row) {
      return h(NSpace, null, {
        default: () => [
          h(
            NPopconfirm,
            { onPositiveClick: () => deleteMemory(row.key) },
            {
              trigger: () =>
                h(
                  NButton,
                  {
                    size: 'small',
                    type: 'error',
                    loading: mem.deleteLoading.has(row.key),
                    disabled: mem.deleteLoading.has(row.key),
                  },
                  { default: () => 'Delete' },
                ),
              default: () => 'Delete this memory?',
            },
          ),
        ],
      })
    },
  },
]
</script>
