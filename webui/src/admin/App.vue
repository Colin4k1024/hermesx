<template>
  <AdminShell v-if="showShell">
    <RouterView />
  </AdminShell>
  <RouterView v-else />
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@shared/stores/auth'
import AdminShell from '@/pages/admin/AdminShell.vue'

const auth = useAuthStore()
const route = useRoute()
onMounted(() => auth.rehydrateAdmin())
const showShell = computed(() => !route.meta.public)
</script>
