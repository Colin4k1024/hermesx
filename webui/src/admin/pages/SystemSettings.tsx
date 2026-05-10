import { useEffect, useState } from 'react'
import { Typography, Card, Descriptions, Tag, Spin } from 'antd'
import { apiClient } from '@shared/api/client'
import type { BootstrapStatusResponse } from '@shared/types'

const { Title } = Typography

export default function SystemSettings() {
  const [status, setStatus] = useState<BootstrapStatusResponse | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    apiClient.get<BootstrapStatusResponse>('/admin/v1/bootstrap/status', { asAdmin: true })
      .then(setStatus)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <Spin style={{ display: 'block', padding: 48 }} />

  return (
    <div style={{ padding: 24 }}>
      <Title level={4}>System Settings</Title>
      <Card>
        <Descriptions column={1} size="middle">
          <Descriptions.Item label="Bootstrap Status">
            <Tag color={status?.bootstrap_required ? 'orange' : 'green'}>
              {status?.bootstrap_required ? 'Required' : 'Completed'}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="API Version">v1</Descriptions.Item>
          <Descriptions.Item label="WebUI Version">3.0.0</Descriptions.Item>
        </Descriptions>
      </Card>
    </div>
  )
}
