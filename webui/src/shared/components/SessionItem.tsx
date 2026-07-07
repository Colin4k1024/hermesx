import { List, Space, Typography } from 'antd'
import { MessageSquare } from 'lucide-react'
import type { Session } from '@shared/types'

const { Text } = Typography

interface Props {
  session: Session
  active: boolean
  onClick: () => void
}

export function SessionItem({ session, active, onClick }: Props) {
  return (
    <List.Item
      onClick={onClick}
      style={{
        cursor: 'pointer',
        padding: '8px 12px',
        borderRadius: 6,
        background: active
          ? 'var(--ant-color-bg-elevated)'
          : 'transparent',
        border: 'none',
      }}
    >
      <Space size={8}>
        <MessageSquare size={14} style={{ opacity: 0.5 }} />
        <Text ellipsis style={{ fontSize: 13, maxWidth: 160 }}>
          {session.title || session.id.slice(0, 12)}
        </Text>
      </Space>
    </List.Item>
  )
}
