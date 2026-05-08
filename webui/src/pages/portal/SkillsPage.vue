<template>
  <div style="padding:24px;height:100%;overflow-y:auto;box-sizing:border-box">
    <!-- Header -->
    <div style="display:flex;align-items:center;gap:12px;margin-bottom:24px">
      <h1 style="color:#e6edf3;font-size:20px;font-weight:600;margin:0">Skills</h1>
      <n-tag v-if="!loading && !error" type="default" size="small" style="background:#21262d;color:#7d8590;border-color:#30363d">
        {{ skills.length }}
      </n-tag>
    </div>

    <!-- Loading -->
    <div v-if="loading" style="display:flex;justify-content:center;padding:48px 0">
      <n-spin size="medium" />
    </div>

    <!-- Error -->
    <n-alert v-else-if="error" type="error" :title="error" style="margin-bottom:16px" />

    <!-- Empty -->
    <n-empty v-else-if="skills.length === 0" description="No skills available" style="padding:48px 0" />

    <!-- Skills grid -->
    <div
      v-else
      style="display:grid;grid-template-columns:repeat(3,1fr);gap:16px"
    >
      <div
        v-for="skill in skills"
        :key="skill.name"
        style="background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px;display:flex;flex-direction:column;gap:12px"
      >
        <div style="flex:1">
          <div style="display:flex;align-items:flex-start;justify-content:space-between;gap:8px;margin-bottom:8px">
            <div style="color:#e6edf3;font-weight:600;font-size:15px;word-break:break-word">{{ skill.name }}</div>
            <n-tag v-if="skill.version" size="tiny" style="background:#21262d;color:#7d8590;border-color:#30363d;flex-shrink:0;font-family:monospace">
              {{ skill.version }}
            </n-tag>
          </div>
          <div
            v-if="skill.description"
            style="color:#7d8590;font-size:13px;line-height:1.5;overflow:hidden;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical"
          >
            {{ skill.description }}
          </div>
          <div v-else style="color:#484f58;font-size:13px;font-style:italic">No description</div>
        </div>
        <n-button size="small" ghost style="border-color:#30363d;color:#7d8590;align-self:flex-start" @click="openDetail(skill.name)">
          Details
        </n-button>
      </div>
    </div>

    <!-- Detail modal -->
    <n-modal
      v-model:show="showModal"
      preset="card"
      style="width:560px;background:#161b22;border:1px solid #30363d"
      :title="selectedSkillName ?? ''"
      :bordered="false"
      @after-enter="loadDetail"
    >
      <div v-if="detailLoading" style="display:flex;justify-content:center;padding:32px 0">
        <n-spin size="medium" />
      </div>
      <div v-else-if="detailError" style="color:#f85149;font-size:14px">{{ detailError }}</div>
      <div v-else-if="selectedSkill" style="display:flex;flex-direction:column;gap:16px">
        <div style="display:flex;align-items:center;gap:10px;flex-wrap:wrap">
          <n-tag v-if="selectedSkill.version" size="small" style="background:#21262d;color:#7d8590;border-color:#30363d;font-family:monospace">
            v{{ selectedSkill.version }}
          </n-tag>
          <n-tag v-if="selectedSkill.source" size="small" style="background:#21262d;color:#7d8590;border-color:#30363d">
            {{ selectedSkill.source }}
          </n-tag>
        </div>
        <div v-if="selectedSkill.description">
          <div style="color:#7d8590;font-size:12px;text-transform:uppercase;letter-spacing:0.5px;margin-bottom:6px">Description</div>
          <div style="color:#e6edf3;font-size:14px;line-height:1.6">{{ selectedSkill.description }}</div>
        </div>
        <div v-if="selectedSkill.source">
          <div style="color:#7d8590;font-size:12px;text-transform:uppercase;letter-spacing:0.5px;margin-bottom:6px">Source</div>
          <div style="color:#e6edf3;font-size:14px;font-family:monospace;word-break:break-all">{{ selectedSkill.source }}</div>
        </div>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NSpin, NAlert, NEmpty, NButton, NTag, NModal } from 'naive-ui'
import { useApi } from '@shared/composables/useApi'
import type { SkillItem, SkillListResponse } from '@shared/types/index'

const { get } = useApi()

const skills = ref<SkillItem[]>([])
const loading = ref(false)
const error = ref<string | null>(null)

const showModal = ref(false)
const selectedSkillName = ref<string | null>(null)
const selectedSkill = ref<SkillItem | null>(null)
const detailLoading = ref(false)
const detailError = ref<string | null>(null)

async function loadSkills() {
  loading.value = true
  error.value = null
  try {
    const data = await get<SkillListResponse>('/v1/skills')
    skills.value = data?.skills ?? []
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load skills'
  } finally {
    loading.value = false
  }
}

function openDetail(name: string) {
  selectedSkillName.value = name
  selectedSkill.value = null
  detailError.value = null
  showModal.value = true
}

async function loadDetail() {
  if (!selectedSkillName.value) return
  detailLoading.value = true
  detailError.value = null
  try {
    const data = await get<SkillItem>(`/v1/skills/${encodeURIComponent(selectedSkillName.value)}`)
    selectedSkill.value = data ?? null
  } catch (err) {
    detailError.value = err instanceof Error ? err.message : 'Failed to load skill details'
  } finally {
    detailLoading.value = false
  }
}

onMounted(() => loadSkills())
</script>

<style scoped>
@media (max-width: 900px) {
  div[style*="grid-template-columns:repeat(3"] {
    grid-template-columns: repeat(2, 1fr) !important;
  }
}
@media (max-width: 560px) {
  div[style*="grid-template-columns:repeat(3"] {
    grid-template-columns: 1fr !important;
  }
}
</style>
