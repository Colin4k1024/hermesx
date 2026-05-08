<template>
  <UserPortalShell v-if="showShell">
    <RouterView />
  </UserPortalShell>
  <RouterView v-else />
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@shared/stores/auth'
import UserPortalShell from '@/pages/portal/UserPortalShell.vue'

const auth = useAuthStore()
const route = useRoute()

onMounted(() => auth.rehydrateUser())

const showShell = computed(() => !route.meta.public)
</script>
