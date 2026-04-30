<template>
  <div v-if="!auth.connected" style="height: 100vh">
    <RouterView />
  </div>

  <n-layout v-else style="height: 100vh" has-sider>
    <n-layout-sider
      bordered
      collapse-mode="width"
      :collapsed-width="64"
      :width="220"
      show-trigger
    >
      <div style="padding: 16px 8px 8px; font-weight: 700; font-size: 18px; letter-spacing: -0.5px">
        Hermes
      </div>
      <n-menu :options="menuOptions" :value="activeKey" @update:value="handleMenuClick" />

      <div style="position: absolute; bottom: 16px; left: 0; right: 0; padding: 0 12px">
        <n-space vertical>
          <n-text depth="3" style="font-size: 12px; word-break: break-all">
            {{ auth.tenantId ? `Tenant: ${auth.tenantId.slice(0, 8)}…` : '' }}
          </n-text>
          <n-text depth="3" style="font-size: 12px">
            {{ auth.userId }}
          </n-text>
          <n-button size="small" quaternary @click="handleDisconnect">Disconnect</n-button>
        </n-space>
      </div>
    </n-layout-sider>

    <n-layout>
      <n-layout-content style="height: 100vh; overflow-y: auto">
        <RouterView />
      </n-layout-content>
    </n-layout>
  </n-layout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import {
  NLayout,
  NLayoutSider,
  NLayoutContent,
  NMenu,
  NButton,
  NSpace,
  NText,
  type MenuOption,
} from 'naive-ui'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()

const activeKey = computed(() => route.path)

const menuOptions = computed<MenuOption[]>(() => {
  const base: MenuOption[] = [
    { label: 'Chat', key: '/chat' },
    { label: 'Memories', key: '/memories' },
    { label: 'Skills', key: '/skills' },
  ]
  if (auth.isAdmin) {
    base.push(
      { type: 'divider', key: 'divider-admin' },
      { label: 'Admin: Skills', key: '/admin/skills' },
      { label: 'Admin: Tenants', key: '/admin/tenants' },
      { label: 'Admin: API Keys', key: '/admin/keys' },
    )
  }
  return base
})

function handleMenuClick(key: string) {
  router.push(key)
}

function handleDisconnect() {
  auth.disconnect()
  router.push('/connect')
}
</script>
