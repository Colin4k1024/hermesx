import { useState, useRef, useEffect, useCallback } from 'react'
import { Button, Input, List, Typography, Space, Spin } from 'antd'
import { Send, Square, Plus, MessageSquare } from 'lucide-react'
import { useSse } from '@shared/hooks/useSse'
import { apiClient } from '@shared/api/client'
import type { ChatMessage, SessionListResponse, SessionDetailResponse } from '@shared/types'

const { Text } = Typography
const { TextArea } = Input

interface DisplayMessage extends ChatMessage {
  id: string
}

export default function Chat() {
  const [messages, setMessages] = useState<DisplayMessage[]>([])
  const [input, setInput] = useState('')
  const [sessionId, setSessionId] = useState<string | null>(null)
  const [sessions, setSessions] = useState<{ id: string; started_at: string; message_count: number }[]>([])
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const streamingRef = useRef('')
  const { loading, stream, abort } = useSse()

  useEffect(() => {
    apiClient.get<SessionListResponse>('/v1/sessions').then((data) => {
      setSessions(data.sessions ?? [])
    }).catch(() => {})
  }, [])

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [])

  useEffect(scrollToBottom, [messages, scrollToBottom])

  const loadSession = async (sid: string) => {
    try {
      const data = await apiClient.get<SessionDetailResponse>(`/v1/sessions/${sid}`)
      setSessionId(sid)
      setMessages(
        (data.messages ?? []).map((m, i) => ({ ...m, id: `${sid}-${i}` }))
      )
    } catch { /* ignore */ }
  }

  const handleNewChat = () => {
    setSessionId(null)
    setMessages([])
  }

  const handleSend = async () => {
    const content = input.trim()
    if (!content || loading) return
    setInput('')

    const userMsg: DisplayMessage = { id: crypto.randomUUID(), role: 'user', content }
    const assistantMsg: DisplayMessage = { id: crypto.randomUUID(), role: 'assistant', content: '' }
    setMessages((prev) => [...prev, userMsg, assistantMsg])
    streamingRef.current = ''

    const history = [...messages, userMsg].map(({ role, content: c }) => ({ role, content: c }))

    await stream('default', history, {
      sessionId: sessionId ?? undefined,
      onToken: (token) => {
        streamingRef.current += token
        setMessages((prev) => {
          const updated = [...prev]
          const last = updated[updated.length - 1]
          if (last) updated[updated.length - 1] = { ...last, content: streamingRef.current }
          return updated
        })
      },
      onDone: (newSid) => {
        if (newSid && !sessionId) {
          setSessionId(newSid)
          setSessions((prev) => [{ id: newSid, started_at: new Date().toISOString(), message_count: 2 }, ...prev])
        }
      },
      onError: (msg) => {
        setMessages((prev) => {
          const updated = [...prev]
          const last = updated[updated.length - 1]
          if (last) updated[updated.length - 1] = { ...last, content: `Error: ${msg}` }
          return updated
        })
      },
    })
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  return (
    <div style={{ display: 'flex', height: '100vh' }}>
      {/* Session Sidebar */}
      <div style={{ width: 240, borderRight: '1px solid var(--ant-color-border)', display: 'flex', flexDirection: 'column' }}>
        <div style={{ padding: 12 }}>
          <Button block icon={<Plus size={14} />} onClick={handleNewChat}>New Chat</Button>
        </div>
        <div style={{ flex: 1, overflow: 'auto', padding: '0 8px' }}>
          <List
            dataSource={sessions}
            renderItem={(s) => (
              <List.Item
                onClick={() => loadSession(s.id)}
                style={{
                  cursor: 'pointer',
                  padding: '8px 12px',
                  borderRadius: 6,
                  background: s.id === sessionId ? 'var(--ant-color-bg-elevated)' : 'transparent',
                  border: 'none',
                }}
              >
                <Space size={8}>
                  <MessageSquare size={14} style={{ opacity: 0.5 }} />
                  <Text ellipsis style={{ fontSize: 13, maxWidth: 160 }}>{s.id.slice(0, 12)}</Text>
                </Space>
              </List.Item>
            )}
          />
        </div>
      </div>

      {/* Chat Area */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
        <div style={{ flex: 1, overflow: 'auto', padding: 24 }}>
          {messages.map((msg) => (
            <div
              key={msg.id}
              style={{
                marginBottom: 16,
                display: 'flex',
                justifyContent: msg.role === 'user' ? 'flex-end' : 'flex-start',
              }}
            >
              <div
                style={{
                  maxWidth: '70%',
                  padding: '10px 14px',
                  borderRadius: 12,
                  background: msg.role === 'user' ? '#6366f1' : 'var(--ant-color-bg-container)',
                  color: msg.role === 'user' ? '#fff' : 'var(--ant-color-text)',
                  border: msg.role === 'user' ? 'none' : '1px solid var(--ant-color-border)',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                  fontSize: 14,
                  lineHeight: 1.6,
                }}
              >
                {msg.content || (loading && msg.role === 'assistant' ? <Spin size="small" /> : '')}
              </div>
            </div>
          ))}
          <div ref={messagesEndRef} />
        </div>

        {/* Input Area */}
        <div style={{ padding: '12px 24px', borderTop: '1px solid var(--ant-color-border)' }}>
          <Space.Compact style={{ width: '100%' }}>
            <TextArea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Type a message..."
              autoSize={{ minRows: 1, maxRows: 4 }}
              style={{ borderRadius: '8px 0 0 8px' }}
            />
            {loading ? (
              <Button icon={<Square size={16} />} onClick={abort} danger style={{ borderRadius: '0 8px 8px 0' }} />
            ) : (
              <Button type="primary" icon={<Send size={16} />} onClick={handleSend} style={{ borderRadius: '0 8px 8px 0' }} />
            )}
          </Space.Compact>
        </div>
      </div>
    </div>
  )
}
