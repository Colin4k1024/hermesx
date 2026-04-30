import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Session, ChatMessage } from '@/types'

export const useChatStore = defineStore('chat', () => {
  const sessions = ref<Session[]>([])
  const currentSessionId = ref<string | null>(null)
  const messages = ref<ChatMessage[]>([])

  const sessionsLoading = ref(false)
  const sessionsError = ref<string | null>(null)
  const messagesLoading = ref(false)
  const messagesError = ref<string | null>(null)
  const sendLoading = ref(false)
  const sendError = ref<string | null>(null)

  function newSession() {
    currentSessionId.value = null
    messages.value = []
    sendError.value = null
  }

  return {
    sessions,
    currentSessionId,
    messages,
    sessionsLoading,
    sessionsError,
    messagesLoading,
    messagesError,
    sendLoading,
    sendError,
    newSession,
  }
})
