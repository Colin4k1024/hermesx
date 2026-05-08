<template>
  <div style="display:flex;height:100vh;overflow:hidden;background:#0d1117">
    <!-- Sidebar -->
    <nav style="width:220px;flex-shrink:0;background:#161b22;border-right:1px solid #30363d;display:flex;flex-direction:column">
      <!-- Header -->
      <div style="padding:20px 16px 16px;border-bottom:1px solid #30363d">
        <div style="color:#e6edf3;font-size:16px;font-weight:700;letter-spacing:0.3px">HermesX</div>
        <div style="color:#7d8590;font-size:12px;margin-top:2px">Admin Console</div>
      </div>

      <!-- Nav -->
      <div style="flex:1;padding:8px 0;overflow-y:auto">
        <router-link
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          custom
          v-slot="{ isActive, navigate }"
        >
          <div
            @click="navigate"
            :style="navItemStyle(isActive)"
            style="display:flex;align-items:center;padding:9px 16px;cursor:pointer;margin:1px 8px;border-radius:6px;font-size:14px;transition:background 0.15s"
          >
            {{ item.label }}
          </div>
        </router-link>
      </div>

      <!-- Footer -->
      <div style="padding:12px 16px;border-top:1px solid #30363d">
        <n-button
          size="small"
          style="width:100%;color:#7d8590;background:transparent;border:1px solid #30363d"
          @click="signOut"
        >
          Sign out
        </n-button>
      </div>
    </nav>

    <!-- Main -->
    <main style="flex:1;padding:24px;overflow-y:auto;background:#0d1117">
      <slot />
    </main>
  </div>
</template>

<script setup lang="ts">
import { NButton } from 'naive-ui'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@shared/stores/auth'

const router = useRouter()
const auth = useAuthStore()

const navItems = [
  { path: '/tenants', label: 'Tenants' },
  { path: '/keys', label: 'API Keys' },
  { path: '/audit', label: 'Audit Logs' },
  { path: '/pricing', label: 'Pricing' },
  { path: '/sandbox', label: 'Sandbox' },
]

function navItemStyle(isActive: boolean) {
  if (isActive) {
    return {
      borderLeft: '3px solid #2ea043',
      paddingLeft: '13px',
      background: 'rgba(46,160,67,0.08)',
      color: '#e6edf3',
      fontWeight: '500',
    }
  }
  return {
    borderLeft: '3px solid transparent',
    paddingLeft: '13px',
    background: 'transparent',
    color: '#7d8590',
    fontWeight: '400',
  }
}

async function signOut() {
  auth.disconnectAdmin()
  router.push('/login')
}
</script>
