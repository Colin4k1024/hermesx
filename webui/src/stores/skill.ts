import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { SkillItem } from '@/types'

export const useSkillStore = defineStore('skill', () => {
  const skills = ref<SkillItem[]>([])
  const selectedSkillName = ref<string | null>(null)
  const selectedSkillContent = ref<string | null>(null)

  const listLoading = ref(false)
  const listError = ref<string | null>(null)
  const contentLoading = ref(false)
  const contentError = ref<string | null>(null)
  const uploadLoading = ref(false)
  const uploadError = ref<string | null>(null)

  return {
    skills,
    selectedSkillName,
    selectedSkillContent,
    listLoading,
    listError,
    contentLoading,
    contentError,
    uploadLoading,
    uploadError,
  }
})
