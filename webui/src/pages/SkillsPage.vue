<template>
  <div style="display: flex; height: 100vh">
    <!-- Skill List -->
    <div style="width: 260px; border-right: 1px solid #e8e8e8; display: flex; flex-direction: column; flex-shrink: 0">
      <div style="padding: 16px 16px 8px; font-weight: 600">Skills</div>

      <div style="flex: 1; overflow-y: auto">
        <n-spin v-if="skill.listLoading" style="display: flex; justify-content: center; padding: 24px" />

        <n-alert v-else-if="skill.listError" type="warning" :title="skill.listError" size="small" style="margin: 8px">
          <n-button size="tiny" @click="loadSkills">Retry</n-button>
        </n-alert>

        <n-empty v-else-if="!skill.skills.length" description="No skills installed" size="small" style="padding: 24px" />

        <div
          v-for="s in skill.skills"
          :key="s.name"
          :style="{
            padding: '10px 16px',
            cursor: 'pointer',
            background: skill.selectedSkillName === s.name ? '#f0f0f0' : 'transparent',
            borderLeft: skill.selectedSkillName === s.name ? '3px solid #18a058' : '3px solid transparent',
          }"
          @click="selectSkill(s.name)"
        >
          <n-text style="font-size: 13px; display: block" strong>{{ s.name }}</n-text>
          <n-text depth="3" style="font-size: 11px">{{ s.version ?? '' }}</n-text>
        </div>
      </div>

      <slot name="sidebar-footer" />
    </div>

    <!-- Skill Content -->
    <div style="flex: 1; padding: 24px; overflow-y: auto">
      <n-spin v-if="skill.contentLoading" style="display: flex; justify-content: center; padding: 48px" />

      <n-alert v-else-if="skill.contentError" type="error" :title="skill.contentError" />

      <n-empty
        v-else-if="!skill.selectedSkillContent"
        description="Select a skill to view its content"
        style="margin-top: 80px"
      />

      <div v-else>
        <n-page-header :title="skill.selectedSkillName ?? ''" />
        <n-code :code="skill.selectedSkillContent" language="markdown" style="margin-top: 16px" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { NSpin, NAlert, NEmpty, NText, NButton, NCode, NPageHeader } from 'naive-ui'
import { useSkillStore } from '@/stores/skill'
import { useAuthStore } from '@/stores/auth'
import { useApi } from '@/composables/useApi'
import type { SkillListResponse } from '@/types'
import { normalizeApiError, ApiError } from '@/utils/errors'

const skill = useSkillStore()
const auth = useAuthStore()
const api = useApi()

onMounted(loadSkills)

async function loadSkills() {
  skill.listLoading = true
  skill.listError = null
  try {
    const data = await api.get<SkillListResponse>('/v1/skills')
    skill.skills = data.skills ?? []
  } catch (e) {
    // 404 means skills service not configured — show as informational, not blocking error
    if (e instanceof ApiError && e.status === 404) {
      skill.skills = []
      skill.listError = 'Skills not configured on this server'
    } else {
      skill.listError = normalizeApiError(e).message
    }
  } finally {
    skill.listLoading = false
  }
}

async function selectSkill(name: string) {
  skill.selectedSkillName = name
  skill.selectedSkillContent = null
  skill.contentLoading = true
  skill.contentError = null
  try {
    // GET /v1/skills/{name} returns plain text Markdown
    const r = await fetch(`/v1/skills/${encodeURIComponent(name)}`, {
      headers: {
        Authorization: `Bearer ${auth.apiKey}`,
        'X-Hermes-User-Id': auth.userId,
      },
    })
    if (!r.ok) throw new ApiError(`Failed to load skill (${r.status})`, r.status)
    skill.selectedSkillContent = await r.text()
  } catch (e) {
    skill.contentError = normalizeApiError(e).message
  } finally {
    skill.contentLoading = false
  }
}
</script>
