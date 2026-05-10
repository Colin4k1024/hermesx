import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Input, Typography, Space, Alert, message } from 'antd'
import { KeyRound } from 'lucide-react'
import type { ApiKeyCreateResponse } from '@shared/types'

const { Title, Text, Paragraph } = Typography

export default function Bootstrap() {
  const [acpToken, setAcpToken] = useState('')
  const [keyName, setKeyName] = useState('admin-initial')
  const [createdKey, setCreatedKey] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const handleBootstrap = async () => {
    if (!acpToken.trim()) {
      message.warning('ACP Token is required')
      return
    }
    setLoading(true)
    try {
      const res = await fetch('/admin/v1/bootstrap', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${acpToken.trim()}` },
        body: JSON.stringify({ name: keyName.trim() || 'admin-initial' }),
      })
      if (!res.ok) {
        const text = await res.text()
        message.error(`Bootstrap failed: ${text}`)
        return
      }
      const data: ApiKeyCreateResponse = await res.json()
      setCreatedKey(data.key)
      message.success('Admin key created successfully')
    } catch (e) {
      message.error(e instanceof Error ? e.message : 'Bootstrap failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex items-center justify-center min-h-screen" style={{ background: 'var(--ant-color-bg-layout)' }}>
      <div style={{ width: 440, padding: 40 }}>
        <Space direction="vertical" size="large" style={{ width: '100%', textAlign: 'center' }}>
          <KeyRound size={40} strokeWidth={1.5} style={{ color: '#6366f1' }} />
          <Title level={3} style={{ margin: 0 }}>System Bootstrap</Title>
          <Text type="secondary">Create your first admin API key</Text>
        </Space>

        {createdKey ? (
          <Space direction="vertical" size="middle" style={{ width: '100%', marginTop: 32 }}>
            <Alert
              type="success"
              message="Admin key created"
              description="Save this key securely. It will not be shown again."
              showIcon
            />
            <Input.TextArea value={createdKey} readOnly rows={2} style={{ fontFamily: 'monospace' }} />
            <Paragraph type="secondary" style={{ fontSize: 12 }}>
              Copy the key above, then sign in with it.
            </Paragraph>
            <Button type="primary" block onClick={() => navigate('/login', { replace: true })}>
              Go to Login
            </Button>
          </Space>
        ) : (
          <Space direction="vertical" size="middle" style={{ width: '100%', marginTop: 32 }}>
            <Input
              placeholder="ACP Token (HERMES_ACP_TOKEN)"
              value={acpToken}
              onChange={(e) => setAcpToken(e.target.value)}
              size="large"
              type="password"
            />
            <Input
              placeholder="Key name (default: admin-initial)"
              value={keyName}
              onChange={(e) => setKeyName(e.target.value)}
              size="large"
            />
            <Button type="primary" block size="large" loading={loading} onClick={handleBootstrap}>
              Create Admin Key
            </Button>
          </Space>
        )}
      </div>
    </div>
  )
}
