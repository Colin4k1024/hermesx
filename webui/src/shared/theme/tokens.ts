import { theme, type ThemeConfig } from 'antd'

const shared = {
  fontFamily: "'Inter', -apple-system, BlinkMacSystemFont, sans-serif",
  borderRadius: 6,
  borderRadiusLG: 8,
  borderRadiusSM: 4,
  wireframe: false,
  fontSize: 14,
  controlHeight: 36,
  controlHeightLG: 40,
  controlHeightSM: 28,
}

export const darkTheme: ThemeConfig = {
  algorithm: theme.darkAlgorithm,
  token: {
    ...shared,
    colorPrimary: '#6366f1',
    colorBgBase: '#09090b',
    colorBgContainer: '#111113',
    colorBgElevated: '#18181b',
    colorBorder: '#27272a',
    colorBorderSecondary: '#1f1f23',
    colorText: '#fafafa',
    colorTextSecondary: '#a1a1aa',
    colorTextTertiary: '#52525b',
    colorBgLayout: '#09090b',
    colorSuccess: '#22c55e',
    colorWarning: '#f59e0b',
    colorError: '#ef4444',
    colorInfo: '#6366f1',
  },
  components: {
    Layout: { headerBg: '#09090b', siderBg: '#09090b', bodyBg: '#09090b' },
    Menu: { darkItemBg: 'transparent', darkItemHoverBg: '#18181b', darkItemSelectedBg: '#1f1f23' },
    Table: { headerBg: '#111113', rowHoverBg: '#18181b', borderColor: '#27272a' },
    Card: { colorBgContainer: '#111113' },
    Input: { colorBgContainer: '#111113', activeBorderColor: '#6366f1' },
    Button: { primaryShadow: 'none', defaultShadow: 'none' },
    Modal: { contentBg: '#111113', headerBg: '#111113' },
    Drawer: { colorBgElevated: '#111113' },
  },
}

export const lightTheme: ThemeConfig = {
  algorithm: theme.defaultAlgorithm,
  token: {
    ...shared,
    colorPrimary: '#4f46e5',
    colorBgBase: '#ffffff',
    colorBgContainer: '#ffffff',
    colorBgElevated: '#ffffff',
    colorBorder: '#e4e4e7',
    colorBorderSecondary: '#f4f4f5',
    colorText: '#09090b',
    colorTextSecondary: '#71717a',
    colorTextTertiary: '#a1a1aa',
    colorBgLayout: '#fafafa',
    colorSuccess: '#16a34a',
    colorWarning: '#d97706',
    colorError: '#dc2626',
    colorInfo: '#4f46e5',
  },
  components: {
    Table: { headerBg: '#fafafa', rowHoverBg: '#f4f4f5' },
    Button: { primaryShadow: 'none', defaultShadow: 'none' },
  },
}
