<template>
  <div style="min-height:100vh;background:#0d1117;display:flex;align-items:center;justify-content:center">
    <div style="width:360px;background:#161b22;border:1px solid #30363d;border-radius:12px;padding:32px">
      <div style="text-align:center;margin-bottom:28px">
        <div style="font-size:24px;font-weight:700;color:#e6edf3;letter-spacing:-0.5px">HermesX</div>
        <div style="font-size:13px;color:#8b949e;margin-top:4px">Agent Portal</div>
      </div>

      <n-form @submit.prevent="handleConnect">
        <n-form-item label="API Key" :show-feedback="false" style="margin-bottom:14px">
          <n-input v-model:value="apiKey" type="password" placeholder="hx-..." show-password-on="click" :disabled="loading" />
        </n-form-item>
        <n-form-item label="User ID" :show-feedback="false" style="margin-bottom:20px">
          <n-input v-model:value="userId" placeholder="your-user-id" :disabled="loading" />
        </n-form-item>

        <n-alert v-if="error" type="error" :title="error" style="margin-bottom:16px" />

        <n-button block type="primary" :loading="loading" @click="handleConnect" style="height:40px">
          Connect
        </n-button>
      </n-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { NForm, NFormItem, NInput, NButton, NAlert } from 'naive-ui'
import { useAuthStore } from '@shared/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const apiKey = ref('')
const userId = ref('')
const loading = ref(false)
const error = ref<string | null>(null)

async function handleConnect() {
  if (!apiKey.value.trim() || !userId.value.trim()) {
    error.value = 'API Key and User ID are required'
    return
  }
  loading.value = true
  error.value = null
  try {
    await auth.connectUser(apiKey.value.trim(), userId.value.trim())
    void router.push('/chat')
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Connection failed'
  } finally {
    loading.value = false
  }
}
</script>
