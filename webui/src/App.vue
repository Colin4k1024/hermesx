<template>
  <n-config-provider :theme="null" :locale="enUS" :date-locale="dateEnUS">
    <n-message-provider>
      <n-notification-provider>
        <AppShell />
      </n-notification-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { enUS, dateEnUS, NConfigProvider, NMessageProvider, NNotificationProvider } from 'naive-ui'
import AppShell from '@/components/layout/AppShell.vue'
</script>
