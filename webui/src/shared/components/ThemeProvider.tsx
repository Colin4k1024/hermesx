import { useEffect } from 'react'
import { ConfigProvider, theme as antTheme } from 'antd'
import { useThemeStore } from '@shared/stores/themeStore'
import { darkTheme, lightTheme } from '@shared/theme/tokens'

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const mode = useThemeStore((s) => s.mode)

  useEffect(() => {
    const isDark = mode === 'dark'
    document.documentElement.classList.toggle('dark', isDark)
    document.body.style.backgroundColor = isDark ? '#09090b' : '#ffffff'
    document.body.style.color = isDark ? '#fafafa' : '#09090b'
  }, [mode])

  return (
    <ConfigProvider
      theme={{
        ...(mode === 'dark' ? darkTheme : lightTheme),
        algorithm: mode === 'dark' ? antTheme.darkAlgorithm : antTheme.defaultAlgorithm,
      }}
    >
      {children}
    </ConfigProvider>
  )
}
