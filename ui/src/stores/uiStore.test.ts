import { describe, it, expect, beforeEach } from 'vitest'
import { useUIStore } from './uiStore'

describe('uiStore', () => {
  beforeEach(() => {
    useUIStore.setState({
      theme: 'dark',
      sidebarOpen: true,
      chatWidth: '55em',
      detailMode: 'current',
      connectionStatus: 'connected',
    })
  })

  it('toggles theme', () => {
    useUIStore.getState().toggleTheme()
    expect(useUIStore.getState().theme).toBe('light')
    useUIStore.getState().toggleTheme()
    expect(useUIStore.getState().theme).toBe('dark')
  })

  it('toggles sidebar', () => {
    useUIStore.getState().toggleSidebar()
    expect(useUIStore.getState().sidebarOpen).toBe(false)
    useUIStore.getState().toggleSidebar()
    expect(useUIStore.getState().sidebarOpen).toBe(true)
  })

  it('sets chat width', () => {
    useUIStore.getState().setChatWidth('80em')
    expect(useUIStore.getState().chatWidth).toBe('80em')
  })

  it('sets detail mode', () => {
    useUIStore.getState().setDetailMode('expanded')
    expect(useUIStore.getState().detailMode).toBe('expanded')
  })

  it('sets sidebar open state', () => {
    useUIStore.getState().setSidebarOpen(false)
    expect(useUIStore.getState().sidebarOpen).toBe(false)
  })
})
