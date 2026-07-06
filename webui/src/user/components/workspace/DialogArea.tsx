import { useState, useCallback } from 'react'
import { Button, Tooltip } from 'antd'
import { PanelRightClose, PanelRightOpen } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { useSseManager } from '@shared/hooks/useSseManager'
import { useWorkspaceStore, selectActiveSession } from '@shared/stores/workspaceStore'
import type { DisplayMessage } from '@shared/stores/workspaceStore'
import { MessageList } from '@shared/components/MessageList'
import { InputBar } from '@shared/components/InputBar'
import { StatusDot } from './StatusDot'
import { PlanSteps } from './PlanSteps'

export function DialogArea() {
  const [input, setInput] = useState('')
  const { startStream, stopStream } = useSseManager()
  const queryClient = useQueryClient()

  const activeSession = useWorkspaceStore(selectActiveSession)
  const activeSessionId = useWorkspaceStore((s) => s.activeSessionId)
  const streaming = useWorkspaceStore((s) => s.streaming)
  const resultsPanelCollapsed = useWorkspaceStore((s) => s.resultsPanelCollapsed)
  const toggleResultsPanel = useWorkspaceStore((s) => s.toggleResultsPanel)
  const addMessage = useWorkspaceStore((s) => s.addMessage)
  const upsertSession = useWorkspaceStore((s) => s.upsertSession)
  const switchSession = useWorkspaceStore((s) => s.switchSession)
  const addPlanSteps = useWorkspaceStore((s) => s.addPlanSteps)
  const updatePlanStep = useWorkspaceStore((s) => s.updatePlanStep)

  const messages = activeSession?.messages ?? []
  const activeIsStreaming = activeSessionId ? !!streaming[activeSessionId] : false

  const titleFromMessage = useCallback((content: string) => {
    const title = content.replace(/\s+/g, ' ').trim()
    if (!title) return '未命名任务'
    return title.length > 64 ? `${title.slice(0, 61)}...` : title
  }, [])

  const handleSend = useCallback(async () => {
    const content = input.trim()
    if (!content || activeIsStreaming) return
    setInput('')

    const sessionId = activeSessionId ?? crypto.randomUUID()
    const userMsg: DisplayMessage = { id: crypto.randomUUID(), role: 'user', content }
    const assistantMsg: DisplayMessage = { id: crypto.randomUUID(), role: 'assistant', content: '' }

    // If no active session, create one
    if (!activeSessionId) {
      upsertSession({
        id: sessionId,
        title: titleFromMessage(content),
        status: 'running',
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        messages: [userMsg, assistantMsg],
        artifacts: [],
      })
      switchSession(sessionId)
    } else {
      addMessage(sessionId, userMsg)
      addMessage(sessionId, assistantMsg)
    }

    // Start SSE stream for this session (runs in background, survives session switch)
    await startStream({
      sessionId,
      message: content,
      onToolResult: () => {
        queryClient.invalidateQueries({ queryKey: ['artifacts', sessionId] })
      },
      onPlanStart: (data) => {
        const steps = data.steps.map((s) => ({
          id: s.id,
          title: s.title,
          status: 'pending' as const,
        }))
        addPlanSteps(sessionId, steps)
      },
      onPlanStepUpdate: (data) => {
        const validStatus = data.status === 'running' || data.status === 'completed' || data.status === 'failed'
        if (validStatus) {
          updatePlanStep(sessionId, data.step_id, data.status as 'running' | 'completed' | 'failed')
        }
      },
    })
  }, [input, activeIsStreaming, activeSessionId, addMessage, upsertSession, switchSession, addPlanSteps, updatePlanStep, titleFromMessage, startStream, queryClient])

  const handleAbort = useCallback(() => {
    if (activeSessionId) {
      stopStream(activeSessionId)
    }
  }, [activeSessionId, stopStream])

  return (
    <main
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        background: 'var(--ant-color-bg-container)',
      }}
    >
      {/* Top Bar */}
      <div
        style={{
          height: 48,
          padding: '0 16px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderBottom: '1px solid var(--ant-color-border-secondary)',
          flexShrink: 0,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
          {activeSession ? (
            <>
              <StatusDot status={activeSession.status} />
              <span
                style={{
                  fontSize: 14,
                  fontWeight: 500,
                  color: 'var(--ant-color-text)',
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                }}
              >
                {activeSession.title}
              </span>
            </>
          ) : (
            <span style={{ fontSize: 14, color: 'var(--ant-color-text-tertiary)' }}>
              选择或创建一个任务开始
            </span>
          )}
        </div>
        <Tooltip title={resultsPanelCollapsed ? '显示结果面板' : '隐藏结果面板'}>
          <Button
            type="text"
            size="small"
            icon={
              resultsPanelCollapsed ? (
                <PanelRightOpen size={16} />
              ) : (
                <PanelRightClose size={16} />
              )
            }
            onClick={toggleResultsPanel}
            aria-label="切换结果面板"
          />
        </Tooltip>
      </div>

      {/* Plan Steps */}
      {activeSessionId && (
        <div style={{ padding: '8px 16px 0', flexShrink: 0 }}>
          <PlanSteps sessionId={activeSessionId} />
        </div>
      )}

      {/* Messages */}
      <MessageList messages={messages} loading={activeIsStreaming} />

      {/* Input */}
      <InputBar
        value={input}
        onChange={setInput}
        onSend={handleSend}
        loading={activeIsStreaming}
        onAbort={handleAbort}
        placeholder={activeSession ? '继续对话...' : '描述你的任务...'}
      />
    </main>
  )
}
