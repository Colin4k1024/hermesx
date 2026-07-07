import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { Button, Divider, Input, Typography, Space, message, Tabs } from 'antd'
import { Bot } from 'lucide-react'
import { UserOutlined, LockOutlined } from '@ant-design/icons'
import { useAuthStore } from '@shared/stores/authStore'

const { Title, Text } = Typography

const CHANNEL_PLATFORMS = [
  { key: 'feishu', label: 'Feishu / Lark' },
  { key: 'wecom', label: 'WeCom' },
  { key: 'weixin', label: 'WeChat' },
]

export default function Login() {
  // API Key login state
  const [apiKey, setApiKey] = useState('')
  const [userId, setUserId] = useState('')
  // Password login state
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [tenantId, setTenantId] = useState('')

  const [loading, setLoading] = useState(false)
  const connectUser = useAuthStore((s) => s.connectUser)
  const loginWithPassword = useAuthStore((s) => s.loginWithPassword)
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

  const handlePasswordLogin = async () => {
    if (!username.trim() || !password.trim()) {
      message.warning('Please enter username and password')
      return
    }
    setLoading(true)
    const ok = await loginWithPassword(username.trim(), password.trim(), tenantId.trim())
    setLoading(false)
    if (ok) {
      navigate('/chat', { replace: true })
    } else {
      message.error('Invalid credentials')
    }
  }

  const handleChannelLogin = (platform: string) => {
    window.location.assign(`/auth/channel/${platform}/start?return_to=/chat`)
  }

  const tabItems = [
    {
      key: 'password',
      label: 'Username & Password',
      children: (
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Input
            prefix={<UserOutlined />}
            placeholder="Username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            onPressEnter={handlePasswordLogin}
            size="large"
          />
          <Input.Password
            prefix={<LockOutlined />}
            placeholder="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            onPressEnter={handlePasswordLogin}
            size="large"
          />
          <Input
            placeholder="Tenant ID"
            value={tenantId}
            onChange={(e) => setTenantId(e.target.value)}
            onPressEnter={handlePasswordLogin}
            size="large"
          />
          <Button type="primary" block size="large" loading={loading} onClick={handlePasswordLogin}>
            Log In
          </Button>
          <Text style={{ textAlign: 'center', display: 'block' }}>
            Don't have an account? <Link to="/register">Register</Link>
          </Text>
        </Space>
      ),
    },
    {
      key: 'apikey',
      label: 'API Key',
      children: (
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
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
      ),
    },
  ]

  return (
    <div className="flex items-center justify-center min-h-screen" style={{ background: 'var(--ant-color-bg-layout)' }}>
      <div style={{ width: 400, padding: 40 }}>
        <Space direction="vertical" size="large" style={{ width: '100%', textAlign: 'center' }}>
          <Bot size={40} strokeWidth={1.5} style={{ color: '#6366f1' }} />
          <Title level={3} style={{ margin: 0 }}>HermesX</Title>
          <Text type="secondary">Connect to your AI agent</Text>
        </Space>

        <div style={{ marginTop: 32 }}>
          <Tabs items={tabItems} centered />
        </div>

        <Divider plain style={{ margin: '16px 0' }}>
          <Text type="secondary" style={{ fontSize: 12 }}>or sign in with</Text>
        </Divider>

        <Space direction="vertical" size="small" style={{ width: '100%' }}>
          {CHANNEL_PLATFORMS.map(({ key, label }) => (
            <Button
              key={key}
              block
              size="large"
              onClick={() => handleChannelLogin(key)}
            >
              {label}
            </Button>
          ))}
        </Space>
      </div>
    </div>
  )
}
