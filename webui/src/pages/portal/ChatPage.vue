<template>
  <div style="display:flex;height:100%;overflow:hidden">
    <!-- Sessions sidebar -->
    <div style="width:220px;flex-shrink:0;background:#161b22;border-right:1px solid #30363d;display:flex;flex-direction:column;overflow:hidden">
      <div style="padding:12px;border-bottom:1px solid #30363d">
        <n-button block type="primary" size="small" @click="newSession">
          + New Session
        </n-button>
      </div>

      <div style="flex:1;overflow-y:auto">
        <div v-if="sessionsLoading" style="padding:16px;text-align:center">
          <n-spin size="small" />
        </div>
        <div v-else-if="sessions.length === 0" style="padding:16px;color:#7d8590;font-size:13px;text-align:center">
          No sessions yet
        </div>
        <div
          v-else
          v-for="s in sessions"
          :key="s.id"
          :style="sessionItemStyle(s.id === currentSessionId)"
          style="padding:10px 12px;cursor:pointer;border-bottom:1px solid #21262d;display:flex;align-items:center;justify-content:space-between;gap:8px"
          @click="selectSession(s.id)"
        >
          <div style="min-width:0">
            <div style="font-size:13px;font-family:monospace;white-space:nowrap;overflow:hidden;text-overflow:ellipsis" :style="{ color: s.id === currentSessionId ? '#e6edf3' : '#c9d1d9' }">
              {{ s.id.slice(0, 8) }}
            </div>
            <div style="font-size:11px;color:#7d8590;margin-top:2px">{{ s.message_count }} msgs</div>
          </div>
          <n-button
            text
            size="tiny"
            style="color:#7d8590;flex-shrink:0"
            @click.stop="deleteSession(s.id)"
          >
            🗑
          </n-button>
        </div>
      </div>
    </div>

    <!-- Chat area -->
    <div style="flex:1;display:flex;flex-direction:column;overflow:hidden;min-width:0">
      <!-- Messages -->
      <div
        ref="messagesEl"
        style="flex:1;overflow-y:auto;padding:16px;background:#0d1117;display:flex;flex-direction:column;gap:12px"
      >
        <div v-if="messages.length === 0" style="flex:1;display:flex;align-items:center;justify-content:center">
          <div style="text-align:center;color:#7d8590">
            <div style="font-size:32px;margin-bottom:12px">💬</div>
            <div style="font-size:15px">Start a conversation...</div>
            <div v-if="currentSessionId" style="font-size:12px;margin-top:6px;font-family:monospace">
              Session: {{ currentSessionId.slice(0, 8) }}
            </div>
          </div>
        </div>

        <div
          v-for="(msg, idx) in messages"
          :key="idx"
          :style="msg.role === 'user' ? 'display:flex;justify-content:flex-end' : 'display:flex;justify-content:flex-start'"
        >
          <div
            :style="bubbleStyle(msg.role)"
            style="max-width:75%;padding:10px 14px;border-radius:12px;font-size:14px;line-height:1.6;white-space:pre-wrap;word-break:break-word"
          >
            {{ msg.content }}<span v-if="streaming && idx === messages.length - 1 && msg.role === 'assistant'" style="opacity:0.7;animation:blink 1s infinite">▋</span>
          </div>
        </div>
      </div>

      <!-- Input area -->
      <div style="padding:12px 16px;border-top:1px solid #30363d;background:#161b22;display:flex;gap:8px;align-items:flex-end">
        <n-input
          v-model:value="inputText"
          type="textarea"
          :autosize="{ minRows: 1, maxRows: 5 }"
          placeholder="Message..."
          style="flex:1"
          :disabled="streaming"
          @keydown.enter.exact.prevent="sendMessage"
          @keydown.enter.shift.exact="() => {}"
        />
        <n-button
          v-if="!streaming"
          type="primary"
          :disabled="!inputText.trim()"
          @click="sendMessage"
          style="flex-shrink:0"
        >
          Send
        </n-button>
        <n-button
          v-else
          type="error"
          @click="stopStream"
          style="flex-shrink:0"
        >
          Stop
        </n-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, onMounted } from 'vue'
import { NButton, NInput, NSpin } from 'naive-ui'
import { useApi } from '@shared/composables/useApi'
import { useSse } from '@shared/composables/useSse'
import type { ChatMessage, Session, SessionListResponse } from '@shared/types/index'

const { get, del } = useApi()
const { stream, abort, loading: streaming } = useSse()

const sessions = ref<Session[]>([])
const sessionsLoading = ref(false)
const currentSessionId = ref<string | null>(null)
const messages = ref<ChatMessage[]>([])
const inputText = ref('')
const messagesEl = ref<HTMLElement | null>(null)

let currentAssistantContent = ''

function sessionItemStyle(active: boolean) {
  return active
    ? { background: 'rgba(46,160,67,0.08)', borderLeft: '2px solid #2ea043' }
    : { background: 'transparent', borderLeft: '2px solid transparent' }
}

function bubbleStyle(role: string) {
  return role === 'user'
    ? { background: '#2ea043', color: '#fff', borderBottomRightRadius: '4px' }
    : { background: '#1c2128', color: '#e6edf3', borderBottomLeftRadius: '4px' }
}

async function loadSessions() {
  sessionsLoading.value = true
  try {
    const data = await get<SessionListResponse>('/v1/sessions')
    sessions.value = (data?.sessions ?? []).sort(
      (a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime()
    )
  } catch {
    sessions.value = []
  } finally {
    sessionsLoading.value = false
  }
}

function newSession() {
  currentSessionId.value = null
  messages.value = []
  inputText.value = ''
}

function selectSession(id: string) {
  if (id === currentSessionId.value) return
  currentSessionId.value = id
  messages.value = []
}

async function deleteSession(id: string) {
  try {
    await del(`/v1/sessions/${id}`)
    if (currentSessionId.value === id) {
      currentSessionId.value = null
      messages.value = []
    }
    await loadSessions()
  } catch {
    // ignore
  }
}

function stopStream() {
  abort()
}

async function scrollToBottom() {
  await nextTick()
  if (messagesEl.value) {
    messagesEl.value.scrollTop = messagesEl.value.scrollHeight
  }
}

async function sendMessage() {
  const text = inputText.value.trim()
  if (!text || streaming.value) return

  inputText.value = ''

  messages.value.push({ role: 'user', content: text })
  await scrollToBottom()

  const assistantMsg: ChatMessage = { role: 'assistant', content: '' }
  messages.value.push(assistantMsg)
  currentAssistantContent = ''
  await scrollToBottom()

  // Build history excluding the empty placeholder
  const history = messages.value.slice(0, -1).map(m => ({ role: m.role, content: m.content }))

  await stream(
    'default',
    history,
    {
      sessionId: currentSessionId.value ?? undefined,
      onToken: (token: string) => {
        currentAssistantContent += token
        assistantMsg.content = currentAssistantContent
        scrollToBottom()
      },
      onDone: (newSessionId: string | null) => {
        if (newSessionId && !currentSessionId.value) {
          currentSessionId.value = newSessionId
        }
        loadSessions()
        scrollToBottom()
      },
      onError: (err: string) => {
        assistantMsg.content = assistantMsg.content || `Error: ${err}`
        scrollToBottom()
      },
    }
  )
}

onMounted(() => {
  loadSessions()
})
</script>

<style scoped>
@keyframes blink {
  0%, 100% { opacity: 1 }
  50% { opacity: 0 }
}
</style>
