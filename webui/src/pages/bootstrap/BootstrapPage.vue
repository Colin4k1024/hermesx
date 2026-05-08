<template>
  <div style="min-height:100vh;background:#0d1117;display:flex;align-items:center;justify-content:center">
    <div style="width:440px;background:#161b22;border:1px solid #30363d;border-radius:12px;padding:32px">
      <div style="margin-bottom:24px">
        <div style="font-size:20px;font-weight:700;color:#e6edf3">Initial Setup</div>
        <div style="font-size:13px;color:#8b949e;margin-top:6px">
          No admin key found. Use the ACP token from your deployment to create the first admin key.
        </div>
      </div>

      <template v-if="!createdKey">
        <n-form @submit.prevent="handleCreate">
          <n-form-item label="ACP Token" :show-feedback="false" style="margin-bottom:14px">
            <n-input v-model:value="acpToken" type="password" placeholder="HERMES_ACP_TOKEN value" show-password-on="click" :disabled="loading" />
          </n-form-item>
          <n-form-item label="Admin Key Name" :show-feedback="false" style="margin-bottom:20px">
            <n-input v-model:value="keyName" placeholder="initial-admin-key" :disabled="loading" />
          </n-form-item>

          <n-alert v-if="error" type="error" :title="error" style="margin-bottom:16px" />

          <n-button block type="primary" :loading="loading" @click="handleCreate" style="height:40px">
            Create Admin Key
          </n-button>
        </n-form>
      </template>

      <template v-else>
        <n-alert type="success" title="Admin key created!" style="margin-bottom:16px">
          Save this key now — it will NOT be shown again.
        </n-alert>

        <div style="background:#0d1117;border:1px solid #2ea043;border-radius:8px;padding:12px 16px;margin-bottom:16px;font-family:monospace;font-size:13px;color:#2ea043;word-break:break-all">
          {{ createdKey }}
        </div>

        <n-space justify="end" style="margin-bottom:16px">
          <n-button size="small" @click="copyKey">Copy Key</n-button>
        </n-space>

        <n-button block type="primary" @click="handleConfirm" style="height:40px">
          I've saved my key → Go to Login
        </n-button>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { NForm, NFormItem, NInput, NButton, NAlert, NSpace } from 'naive-ui'

const router = useRouter()
const acpToken = ref('')
const keyName = ref('initial-admin-key')
const loading = ref(false)
const error = ref<string | null>(null)
const createdKey = ref<string | null>(null)

async function handleCreate() {
  if (!acpToken.value.trim()) { error.value = 'ACP Token is required'; return }
  loading.value = true; error.value = null
  try {
    const res = await fetch('/admin/v1/bootstrap', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${acpToken.value.trim()}`,
      },
      body: JSON.stringify({ name: keyName.value.trim() || 'initial-admin-key' }),
    })
    if (!res.ok) {
      const data = await res.json() as { error?: string }
      throw new Error(data.error ?? `Bootstrap failed (${res.status})`)
    }
    const data = await res.json() as { key: string }
    createdKey.value = data.key
    acpToken.value = '' // clear sensitive input
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Bootstrap failed'
  } finally {
    loading.value = false
  }
}

function copyKey() {
  if (createdKey.value) void navigator.clipboard.writeText(createdKey.value)
}

function handleConfirm() {
  createdKey.value = null
  void router.push('/login')
}
</script>
