<template>
  <div style="padding:24px;height:100%;overflow-y:auto;box-sizing:border-box">
    <!-- Header -->
    <div style="display:flex;align-items:center;gap:12px;margin-bottom:24px">
      <h1 style="color:#e6edf3;font-size:20px;font-weight:600;margin:0">Memories</h1>
      <n-tag v-if="!loading && !error" type="default" size="small" style="background:#21262d;color:#7d8590;border-color:#30363d">
        {{ memories.length }}
      </n-tag>
    </div>

    <!-- Loading -->
    <div v-if="loading" style="display:flex;justify-content:center;padding:48px 0">
      <n-spin size="medium" />
    </div>

    <!-- Error -->
    <n-alert v-else-if="error" type="error" :title="error" style="margin-bottom:16px" />

    <!-- Empty -->
    <n-empty v-else-if="memories.length === 0" description="No memories stored" style="padding:48px 0" />

    <!-- Memory list -->
    <div v-else style="display:flex;flex-direction:column;gap:12px">
      <div
        v-for="mem in memories"
        :key="mem.key"
        style="background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px;display:flex;align-items:flex-start;gap:16px"
      >
        <div style="flex:1;min-width:0">
          <div style="color:#e6edf3;font-weight:600;font-size:14px;margin-bottom:6px;word-break:break-all">
            {{ mem.key }}
          </div>
          <div style="color:#7d8590;font-size:13px;line-height:1.5;overflow:hidden;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical">
            {{ mem.content }}
          </div>
        </div>
        <n-popconfirm
          positive-text="Delete"
          negative-text="Cancel"
          @positive-click="deleteMemory(mem.key)"
        >
          <template #trigger>
            <n-button text size="small" style="color:#7d8590;flex-shrink:0;margin-top:2px" :loading="deletingKey === mem.key">
              🗑
            </n-button>
          </template>
          Delete this memory?
        </n-popconfirm>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NSpin, NAlert, NEmpty, NButton, NPopconfirm, NTag } from 'naive-ui'
import { useApi } from '@shared/composables/useApi'
import type { MemoryEntry, MemoryListResponse } from '@shared/types/index'

const { get, del } = useApi()

const memories = ref<MemoryEntry[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const deletingKey = ref<string | null>(null)

async function loadMemories() {
  loading.value = true
  error.value = null
  try {
    const data = await get<MemoryListResponse>('/v1/memories')
    memories.value = data?.memories ?? []
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load memories'
  } finally {
    loading.value = false
  }
}

async function deleteMemory(key: string) {
  deletingKey.value = key
  try {
    await del(`/v1/memories/${encodeURIComponent(key)}`)
    memories.value = memories.value.filter(m => m.key !== key)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to delete memory'
  } finally {
    deletingKey.value = null
  }
}

onMounted(() => loadMemories())
</script>
