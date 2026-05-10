import { useState, useEffect } from 'react'
import { Layout, Menu, Button, Tooltip, Badge, Drawer } from 'antd'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import {
  MessageSquare,
  Brain,
  Zap,
  BarChart3,
  Settings,
  Bell,
  LogOut,
  Sun,
  Moon,
  MenuIcon,
  BellRing,
} from 'lucide-react'
import { useAuthStore } from '@shared/stores/authStore'
import { useThemeStore } from '@shared/stores/themeStore'
import { useNotificationStore } from '@shared/hooks/useNotification'
import { SIDEBAR_WIDTH } from '@shared/theme/constants'

const { Sider, Content, Header } = Layout

const navItems = [
  { key: '/chat', icon: <MessageSquare size={18} />, label: 'Chat' },
  { key: '/memories', icon: <Brain size={18} />, label: 'Memories' },
  { key: '/skills', icon: <Zap size={18} />, label: 'Skills' },
  { key: '/usage', icon: <BarChart3 size={18} />, label: 'Usage' },
  { key: '/notifications', icon: <BellRing size={18} />, label: 'Notifications' },
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
  const disconnectUser = useAuthStore((s) => s.disconnectUser)
  const { mode, toggle } = useThemeStore()
  const unreadCount = useNotificationStore((s) => s.unreadCount)

  const handleNav = (key: string) => {
    navigate(key)
    onNavigate?.()
  }

  const handleLogout = () => {
    disconnectUser()
    navigate('/login', { replace: true })
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div style={{ padding: '16px 20px', display: 'flex', alignItems: 'center', gap: 10 }}>
        <MessageSquare size={20} style={{ color: '#6366f1' }} />
        <span style={{ fontWeight: 600, fontSize: 15 }}>HermesX</span>
      </div>
      <Menu
        mode="inline"
        selectedKeys={[location.pathname]}
        items={navItems}
        onClick={({ key }) => handleNav(key)}
        style={{ borderRight: 'none', flex: 1 }}
      />
      <div style={{ padding: '12px', display: 'flex', gap: 8 }}>
        <Tooltip title="Notifications">
          <Badge count={unreadCount} size="small">
            <Button type="text" icon={<Bell size={16} />} onClick={() => { navigate('/notifications'); onNavigate?.() }} />
          </Badge>
        </Tooltip>
        <Tooltip title={mode === 'dark' ? 'Light mode' : 'Dark mode'}>
          <Button type="text" icon={mode === 'dark' ? <Sun size={16} /> : <Moon size={16} />} onClick={toggle} />
        </Tooltip>
        <Tooltip title="Disconnect">
          <Button type="text" icon={<LogOut size={16} />} onClick={handleLogout} />
        </Tooltip>
      </div>
    </div>
  )
}

export default function UserShell() {
  const isMobile = useIsMobile()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const unreadCount = useNotificationStore((s) => s.unreadCount)

  if (isMobile) {
    return (
      <Layout style={{ minHeight: '100vh' }}>
        <Header style={{ padding: '0 16px', display: 'flex', alignItems: 'center', gap: 12, height: 48, lineHeight: '48px' }}>
          <Button type="text" icon={<MenuIcon size={18} />} onClick={() => setDrawerOpen(true)} />
          <MessageSquare size={18} style={{ color: '#6366f1' }} />
          <span style={{ fontWeight: 600, fontSize: 14 }}>HermesX</span>
          <div style={{ marginLeft: 'auto' }}>
            <Badge count={unreadCount} size="small">
              <Bell size={16} style={{ opacity: 0.7 }} />
            </Badge>
          </div>
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
