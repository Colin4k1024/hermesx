<template>
  <SkillsPageBase>
    <template #sidebar-footer>
      <div style="padding: 12px; border-top: 1px solid #e8e8e8">
        <n-button block size="small" type="primary" @click="showUpload = true">Upload Skill</n-button>
      </div>
    </template>
  </SkillsPageBase>

  <!-- Upload Modal -->
  <n-modal v-model:show="showUpload" preset="card" title="Upload Skill" style="width: 440px">
    <n-form @submit.prevent="handleUpload">
      <n-form-item label="Skill Name">
        <n-input v-model:value="uploadName" placeholder="e.g. my-skill" :disabled="uploading" />
      </n-form-item>
      <n-form-item label="Skill File (.md)">
        <n-upload accept=".md,.yaml,.yml" :max="1" :file-list="fileList" @change="onFileChange">
          <n-button>Select File</n-button>
        </n-upload>
      </n-form-item>
      <n-alert v-if="uploadError" type="error" :title="uploadError" style="margin-bottom: 12px" />
      <n-space justify="end">
        <n-button @click="showUpload = false" :disabled="uploading">Cancel</n-button>
        <n-button type="primary" :loading="uploading" @click="handleUpload">Upload</n-button>
      </n-space>
    </n-form>
  </n-modal>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import {
  NModal, NForm, NFormItem, NInput, NUpload, NButton, NSpace, NAlert,
  type UploadFileInfo,
} from 'naive-ui'
import SkillsPageBase from '@/pages/SkillsPage.vue'
import { useApi } from '@/composables/useApi'
import { useSkillStore } from '@/stores/skill'
import { useAuthStore } from '@/stores/auth'
import { normalizeApiError, ApiError } from '@/utils/errors'

const api = useApi()
const skill = useSkillStore()
const auth = useAuthStore()

const showUpload = ref(false)
const uploadName = ref('')
const fileList = ref<UploadFileInfo[]>([])
const uploading = ref(false)
const uploadError = ref<string | null>(null)

function onFileChange({ fileList: list }: { fileList: UploadFileInfo[] }) {
  fileList.value = list
}

async function handleUpload() {
  if (!uploadName.value.trim() || !fileList.value[0]?.file) return
  uploading.value = true
  uploadError.value = null
  try {
    const file = fileList.value[0].file
    const r = await fetch(`/v1/skills/${encodeURIComponent(uploadName.value.trim())}`, {
      method: 'PUT',
      headers: {
        Authorization: `Bearer ${auth.acpToken}`,
        'X-Hermes-User-Id': auth.userId,
        'Content-Type': 'text/plain',
      },
      body: file,
    })
    if (!r.ok) throw new ApiError(`Upload failed (${r.status})`, r.status)
    showUpload.value = false
    uploadName.value = ''
    fileList.value = []
    // Refresh skill list
    await reloadSkills()
  } catch (e) {
    uploadError.value = normalizeApiError(e).message
  } finally {
    uploading.value = false
  }
}

async function reloadSkills() {
  skill.listLoading = true
  skill.listError = null
  try {
    const data = await api.get<{ skills: typeof skill.skills; count: number }>('/v1/skills')
    skill.skills = data.skills ?? []
  } catch {
    // ignore
  } finally {
    skill.listLoading = false
  }
}
</script>
