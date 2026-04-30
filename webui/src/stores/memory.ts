import { defineStore } from 'pinia'
import { ref, reactive } from 'vue'
import type { MemoryEntry } from '@/types'

export const useMemoryStore = defineStore('memory', () => {
  const memories = ref<MemoryEntry[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const deleteLoading = reactive(new Set<string>())

  return { memories, loading, error, deleteLoading }
})
