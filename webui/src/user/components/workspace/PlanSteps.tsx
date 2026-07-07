import { useEffect, useRef, useState } from 'react'
import { Collapse, Tag } from 'antd'
import {
  ClockCircleOutlined,
  SyncOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
} from '@ant-design/icons'
import { useWorkspaceStore, type PlanStep } from '@shared/stores/workspaceStore'

const statusConfig: Record<
  PlanStep['status'],
  { icon: React.ReactNode; color: string; label: string }
> = {
  pending: {
    icon: <ClockCircleOutlined />,
    color: 'default',
    label: '等待中',
  },
  running: {
    icon: <SyncOutlined spin />,
    color: 'processing',
    label: '执行中',
  },
  completed: {
    icon: <CheckCircleOutlined />,
    color: 'success',
    label: '已完成',
  },
  failed: {
    icon: <CloseCircleOutlined />,
    color: 'error',
    label: '失败',
  },
}

function formatDuration(startedAt?: number, completedAt?: number): string {
  if (!startedAt || !completedAt) return ''
  const ms = completedAt - startedAt
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

// Stable empty array to avoid re-renders from new reference creation
const EMPTY_STEPS: PlanStep[] = []

interface PlanStepsProps {
  sessionId: string
}

export function PlanSteps({ sessionId }: PlanStepsProps) {
  const planSteps = useWorkspaceStore((s) => s.planSteps[sessionId] ?? EMPTY_STEPS)
  const [collapsed, setCollapsed] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Auto-collapse 5 seconds after all steps are completed or failed
  useEffect(() => {
    if (planSteps.length === 0) return
    const allDone = planSteps.every(
      (s) => s.status === 'completed' || s.status === 'failed',
    )
    if (allDone) {
      timerRef.current = setTimeout(() => setCollapsed(true), 5000)
    }
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current)
    }
  }, [planSteps])

  if (planSteps.length === 0) return null

  const runningCount = planSteps.filter((s) => s.status === 'running').length
  const completedCount = planSteps.filter((s) => s.status === 'completed').length
  const failedCount = planSteps.filter((s) => s.status === 'failed').length
  const allDone = planSteps.every(
    (s) => s.status === 'completed' || s.status === 'failed',
  )

  const header = (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13 }}>
      <span style={{ fontWeight: 500 }}>执行计划</span>
      <Tag color="blue">{planSteps.length} 步</Tag>
      {runningCount > 0 && <Tag color="processing">{runningCount} 执行中</Tag>}
      {completedCount > 0 && <Tag color="success">{completedCount} 已完成</Tag>}
      {failedCount > 0 && <Tag color="error">{failedCount} 失败</Tag>}
      {allDone && (
        <span style={{ color: 'var(--ant-color-text-tertiary)', marginLeft: 'auto' }}>
          全部完成
        </span>
      )}
    </div>
  )

  return (
    <Collapse
      activeKey={collapsed ? [] : ['plan']}
      onChange={(keys) => setCollapsed(!keys.includes('plan'))}
      size="small"
      style={{ marginBottom: 8 }}
      items={[
        {
          key: 'plan',
          label: header,
          children: (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {planSteps.map((step, idx) => {
                const cfg = statusConfig[step.status]
                const duration = formatDuration(step.startedAt, step.completedAt)
                return (
                  <div
                    key={step.id}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 8,
                      padding: '4px 0',
                      fontSize: 13,
                    }}
                  >
                    <span
                      style={{
                        width: 20,
                        textAlign: 'center',
                        color: 'var(--ant-color-text-secondary)',
                        fontSize: 12,
                      }}
                    >
                      {idx + 1}
                    </span>
                    <Tag
                      icon={cfg.icon}
                      color={cfg.color}
                      style={{ margin: 0, minWidth: 60, textAlign: 'center' }}
                    >
                      {cfg.label}
                    </Tag>
                    <span style={{ flex: 1 }}>{step.title}</span>
                    {duration && (
                      <span
                        style={{
                          color: 'var(--ant-color-text-tertiary)',
                          fontSize: 12,
                        }}
                      >
                        {duration}
                      </span>
                    )}
                  </div>
                )
              })}
            </div>
          ),
        },
      ]}
    />
  )
}
