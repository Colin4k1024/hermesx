import { useState, useEffect } from 'react'
import { Layout, Menu, Button, Tooltip, Drawer } from 'antd'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import {
  LayoutDashboard,
  Building2,
  Users,
  KeyRound,
  ScrollText,
  DollarSign,
  Shield,
  Settings,
  LogOut,
  Sun,
  Moon,
  MenuIcon,
} from 'lucide-react'
import { useAuthStore } from '@shared/stores/authStore'
import { useThemeStore } from '@shared/stores/themeStore'
import { SIDEBAR_WIDTH } from '@shared/theme/constants'

const { Sider, Content, Header } = Layout

const navItems = [
  { key: '/dashboard', icon: <LayoutDashboard size={18} />, label: 'Dashboard' },
  { key: '/tenants', icon: <Building2 size={18} />, label: 'Tenants' },
  { key: '/users', icon: <Users size={18} />, label: 'Users' },
  { key: '/keys', icon: <KeyRound size={18} />, label: 'API Keys' },
  { key: '/audit', icon: <ScrollText size={18} />, label: 'Audit Logs' },
  { key: '/pricing', icon: <DollarSign size={18} />, label: 'Pricing' },
  { key: '/sandbox', icon: <Shield size={18} />, label: 'Sandbox' },
  { key: '/settings', icon: <Settings size={18} />, label: 'Settings' },
]

function useIsMobile() {
  const [mobile, setMobile] = useState(window.innerWidth < 768)
  useEffect(() => {
    const handler = () => setMobile(window.innerWidth < 768)
    window.addEventListener('resize', handler)
    return () => window.removeEventListener('resize', handler)
  }, [])
  return mobile
}

function SidebarContent({ onNavigate }: { onNavigate?: () => void }) {
  const navigate = useNavigate()
  const location = useLocation()
  const disconnectAdmin = useAuthStore((s) => s.disconnectAdmin)
  const { mode, toggle } = useThemeStore()

  const handleNav = (key: string) => {
    navigate(key)
    onNavigate?.()
  }

  const handleLogout = () => {
    disconnectAdmin()
    navigate('/login', { replace: true })
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div style={{ padding: '16px 20px', display: 'flex', alignItems: 'center', gap: 10 }}>
        <Shield size={20} style={{ color: '#6366f1' }} />
        <span style={{ fontWeight: 600, fontSize: 15 }}>HermesX Admin</span>
      </div>
      <Menu
        mode="inline"
        selectedKeys={[location.pathname]}
        items={navItems}
        onClick={({ key }) => handleNav(key)}
        style={{ borderRight: 'none', flex: 1 }}
      />
      <div style={{ padding: '12px', display: 'flex', gap: 8 }}>
        <Tooltip title={mode === 'dark' ? 'Light mode' : 'Dark mode'}>
          <Button type="text" icon={mode === 'dark' ? <Sun size={16} /> : <Moon size={16} />} onClick={toggle} />
        </Tooltip>
        <Tooltip title="Sign out">
          <Button type="text" icon={<LogOut size={16} />} onClick={handleLogout} />
        </Tooltip>
      </div>
    </div>
  )
}

export default function AdminShell() {
  const isMobile = useIsMobile()
  const [drawerOpen, setDrawerOpen] = useState(false)

  if (isMobile) {
    return (
      <Layout style={{ minHeight: '100vh' }}>
        <Header style={{ padding: '0 16px', display: 'flex', alignItems: 'center', gap: 12, height: 48, lineHeight: '48px' }}>
          <Button type="text" icon={<MenuIcon size={18} />} onClick={() => setDrawerOpen(true)} />
          <Shield size={18} style={{ color: '#6366f1' }} />
          <span style={{ fontWeight: 600, fontSize: 14 }}>HermesX Admin</span>
        </Header>
        <Drawer
          placement="left"
          width={SIDEBAR_WIDTH}
          open={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          styles={{ body: { padding: 0 } }}
        >
          <SidebarContent onNavigate={() => setDrawerOpen(false)} />
        </Drawer>
        <Content style={{ overflow: 'auto' }}>
          <Outlet />
        </Content>
      </Layout>
    )
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider width={SIDEBAR_WIDTH} style={{ borderRight: '1px solid var(--ant-color-border)' }}>
        <SidebarContent />
      </Sider>
      <Content style={{ overflow: 'auto' }}>
        <Outlet />
      </Content>
    </Layout>
  )
}
