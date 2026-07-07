import { useState, useEffect } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { Button, Input, Select, message, Typography, Space, Divider, Radio } from 'antd'
import { UserOutlined, LockOutlined, TeamOutlined, PlusOutlined } from '@ant-design/icons'
import { useAuthStore } from '@shared/stores/authStore'

const { Title, Text } = Typography

interface Tenant {
  id: string
  name: string
}

export default function Register() {
  const navigate = useNavigate()
  const register = useAuthStore((s) => s.register)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [tenantMode, setTenantMode] = useState<'existing' | 'new'>('existing')
  const [selectedTenantId, setSelectedTenantId] = useState('')
  const [newTenantName, setNewTenantName] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    fetch('/auth/tenants')
      .then((res) => res.json())
      .then((data) => {
        setTenants(data.tenants ?? [])
        if (data.tenants?.length > 0) {
          setSelectedTenantId(data.tenants[0].id)
        }
      })
      .catch(() => {})
  }, [])

  const handleSubmit = async () => {
    if (!username || !password) {
      message.error('Please fill in username and password')
      return
    }
    if (password !== confirmPassword) {
      message.error('Passwords do not match')
      return
    }
    if (password.length < 8) {
      message.error('Password must be at least 8 characters')
      return
    }
    if (tenantMode === 'existing' && !selectedTenantId) {
      message.error('Please select a tenant')
      return
    }
    if (tenantMode === 'new' && !newTenantName) {
      message.error('Please enter a tenant name')
      return
    }

    setLoading(true)
    try {
      const ok = await register(
        username,
        password,
        displayName || username,
        tenantMode === 'existing' ? selectedTenantId : undefined,
        tenantMode === 'new' ? newTenantName : undefined,
      )
      if (ok) {
        message.success('Registration successful! Please log in.')
        navigate('/login')
      } else {
        message.error('Registration failed. Username may already be taken.')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', background: '#f5f5f5' }}>
      <div style={{ background: '#fff', padding: 40, borderRadius: 12, width: 400, boxShadow: '0 2px 12px rgba(0,0,0,0.08)' }}>
        <Title level={3} style={{ textAlign: 'center', marginBottom: 32 }}>Create Account</Title>

        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Input
            prefix={<UserOutlined />}
            placeholder="Username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            size="large"
          />
          <Input
            prefix={<UserOutlined />}
            placeholder="Display Name (optional)"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            size="large"
          />
          <Input.Password
            prefix={<LockOutlined />}
            placeholder="Password (min 8 characters)"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            size="large"
          />
          <Input.Password
            prefix={<LockOutlined />}
            placeholder="Confirm Password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            size="large"
          />

          <Divider style={{ margin: '8px 0' }}>Tenant</Divider>

          <Radio.Group
            value={tenantMode}
            onChange={(e) => setTenantMode(e.target.value)}
            style={{ width: '100%' }}
          >
            <Space direction="vertical" style={{ width: '100%' }}>
              <Radio value="existing">
                <TeamOutlined /> Join existing tenant
              </Radio>
              {tenantMode === 'existing' && (
                <Select
                  placeholder="Select a tenant"
                  value={selectedTenantId || undefined}
                  onChange={setSelectedTenantId}
                  style={{ width: '100%' }}
                  size="large"
                  options={tenants.map((t) => ({ label: t.name, value: t.id }))}
                />
              )}
              <Radio value="new">
                <PlusOutlined /> Create new tenant
              </Radio>
              {tenantMode === 'new' && (
                <Input
                  placeholder="New tenant name"
                  value={newTenantName}
                  onChange={(e) => setNewTenantName(e.target.value)}
                  size="large"
                />
              )}
            </Space>
          </Radio.Group>

          <Button
            type="primary"
            size="large"
            block
            loading={loading}
            onClick={handleSubmit}
          >
            Register
          </Button>

          <Text style={{ textAlign: 'center', display: 'block' }}>
            Already have an account? <Link to="/login">Log in</Link>
          </Text>
        </Space>
      </div>
    </div>
  )
}
