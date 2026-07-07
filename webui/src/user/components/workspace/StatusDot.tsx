import type { TaskSession } from '@shared/stores/workspaceStore'

interface Props {
  status: TaskSession['status']
  size?: number
  className?: string
}

const statusColors: Record<TaskSession['status'], string> = {
  running: 'var(--ant-color-primary)',
  completed: 'var(--ant-color-success)',
  failed: 'var(--ant-color-error)',
  pending: 'var(--ant-color-text-tertiary)',
}

const statusLabels: Record<TaskSession['status'], string> = {
  running: '运行中',
  completed: '已完成',
  failed: '失败',
  pending: '等待中',
}

export function StatusDot({ status, size = 8, className }: Props) {
  const isRunning = status === 'running'
  const isPending = status === 'pending'

  return (
    <span
      role="img"
      aria-label={statusLabels[status]}
      className={className}
      style={{
        display: 'inline-block',
        width: size,
        height: size,
        borderRadius: '50%',
        backgroundColor: isPending ? 'transparent' : statusColors[status],
        border: isPending ? `1.5px solid ${statusColors.pending}` : 'none',
        flexShrink: 0,
        animation: isRunning ? 'statusPulse 1.5s ease-in-out infinite' : 'none',
      }}
    />
  )
}
