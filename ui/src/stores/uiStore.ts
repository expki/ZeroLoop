import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { Theme, DetailMode, ChatWidth, ConnectionStatus } from '../types'

interface UIState {
  theme: Theme
  sidebarOpen: boolean
  chatWidth: ChatWidth
  detailMode: DetailMode
  connectionStatus: ConnectionStatus
  toggleTheme: () => void
  toggleSidebar: () => void
  setSidebarOpen: (open: boolean) => void
  setChatWidth: (w: ChatWidth) => void
  setDetailMode: (m: DetailMode) => void
}

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      theme: 'dark',
      sidebarOpen: true,
      chatWidth: '55em',
      detailMode: 'current',
      connectionStatus: 'connected',

      toggleTheme: () =>
        set((state) => ({ theme: state.theme === 'dark' ? 'light' : 'dark' })),

      toggleSidebar: () =>
        set((state) => ({ sidebarOpen: !state.sidebarOpen })),

      setSidebarOpen: (open) => set({ sidebarOpen: open }),
      setChatWidth: (chatWidth) => set({ chatWidth }),
      setDetailMode: (detailMode) => set({ detailMode }),
    }),
    {
      name: 'zeroloop-ui-preferences',
      partialize: (state) => ({
        theme: state.theme,
        chatWidth: state.chatWidth,
        detailMode: state.detailMode,
      }),
    }
  )
)
