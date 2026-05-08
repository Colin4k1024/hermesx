<template>
  <div style="display:flex;height:100vh;background:#0d1117;overflow:hidden">
    <!-- Sidebar -->
    <nav style="width:220px;flex-shrink:0;background:#161b22;border-right:1px solid #30363d;display:flex;flex-direction:column;overflow:hidden">
      <!-- Header -->
      <div style="padding:20px 16px 12px;border-bottom:1px solid #30363d">
        <div style="color:#e6edf3;font-size:18px;font-weight:700;letter-spacing:-0.3px">HermesX</div>
        <div style="color:#7d8590;font-size:12px;margin-top:4px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">{{ auth.userId }}</div>
      </div>

      <!-- Nav links -->
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
            style="display:flex;align-items:center;gap:10px;padding:10px 16px;cursor:pointer;font-size:14px;transition:background 0.1s"
          >
            <span :style="{ color: isActive ? '#e6edf3' : '#7d8590' }">{{ item.icon }}</span>
            <span :style="{ color: isActive ? '#e6edf3' : '#7d8590' }">{{ item.label }}</span>
          </div>
        </router-link>
      </div>

      <!-- Footer -->
      <div style="padding:12px 16px;border-top:1px solid #30363d">
        <n-button
          block
          ghost
          size="small"
          style="border-color:#30363d;color:#7d8590"
          @click="handleSignOut"
        >
          Sign out
        </n-button>
      </div>
    </nav>

    <!-- Main content -->
    <main style="flex:1;display:flex;flex-direction:column;overflow:hidden;min-width:0">
      <slot />
    </main>
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from '@shared/stores/auth'
import { useRouter } from 'vue-router'
import { NButton } from 'naive-ui'

const auth = useAuthStore()
const router = useRouter()

const navItems = [
  { path: '/chat', label: 'Chat', icon: '💬' },
  { path: '/memories', label: 'Memories', icon: '🧠' },
  { path: '/skills', label: 'Skills', icon: '⚡' },
  { path: '/usage', label: 'Usage', icon: '📊' },
]

function navItemStyle(isActive: boolean) {
  if (isActive) {
    return {
      borderLeft: '2px solid #2ea043',
      background: 'rgba(46,160,67,0.08)',
      paddingLeft: '14px',
    }
  }
  return {
    borderLeft: '2px solid transparent',
  }
}

function handleSignOut() {
  auth.disconnectUser()
  router.push('/login')
}
</script>
