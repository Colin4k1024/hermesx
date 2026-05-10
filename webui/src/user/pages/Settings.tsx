import { useEffect, useState } from 'react'
import { Typography, Card, Descriptions, Switch, Space, Button, message } from 'antd'
import { useAuthStore } from '@shared/stores/authStore'
import { useThemeStore } from '@shared/stores/themeStore'
import { apiClient } from '@shared/api/client'
import type { MeResponse } from '@shared/types'

const { Title } = Typography

export default function Settings() {
  const { userId, tenantId } = useAuthStore()
  const { mode, toggle } = useThemeStore()
  const [me, setMe] = useState<MeResponse | null>(null)

  useEffect(() => {
    apiClient.get<MeResponse>('/v1/me').then(setMe).catch(() => {})
  }, [])

  return (
    <div style={{ padding: 24 }}>
      <Title level={4}>Settings</Title>
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <Card title="Profile">
          <Descriptions column={1} size="small">
            <Descriptions.Item label="User ID">{userId}</Descriptions.Item>
            <Descriptions.Item label="Tenant ID">{tenantId}</Descriptions.Item>
            <Descriptions.Item label="Auth Method">{me?.auth_method ?? '—'}</Descriptions.Item>
            <Descriptions.Item label="Plan">{me?.plan ?? 'default'}</Descriptions.Item>
            <Descriptions.Item label="Rate Limit">{me?.rate_limit_rpm ?? '—'} RPM</Descriptions.Item>
          </Descriptions>
        </Card>
        <Card title="Appearance">
          <Space>
            <span>Dark Mode</span>
            <Switch checked={mode === 'dark'} onChange={toggle} />
          </Space>
        </Card>
        <Card title="Session">
          <Button danger onClick={() => { useAuthStore.getState().disconnectUser(); message.info('Disconnected') }}>
            Disconnect
          </Button>
        </Card>
      </Space>
    </div>
  )
}
