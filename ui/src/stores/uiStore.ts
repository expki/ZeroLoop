import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { Theme, DetailMode, AgentWidth, ConnectionStatus } from '../types'

interface UIState {
  theme: Theme
  sidebarOpen: boolean
  agentWidth: AgentWidth
  detailMode: DetailMode
  connectionStatus: ConnectionStatus
  toggleTheme: () => void
  toggleSidebar: () => void
  setSidebarOpen: (open: boolean) => void
  setAgentWidth: (w: AgentWidth) => void
  setDetailMode: (m: DetailMode) => void
}

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      theme: 'dark',
      sidebarOpen: true,
      agentWidth: '55em',
      detailMode: 'current',
      connectionStatus: 'connected',

      toggleTheme: () =>
        set((state) => ({ theme: state.theme === 'dark' ? 'light' : 'dark' })),

      toggleSidebar: () =>
        set((state) => ({ sidebarOpen: !state.sidebarOpen })),

      setSidebarOpen: (open) => set({ sidebarOpen: open }),
      setAgentWidth: (agentWidth) => set({ agentWidth }),
      setDetailMode: (detailMode) => set({ detailMode }),
    }),
    {
      name: 'zeroloop-ui-preferences',
      partialize: (state) => ({
        theme: state.theme,
        agentWidth: state.agentWidth,
        detailMode: state.detailMode,
      }),
    }
  )
)
