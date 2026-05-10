import { create } from 'zustand'
import type { Notification } from '@shared/types'

interface NotificationState {
  items: Notification[]
  unreadCount: number
  add: (n: Omit<Notification, 'id' | 'read' | 'created_at'>) => void
  markRead: (id: string) => void
  markAllRead: () => void
  clear: () => void
}

export const useNotificationStore = create<NotificationState>((set, get) => ({
  items: [],
  unreadCount: 0,

  add: (n) => {
    const item: Notification = {
      ...n,
      id: crypto.randomUUID(),
      read: false,
      created_at: new Date().toISOString(),
    }
    set((s) => ({ items: [item, ...s.items].slice(0, 50), unreadCount: s.unreadCount + 1 }))
  },

  markRead: (id) =>
    set((s) => ({
      items: s.items.map((i) => (i.id === id ? { ...i, read: true } : i)),
      unreadCount: Math.max(0, get().unreadCount - 1),
    })),

  markAllRead: () =>
    set((s) => ({
      items: s.items.map((i) => ({ ...i, read: true })),
      unreadCount: 0,
    })),

  clear: () => set({ items: [], unreadCount: 0 }),
}))
