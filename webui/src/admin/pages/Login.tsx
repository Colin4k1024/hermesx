import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Input, Typography, Space, message } from 'antd'
import { Shield } from 'lucide-react'
import { useAuthStore } from '@shared/stores/authStore'
import type { BootstrapStatusResponse } from '@shared/types'

const { Title, Text } = Typography

export default function Login() {
  const [apiKey, setApiKey] = useState('')
  const [loading, setLoading] = useState(false)
  const connectAdmin = useAuthStore((s) => s.connectAdmin)
  const navigate = useNavigate()

  useEffect(() => {
    fetch('/admin/v1/bootstrap/status')
      .then((r) => r.json())
      .then((data: BootstrapStatusResponse) => {
        if (data.bootstrap_required) navigate('/bootstrap', { replace: true })
      })
      .catch(() => {})
  }, [navigate])

  const handleConnect = async () => {
    if (!apiKey.trim()) {
      message.warning('Please enter your Admin API Key')
      return
    }
    setLoading(true)
    const ok = await connectAdmin(apiKey.trim())
    setLoading(false)
    if (ok) {
      navigate('/dashboard', { replace: true })
    } else {
      message.error('Invalid admin key or insufficient permissions')
    }
  }

  return (
    <div className="flex items-center justify-center min-h-screen" style={{ background: 'var(--ant-color-bg-layout)' }}>
      <div style={{ width: 380, padding: 40 }}>
        <Space direction="vertical" size="large" style={{ width: '100%', textAlign: 'center' }}>
          <Shield size={40} strokeWidth={1.5} style={{ color: '#6366f1' }} />
          <Title level={3} style={{ margin: 0 }}>HermesX Admin</Title>
          <Text type="secondary">Enter your admin API key to continue</Text>
        </Space>
        <Space direction="vertical" size="middle" style={{ width: '100%', marginTop: 32 }}>
          <Input
            placeholder="Admin API Key"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            onPressEnter={handleConnect}
            size="large"
            type="password"
          />
          <Button type="primary" block size="large" loading={loading} onClick={handleConnect}>
            Sign In
          </Button>
        </Space>
      </div>
    </div>
  )
}
