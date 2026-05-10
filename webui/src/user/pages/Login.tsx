import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Input, Typography, Space, message } from 'antd'
import { Bot } from 'lucide-react'
import { useAuthStore } from '@shared/stores/authStore'

const { Title, Text } = Typography

export default function Login() {
  const [apiKey, setApiKey] = useState('')
  const [userId, setUserId] = useState('')
  const [loading, setLoading] = useState(false)
  const connectUser = useAuthStore((s) => s.connectUser)
  const navigate = useNavigate()

  const handleConnect = async () => {
    if (!apiKey.trim() || !userId.trim()) {
      message.warning('Please enter both API Key and User ID')
      return
    }
    setLoading(true)
    const ok = await connectUser(apiKey.trim(), userId.trim())
    setLoading(false)
    if (ok) {
      navigate('/chat', { replace: true })
    } else {
      message.error('Invalid credentials')
    }
  }

  return (
    <div className="flex items-center justify-center min-h-screen" style={{ background: 'var(--ant-color-bg-layout)' }}>
      <div style={{ width: 380, padding: 40 }}>
        <Space direction="vertical" size="large" style={{ width: '100%', textAlign: 'center' }}>
          <Bot size={40} strokeWidth={1.5} style={{ color: '#6366f1' }} />
          <Title level={3} style={{ margin: 0 }}>HermesX</Title>
          <Text type="secondary">Connect to your AI agent</Text>
        </Space>
        <Space direction="vertical" size="middle" style={{ width: '100%', marginTop: 32 }}>
          <Input
            placeholder="API Key"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            onPressEnter={handleConnect}
            size="large"
            type="password"
          />
          <Input
            placeholder="User ID"
            value={userId}
            onChange={(e) => setUserId(e.target.value)}
            onPressEnter={handleConnect}
            size="large"
          />
          <Button type="primary" block size="large" loading={loading} onClick={handleConnect}>
            Connect
          </Button>
        </Space>
      </div>
    </div>
  )
}
