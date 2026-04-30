<template>
  <div style="min-height: 100vh; display: flex; align-items: center; justify-content: center; background: #f5f5f5">
    <n-card style="width: 420px" title="Connect to Hermes">
      <n-form ref="formRef" :model="form" :rules="rules" @keydown.enter="handleConnect">
        <n-form-item label="API Key" path="apiKey">
          <n-input
            v-model:value="form.apiKey"
            type="password"
            show-password-on="click"
            placeholder="hk_..."
            :disabled="loading"
          />
        </n-form-item>

        <n-form-item label="User ID" path="userId">
          <n-input
            v-model:value="form.userId"
            placeholder="e.g. alice"
            :disabled="loading"
          />
        </n-form-item>

        <n-form-item label="Admin Token (optional)" path="acpToken">
          <n-input
            v-model:value="form.acpToken"
            type="password"
            show-password-on="click"
            placeholder="Leave blank if not admin"
            :disabled="loading"
          />
        </n-form-item>

        <n-alert v-if="error" type="error" style="margin-bottom: 16px" :title="error" />

        <n-button
          type="primary"
          block
          :loading="loading"
          @click="handleConnect"
        >
          Connect
        </n-button>
      </n-form>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import {
  NCard,
  NForm,
  NFormItem,
  NInput,
  NButton,
  NAlert,
  type FormInst,
  type FormRules,
} from 'naive-ui'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()

const formRef = ref<FormInst | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

const form = reactive({
  apiKey: '',
  userId: '',
  acpToken: '',
})

const rules: FormRules = {
  apiKey: [{ required: true, message: 'API Key is required', trigger: 'blur' }],
  userId: [{ required: true, message: 'User ID is required', trigger: 'blur' }],
}

async function handleConnect() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  loading.value = true
  error.value = null

  try {
    await auth.connect(form.apiKey, form.userId, form.acpToken)
    router.push('/chat')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Connection failed'
  } finally {
    loading.value = false
  }
}
</script>
