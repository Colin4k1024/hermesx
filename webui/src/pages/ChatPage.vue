<template>
  <div style="display: flex; height: 100vh">
    <!-- Session Sidebar -->
    <div style="width: 220px; border-right: 1px solid #e8e8e8; display: flex; flex-direction: column; flex-shrink: 0">
      <div style="padding: 12px">
        <n-button block type="primary" size="small" @click="handleNewChat">+ New Chat</n-button>
      </div>

      <div style="flex: 1; overflow-y: auto">
        <n-spin v-if="chat.sessionsLoading" style="display: flex; justify-content: center; padding: 24px" />

        <n-empty v-else-if="!chat.sessionsLoading && chat.sessions.length === 0 && !chat.sessionsError"
          description="No sessions yet" style="padding: 24px" size="small" />

        <n-alert v-else-if="chat.sessionsError" type="error" :title="chat.sessionsError" size="small" style="margin: 8px">
          <n-button size="tiny" @click="loadSessions">Retry</n-button>
        </n-alert>

        <div
          v-for="sess in chat.sessions"
          :key="sess.id"
          :style="{
            padding: '10px 12px',
            cursor: 'pointer',
            background: chat.currentSessionId === sess.id ? '#f0f0f0' : 'transparent',
            borderLeft: chat.currentSessionId === sess.id ? '3px solid #18a058' : '3px solid transparent',
          }"
          @click="selectSession(sess.id)"
        >
          <n-text style="font-size: 12px; display: block" strong>
            {{ sess.id.slice(0, 12) }}…
          </n-text>
          <n-text depth="3" style="font-size: 11px">
            {{ formatDate(sess.started_at) }} · {{ sess.message_count }} msgs
          </n-text>
        </div>
      </div>
    </div>

    <!-- Main Chat Area -->
    <div style="flex: 1; display: flex; flex-direction: column; min-width: 0">
      <!-- Messages -->
      <div ref="messageListEl" style="flex: 1; overflow-y: auto; padding: 24px">
        <n-spin v-if="chat.messagesLoading" style="display: flex; justify-content: center; padding: 48px" />

        <n-empty
          v-else-if="chat.messages.length === 0 && !chat.messagesLoading"
          description="Send a message to start"
          style="margin-top: 80px"
        />

        <div v-else>
          <div
            v-for="(msg, i) in chat.messages"
            :key="i"
            :style="{
              display: 'flex',
              justifyContent: msg.role === 'user' ? 'flex-end' : 'flex-start',
              marginBottom: '16px',
            }"
          >
            <div
              :style="{
                maxWidth: '70%',
                padding: '10px 14px',
                borderRadius: '12px',
                background: msg.role === 'user' ? '#18a058' : '#f5f5f5',
                color: msg.role === 'user' ? '#fff' : 'inherit',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
                fontSize: '14px',
                lineHeight: '1.6',
              }"
            >
              {{ msg.content }}
            </div>
          </div>

          <!-- Loading bubble -->
          <div v-if="chat.sendLoading" style="display: flex; justify-content: flex-start; margin-bottom: 16px">
            <div style="padding: 10px 14px; border-radius: 12px; background: #f5f5f5">
              <n-spin size="small" />
            </div>
          </div>
        </div>

        <n-alert v-if="chat.sendError" type="error" :title="chat.sendError" style="margin-top: 8px">
          <n-button size="tiny" @click="chat.sendError = null">Dismiss</n-button>
        </n-alert>
      </div>

      <!-- Input Area -->
      <div style="padding: 16px; border-top: 1px solid #e8e8e8">
        <n-input-group>
          <n-input
            v-model:value="inputText"
            type="textarea"
            :autosize="{ minRows: 1, maxRows: 4 }"
            placeholder="Type a message… (Enter to send, Shift+Enter for newline)"
            :disabled="chat.sendLoading"
            @keydown="handleKeydown"
          />
          <n-button
            type="primary"
            :disabled="!inputText.trim() || chat.sendLoading"
            :loading="chat.sendLoading"
            @click="handleSend"
          >
            Send
          </n-button>
        </n-input-group>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, onMounted } from 'vue'
import {
  NButton, NInput, NInputGroup, NSpin, NEmpty, NAlert, NText,
} from 'naive-ui'
import { useChatStore } from '@/stores/chat'
import { useApi } from '@/composables/useApi'
import type { SessionListResponse, SessionDetailResponse, ChatResponse } from '@/types'
import { normalizeApiError } from '@/utils/errors'

const chat = useChatStore()
const api = useApi()

const inputText = ref('')
const messageListEl = ref<HTMLElement | null>(null)

onMounted(() => {
  loadSessions()
})

async function loadSessions() {
  chat.sessionsLoading = true
  chat.sessionsError = null
  try {
    const data = await api.get<SessionListResponse>('/v1/sessions')
    chat.sessions = data.sessions ?? []
  } catch (e) {
    chat.sessionsError = normalizeApiError(e).message
  } finally {
    chat.sessionsLoading = false
  }
}

async function selectSession(id: string) {
  chat.currentSessionId = id
  chat.messagesLoading = true
  chat.messagesError = null
  try {
    const data = await api.get<SessionDetailResponse>(`/v1/sessions/${id}`)
    chat.messages = data.messages ?? []
  } catch (e) {
    chat.messagesError = normalizeApiError(e).message
  } finally {
    chat.messagesLoading = false
    scrollToBottom()
  }
}

function handleNewChat() {
  chat.newSession()
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    handleSend()
  }
}

async function handleSend() {
  const content = inputText.value.trim()
  if (!content || chat.sendLoading) return

  inputText.value = ''
  chat.sendError = null

  // Optimistic user message
  chat.messages.push({ role: 'user', content })
  scrollToBottom()

  chat.sendLoading = true

  try {
    const body = {
      model: '',
      messages: chat.messages.filter((m) => m.role !== 'system'),
      stream: false as const,
    }

    const data = await api.post<ChatResponse>('/v1/agent/chat', body, {
      sessionId: chat.currentSessionId ?? undefined,
    })

    const reply = data.choices?.[0]?.message?.content ?? ''
    chat.messages.push({ role: 'assistant', content: reply })

    // Update session ID from response if not set
    if (!chat.currentSessionId && data.id) {
      chat.currentSessionId = data.id
      loadSessions()
    }
  } catch (e) {
    chat.sendError = normalizeApiError(e).message
    // Remove optimistic message on failure
    chat.messages.pop()
    inputText.value = content
  } finally {
    chat.sendLoading = false
    scrollToBottom()
  }
}

function scrollToBottom() {
  nextTick(() => {
    if (messageListEl.value) {
      messageListEl.value.scrollTop = messageListEl.value.scrollHeight
    }
  })
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString()
}
</script>
