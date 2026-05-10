import { create } from 'zustand'

type ThemeMode = 'dark' | 'light'

interface ThemeState {
  mode: ThemeMode
  toggle: () => void
  setMode: (mode: ThemeMode) => void
}

export const useThemeStore = create<ThemeState>((set) => ({
  mode: (localStorage.getItem('hx-theme') as ThemeMode) || 'dark',

  toggle: () =>
    set((state) => {
      const next = state.mode === 'dark' ? 'light' : 'dark'
      localStorage.setItem('hx-theme', next)
      document.documentElement.classList.toggle('dark', next === 'dark')
      return { mode: next }
    }),

  setMode: (mode) => {
    localStorage.setItem('hx-theme', mode)
    document.documentElement.classList.toggle('dark', mode === 'dark')
    set({ mode })
  },
}))
