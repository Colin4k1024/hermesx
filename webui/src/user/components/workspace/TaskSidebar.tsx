import { useEffect, useCallback, useState } from 'react'
import { Button, Input, Tooltip, message } from 'antd'
import { Briefcase, Plus, ClipboardList, RefreshCw } from 'lucide-react'
import { useWorkspaceStore, selectGroupedSessions } from '@shared/stores/workspaceStore'
import { apiClient } from '@shared/api/client'
import type { SessionListResponse } from '@shared/types'
import { StatusDot } from './StatusDot'

export function TaskSidebar() {
  const activeSessionId = useWorkspaceStore((s) => s.activeSessionId)
  const streaming = useWorkspaceStore((s) => s.streaming)
  const searchQuery = useWorkspaceStore((s) => s.searchQuery)
  const setSearchQuery = useWorkspaceStore((s) => s.setSearchQuery)
  const setSessions = useWorkspaceStore((s) => s.setSessions)
  const switchSession = useWorkspaceStore((s) => s.switchSession)
  const grouped = useWorkspaceStore(selectGroupedSessions)
  const [fetchError, setFetchError] = useState(false)

  const fetchSessions = useCallback(() => {
    setFetchError(false)
    apiClient
      .get<SessionListResponse>('/v1/sessions')
      .then((data) => setSessions(data.sessions ?? []))
      .catch(() => {
        message.error('获取会话列表失败')
        setFetchError(true)
      })
  }, [setSessions])

  // Fetch sessions on mount
  useEffect(() => {
    fetchSessions()
  }, [fetchSessions])

  const handleNewTask = useCallback(() => {
    switchSession(null)
  }, [switchSession])

  const hasAnySession =
    grouped.today.length > 0 ||
    grouped.thisWeek.length > 0 ||
    grouped.earlier.length > 0

  const formatTime = (dateStr: string) => {
    const d = new Date(dateStr)
    const now = new Date()
    const isToday =
      d.getFullYear() === now.getFullYear() &&
      d.getMonth() === now.getMonth() &&
      d.getDate() === now.getDate()
    if (isToday) {
      return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
    }
    return d.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
  }

  const renderGroup = (label: string, sessions: typeof grouped.today) => {
    if (sessions.length === 0) return null
    return (
      <div key={label}>
        <div
          style={{
            fontSize: 12,
            fontWeight: 500,
            color: 'var(--ant-color-text-tertiary)',
            padding: '8px 16px 4px',
            letterSpacing: '0.02em',
          }}
        >
          {label}
        </div>
        {sessions.map((s) => (
          <div
            key={s.id}
            role="button"
            tabIndex={0}
            onClick={() => switchSession(s.id)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                switchSession(s.id)
              }
            }}
            style={{
              padding: '8px 16px',
              borderRadius: 6,
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'flex-start',
              gap: 8,
              minHeight: 52,
              background:
                s.id === activeSessionId
                  ? 'var(--ant-color-bg-elevated)'
                  : 'transparent',
              borderLeft:
                s.id === activeSessionId
                  ? '3px solid var(--ant-color-primary)'
                  : '3px solid transparent',
              transition: 'background 150ms ease',
            }}
          >
            <StatusDot status={s.status} className="mt-[5px]" />
            <div style={{ flex: 1, minWidth: 0 }}>
              <div
                style={{
                  fontSize: 13,
                  fontWeight: 400,
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  color: 'var(--ant-color-text)',
                }}
              >
                {s.title || s.id.slice(0, 12)}
              </div>
              <div
                style={{
                  fontSize: 11,
                  color: 'var(--ant-color-text-tertiary)',
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                }}
              >
                {streaming[s.id]
                  ? '生成中...'
                  : s.status === 'running'
                    ? '处理中...'
                    : `${s.messages.length} 条消息`}
              </div>
            </div>
            <span
              style={{
                fontSize: 11,
                color: 'var(--ant-color-text-tertiary)',
                flexShrink: 0,
                marginTop: 2,
              }}
            >
              {formatTime(s.createdAt)}
            </span>
          </div>
        ))}
      </div>
    )
  }

  return (
    <nav
      aria-label="任务列表"
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        background: 'var(--ant-color-bg-container)',
      }}
    >
      {/* Header */}
      <div
        style={{
          height: 48,
          padding: '12px 16px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderBottom: '1px solid var(--ant-color-border-secondary)',
          flexShrink: 0,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Briefcase size={16} style={{ color: 'var(--ant-color-text-secondary)' }} />
          <span style={{ fontSize: 15, fontWeight: 600, color: 'var(--ant-color-text)' }}>
            工作区
          </span>
        </div>
        <Tooltip title="新建任务 (Cmd+N)">
          <Button
            type="text"
            size="middle"
            icon={<Plus size={16} />}
            onClick={handleNewTask}
            aria-label="新建任务"
          />
        </Tooltip>
      </div>

      {/* Search */}
      <div style={{ padding: '8px 12px', flexShrink: 0 }}>
        <Input
          prefix={
            <span style={{ color: 'var(--ant-color-text-tertiary)', fontSize: 13 }}>
              搜索任务...
            </span>
          }
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          allowClear
          size="small"
          style={{ borderRadius: 6 }}
        />
      </div>

      {/* Task list */}
      <div style={{ flex: 1, overflow: 'auto' }}>
        {fetchError ? (
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              padding: 48,
              gap: 12,
            }}
          >
            <span style={{ fontSize: 13, color: 'var(--ant-color-text-tertiary)', textAlign: 'center' }}>
              获取会话列表失败
            </span>
            <Button ghost size="small" icon={<RefreshCw size={14} />} onClick={fetchSessions}>
              重试
            </Button>
          </div>
        ) : hasAnySession ? (
          <>
            {renderGroup('今天', grouped.today)}
            {renderGroup('本周', grouped.thisWeek)}
            {renderGroup('更早', grouped.earlier)}
          </>
        ) : (
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              padding: 48,
              gap: 12,
            }}
          >
            <ClipboardList size={48} style={{ color: 'var(--ant-color-text-tertiary)' }} />
            <span style={{ fontSize: 13, color: 'var(--ant-color-text-tertiary)', textAlign: 'center' }}>
              还没有任务，点击上方按钮创建
            </span>
            <Button ghost size="middle" icon={<Plus size={14} />} onClick={handleNewTask}>
              新建任务
            </Button>
          </div>
        )}
      </div>

      {/* Pulse animation keyframes */}
      <style>{`
        @keyframes statusPulse {
          0%, 100% { transform: scale(1); opacity: 1; }
          50% { transform: scale(1.4); opacity: 0.6; }
        }
        @media (prefers-reduced-motion: reduce) {
          @keyframes statusPulse {
            0%, 100% { transform: scale(1); opacity: 1; }
          }
        }
      `}</style>
    </nav>
  )
}
