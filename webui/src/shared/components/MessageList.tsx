import { useRef, useEffect } from 'react'
import { Spin } from 'antd'
import type { ChatMessage } from '@shared/types'

export interface DisplayMessage extends ChatMessage {
  id: string
}

interface Props {
  messages: DisplayMessage[]
  loading?: boolean
}

export function MessageList({ messages, loading }: Props) {
  const endRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  if (messages.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <span style={{ color: 'var(--ant-color-text-tertiary)', fontSize: 14 }}>
          选择或创建一个任务开始
        </span>
      </div>
    )
  }

  return (
    <div className="flex-1 overflow-y-auto" style={{ padding: '16px 24px' }}>
      {messages.map((msg) => (
        <div
          key={msg.id}
          style={{
            marginBottom: 12,
            display: 'flex',
            justifyContent: msg.role === 'user' ? 'flex-end' : 'flex-start',
          }}
        >
          <div
            style={{
              maxWidth: msg.role === 'user' ? '70%' : '80%',
              padding: '10px 14px',
              borderRadius:
                msg.role === 'user'
                  ? '12px 12px 4px 12px'
                  : '12px 12px 12px 4px',
              background:
                msg.role === 'user'
                  ? 'var(--ant-color-primary)'
                  : 'var(--ant-color-bg-elevated)',
              color:
                msg.role === 'user' ? '#fff' : 'var(--ant-color-text)',
              border:
                msg.role === 'user'
                  ? 'none'
                  : '1px solid var(--ant-color-border)',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
              fontSize: 14,
              lineHeight: 1.6,
            }}
          >
            {msg.content ||
              (loading && msg.role === 'assistant' ? <Spin size="small" /> : '')}
          </div>
        </div>
      ))}
      <div ref={endRef} />
    </div>
  )
}
