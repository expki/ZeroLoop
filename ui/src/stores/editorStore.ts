import { create } from 'zustand'
import { api } from '../services/api'

// --- Types ---

export interface EditorTab {
  id: string // file path (unique key)
  filePath: string
  dirty: boolean
  content: string | null // null = not yet loaded
  conflictContent: string | null // non-null when external modification detected on a dirty tab
}

export interface SplitPane {
  id: string
  tabs: string[] // tab IDs (file paths)
  activeTabId: string | null
}

export type SplitDirection = 'horizontal' | 'vertical'

export interface SplitLayout {
  panes: SplitPane[]
  direction: SplitDirection
  sizes: number[] // percentage sizes for each pane
}

export interface SearchMatch {
  path: string
  line: number
  column: number
  content: string
}

export interface CursorInfo {
  line: number
  column: number
  selectedLines: number
  selectedChars: number
}

export interface PendingReveal {
  filePath: string
  line: number
  column: number
}

// --- Helpers ---

function createDefaultLayout(): SplitLayout {
  return {
    panes: [{ id: 'main', tabs: [], activeTabId: null }],
    direction: 'horizontal',
    sizes: [100],
  }
}

let currentProjectId: string | null = null

function getPersistKey(projectId: string): string {
  return `zeroloop-editor-state-${projectId}`
}

function loadPersistedState(projectId: string): { tabs: Map<string, EditorTab>; layout: SplitLayout } | null {
  try {
    const raw = localStorage.getItem(getPersistKey(projectId))
    if (!raw) return null
    const data = JSON.parse(raw)
    const tabs = new Map<string, EditorTab>()
    for (const tab of data.tabs || []) {
      tabs.set(tab.id, { ...tab, content: null, conflictContent: null, dirty: false })
    }
    return { tabs, layout: data.layout || createDefaultLayout() }
  } catch {
    return null
  }
}

function persistState(projectId: string, tabs: Map<string, EditorTab>, layout: SplitLayout) {
  try {
    const tabList = Array.from(tabs.values()).map((t) => ({
      id: t.id,
      filePath: t.filePath,
    }))
    localStorage.setItem(getPersistKey(projectId), JSON.stringify({ tabs: tabList, layout }))
  } catch {
    // Ignore storage errors
  }
}

// --- Store ---

interface EditorState {
  tabs: Map<string, EditorTab>
  layout: SplitLayout
  quickOpenVisible: boolean
  searchPanelVisible: boolean
  cursorInfo: CursorInfo
  pendingReveal: PendingReveal | null

  // Actions
  initForProject: (projectId: string) => void
  openTab: (filePath: string, paneId?: string) => Promise<void>
  closeTab: (filePath: string, paneId: string) => void
  closeOtherTabs: (filePath: string, paneId: string) => void
  closeAllTabs: (paneId: string) => void
  closeSavedTabs: (paneId: string) => void
  setActiveTab: (filePath: string, paneId: string) => void
  markDirty: (filePath: string) => void
  markClean: (filePath: string) => void
  updateContent: (filePath: string, content: string) => void
  splitPane: (paneId: string, direction: SplitDirection) => void
  closeSplitPane: (paneId: string) => void
  moveTab: (filePath: string, fromPaneId: string, toPaneId: string) => void
  toggleQuickOpen: () => void
  toggleSearchPanel: () => void
  setCursorInfo: (info: CursorInfo) => void
  revealLine: (filePath: string, line: number, column: number) => void
  clearPendingReveal: () => void
  handleFileEvent: (action: string, path: string, oldPath?: string) => void
  resolveConflict: (filePath: string, choice: 'keep-mine' | 'load-theirs') => Promise<void>
  getActiveFile: () => string | null
}

export const useEditorStore = create<EditorState>((set, get) => ({
  tabs: new Map(),
  layout: createDefaultLayout(),
  quickOpenVisible: false,
  searchPanelVisible: false,
  cursorInfo: { line: 1, column: 1, selectedLines: 0, selectedChars: 0 },
  pendingReveal: null,

  initForProject: (projectId: string) => {
    if (currentProjectId === projectId) return
    // Persist current state before switching
    if (currentProjectId) {
      const state = get()
      persistState(currentProjectId, state.tabs, state.layout)
    }
    currentProjectId = projectId
    const persisted = loadPersistedState(projectId)
    if (persisted) {
      set({ tabs: persisted.tabs, layout: persisted.layout })
      // Reload content for persisted tabs
      for (const tab of persisted.tabs.values()) {
        get().openTab(tab.filePath)
      }
    } else {
      set({ tabs: new Map(), layout: createDefaultLayout() })
    }
  },

  openTab: async (filePath: string, paneId?: string) => {
    const state = get()
    const targetPaneId = paneId || state.layout.panes[0]?.id || 'main'

    // If tab already exists, just activate it
    if (state.tabs.has(filePath)) {
      set((s) => {
        const newLayout = { ...s.layout }
        newLayout.panes = newLayout.panes.map((p) => {
          if (p.id === targetPaneId) {
            if (!p.tabs.includes(filePath)) {
              return { ...p, tabs: [...p.tabs, filePath], activeTabId: filePath }
            }
            return { ...p, activeTabId: filePath }
          }
          return p
        })
        return { layout: newLayout }
      })
      // If content hasn't been loaded yet (e.g. restored from persistence), load it now
      if (state.tabs.get(filePath)?.content === null) {
        try {
          const projectId = currentProjectId
          if (!projectId) return
          const { content } = await api.readProjectFile(projectId, filePath)
          set((s) => {
            const newTabs = new Map(s.tabs)
            const tab = newTabs.get(filePath)
            if (tab) {
              newTabs.set(filePath, { ...tab, content })
            }
            return { tabs: newTabs }
          })
        } catch (err) {
          console.error('Failed to load file:', filePath, err)
        }
      }
      return
    }

    // Create new tab
    const newTab: EditorTab = {
      id: filePath,
      filePath,
      dirty: false,
      content: null,
      conflictContent: null,
    }

    set((s) => {
      const newTabs = new Map(s.tabs)
      newTabs.set(filePath, newTab)
      const newLayout = { ...s.layout }
      newLayout.panes = newLayout.panes.map((p) => {
        if (p.id === targetPaneId) {
          return { ...p, tabs: [...p.tabs, filePath], activeTabId: filePath }
        }
        return p
      })
      return { tabs: newTabs, layout: newLayout }
    })

    // Load content
    try {
      const projectId = currentProjectId
      if (!projectId) return
      const { content } = await api.readProjectFile(projectId, filePath)
      set((s) => {
        const newTabs = new Map(s.tabs)
        const tab = newTabs.get(filePath)
        if (tab) {
          newTabs.set(filePath, { ...tab, content })
        }
        return { tabs: newTabs }
      })
      if (currentProjectId) persistState(currentProjectId, get().tabs, get().layout)
    } catch (err) {
      console.error('Failed to load file:', filePath, err)
    }
  },

  closeTab: (filePath: string, paneId: string) => {
    set((s) => {
      const newTabs = new Map(s.tabs)
      const newLayout = { ...s.layout }

      // Check if tab is in any other pane
      let tabInOtherPane = false
      newLayout.panes = newLayout.panes.map((p) => {
        if (p.id === paneId) {
          const newPaneTabs = p.tabs.filter((t) => t !== filePath)
          let newActiveTab = p.activeTabId
          if (p.activeTabId === filePath) {
            const idx = p.tabs.indexOf(filePath)
            newActiveTab = newPaneTabs[Math.min(idx, newPaneTabs.length - 1)] || null
          }
          return { ...p, tabs: newPaneTabs, activeTabId: newActiveTab }
        } else if (p.tabs.includes(filePath)) {
          tabInOtherPane = true
        }
        return p
      })

      // Only delete tab data if it's not in any other pane
      if (!tabInOtherPane) {
        newTabs.delete(filePath)
      }

      return { tabs: newTabs, layout: newLayout }
    })
    if (currentProjectId) persistState(currentProjectId, get().tabs, get().layout)
  },

  setActiveTab: (filePath: string, paneId: string) => {
    set((s) => {
      const newLayout = { ...s.layout }
      newLayout.panes = newLayout.panes.map((p) =>
        p.id === paneId ? { ...p, activeTabId: filePath } : p
      )
      return { layout: newLayout }
    })
  },

  markDirty: (filePath: string) => {
    set((s) => {
      const newTabs = new Map(s.tabs)
      const tab = newTabs.get(filePath)
      if (tab && !tab.dirty) {
        newTabs.set(filePath, { ...tab, dirty: true })
      }
      return { tabs: newTabs }
    })
  },

  markClean: (filePath: string) => {
    set((s) => {
      const newTabs = new Map(s.tabs)
      const tab = newTabs.get(filePath)
      if (tab) {
        newTabs.set(filePath, { ...tab, dirty: false, conflictContent: null })
      }
      return { tabs: newTabs }
    })
  },

  updateContent: (filePath: string, content: string) => {
    set((s) => {
      const newTabs = new Map(s.tabs)
      const tab = newTabs.get(filePath)
      if (tab) {
        newTabs.set(filePath, { ...tab, content, dirty: true })
      }
      return { tabs: newTabs }
    })
  },

  splitPane: (paneId: string, direction: SplitDirection) => {
    set((s) => {
      const newLayout = { ...s.layout }
      const paneIndex = newLayout.panes.findIndex((p) => p.id === paneId)
      if (paneIndex === -1) return s

      const newPane: SplitPane = {
        id: `pane-${Date.now()}`,
        tabs: [],
        activeTabId: null,
      }

      const newPanes = [...newLayout.panes]
      newPanes.splice(paneIndex + 1, 0, newPane)

      const newSizes = newPanes.map(() => 100 / newPanes.length)

      return {
        layout: {
          panes: newPanes,
          direction: newPanes.length === 2 ? direction : newLayout.direction,
          sizes: newSizes,
        },
      }
    })
  },

  closeSplitPane: (paneId: string) => {
    set((s) => {
      if (s.layout.panes.length <= 1) return s
      const newLayout = { ...s.layout }
      const closingPane = newLayout.panes.find((p) => p.id === paneId)
      if (!closingPane) return s

      // Move tabs from closing pane to first remaining pane
      const remainingPanes = newLayout.panes.filter((p) => p.id !== paneId)
      if (remainingPanes.length > 0 && closingPane.tabs.length > 0) {
        remainingPanes[0] = {
          ...remainingPanes[0],
          tabs: [...remainingPanes[0].tabs, ...closingPane.tabs],
        }
      }

      const newSizes = remainingPanes.map(() => 100 / remainingPanes.length)

      return {
        layout: {
          panes: remainingPanes,
          direction: newLayout.direction,
          sizes: newSizes,
        },
      }
    })
  },

  moveTab: (filePath: string, fromPaneId: string, toPaneId: string) => {
    if (fromPaneId === toPaneId) return
    set((s) => {
      const newLayout = { ...s.layout }
      newLayout.panes = newLayout.panes.map((p) => {
        if (p.id === fromPaneId) {
          const newTabs = p.tabs.filter((t) => t !== filePath)
          const newActive = p.activeTabId === filePath
            ? newTabs[0] || null
            : p.activeTabId
          return { ...p, tabs: newTabs, activeTabId: newActive }
        }
        if (p.id === toPaneId) {
          return { ...p, tabs: [...p.tabs, filePath], activeTabId: filePath }
        }
        return p
      })
      return { layout: newLayout }
    })
  },

  toggleQuickOpen: () => set((s) => ({ quickOpenVisible: !s.quickOpenVisible })),
  toggleSearchPanel: () => set((s) => ({ searchPanelVisible: !s.searchPanelVisible })),

  setCursorInfo: (info: CursorInfo) => set({ cursorInfo: info }),

  revealLine: (filePath: string, line: number, column: number) => {
    // Open the tab first, then set pending reveal
    get().openTab(filePath).then(() => {
      set({ pendingReveal: { filePath, line, column } })
    })
  },

  clearPendingReveal: () => set({ pendingReveal: null }),

  closeOtherTabs: (filePath: string, paneId: string) => {
    set((s) => {
      const newTabs = new Map(s.tabs)
      const newLayout = { ...s.layout }
      newLayout.panes = newLayout.panes.map((p) => {
        if (p.id === paneId) {
          const closedTabs = p.tabs.filter((t) => t !== filePath)
          // Remove tabs not in any other pane
          for (const tabId of closedTabs) {
            const inOtherPane = newLayout.panes.some((op) => op.id !== paneId && op.tabs.includes(tabId))
            if (!inOtherPane) newTabs.delete(tabId)
          }
          return { ...p, tabs: [filePath], activeTabId: filePath }
        }
        return p
      })
      return { tabs: newTabs, layout: newLayout }
    })
    if (currentProjectId) persistState(currentProjectId, get().tabs, get().layout)
  },

  closeAllTabs: (paneId: string) => {
    set((s) => {
      const newTabs = new Map(s.tabs)
      const newLayout = { ...s.layout }
      newLayout.panes = newLayout.panes.map((p) => {
        if (p.id === paneId) {
          for (const tabId of p.tabs) {
            const inOtherPane = newLayout.panes.some((op) => op.id !== paneId && op.tabs.includes(tabId))
            if (!inOtherPane) newTabs.delete(tabId)
          }
          return { ...p, tabs: [], activeTabId: null }
        }
        return p
      })
      return { tabs: newTabs, layout: newLayout }
    })
    if (currentProjectId) persistState(currentProjectId, get().tabs, get().layout)
  },

  closeSavedTabs: (paneId: string) => {
    set((s) => {
      const newTabs = new Map(s.tabs)
      const newLayout = { ...s.layout }
      newLayout.panes = newLayout.panes.map((p) => {
        if (p.id === paneId) {
          const remaining = p.tabs.filter((t) => newTabs.get(t)?.dirty)
          const closed = p.tabs.filter((t) => !newTabs.get(t)?.dirty)
          for (const tabId of closed) {
            const inOtherPane = newLayout.panes.some((op) => op.id !== paneId && op.tabs.includes(tabId))
            if (!inOtherPane) newTabs.delete(tabId)
          }
          const newActive = remaining.includes(p.activeTabId || '') ? p.activeTabId : remaining[0] || null
          return { ...p, tabs: remaining, activeTabId: newActive }
        }
        return p
      })
      return { tabs: newTabs, layout: newLayout }
    })
    if (currentProjectId) persistState(currentProjectId, get().tabs, get().layout)
  },

  handleFileEvent: (action: string, path: string, oldPath?: string) => {
    const state = get()

    if (action === 'deleted') {
      // Close tabs for deleted files
      for (const pane of state.layout.panes) {
        if (pane.tabs.includes(path)) {
          get().closeTab(path, pane.id)
        }
      }
      return
    }

    if (action === 'renamed' && oldPath) {
      // Update tab paths for renamed files
      set((s) => {
        const newTabs = new Map(s.tabs)
        const tab = newTabs.get(oldPath)
        if (tab) {
          newTabs.delete(oldPath)
          const updatedTab = { ...tab, id: path, filePath: path }
          newTabs.set(path, updatedTab)
        }
        const newLayout = { ...s.layout }
        newLayout.panes = newLayout.panes.map((p) => ({
          ...p,
          tabs: p.tabs.map((t) => (t === oldPath ? path : t)),
          activeTabId: p.activeTabId === oldPath ? path : p.activeTabId,
        }))
        return { tabs: newTabs, layout: newLayout }
      })
      return
    }

    if (action === 'changed') {
      const tab = state.tabs.get(path)
      if (!tab) return // File not open, nothing to do

      if (!tab.dirty) {
        // Clean tab: silently reload
        const projectId = currentProjectId
        if (!projectId) return
        api.readProjectFile(projectId, path).then(({ content }) => {
          set((s) => {
            const newTabs = new Map(s.tabs)
            const currentTab = newTabs.get(path)
            if (currentTab && !currentTab.dirty) {
              newTabs.set(path, { ...currentTab, content })
            }
            return { tabs: newTabs }
          })
        }).catch(() => {
          // Ignore reload errors
        })
      } else {
        // Dirty tab: signal conflict
        const projectId = currentProjectId
        if (!projectId) return
        api.readProjectFile(projectId, path).then(({ content }) => {
          set((s) => {
            const newTabs = new Map(s.tabs)
            const currentTab = newTabs.get(path)
            if (currentTab && currentTab.dirty) {
              newTabs.set(path, { ...currentTab, conflictContent: content })
            }
            return { tabs: newTabs }
          })
        }).catch(() => {
          // Ignore errors
        })
      }
      return
    }
  },

  resolveConflict: async (filePath: string, choice: 'keep-mine' | 'load-theirs') => {
    if (choice === 'keep-mine') {
      set((s) => {
        const newTabs = new Map(s.tabs)
        const tab = newTabs.get(filePath)
        if (tab) {
          newTabs.set(filePath, { ...tab, conflictContent: null })
        }
        return { tabs: newTabs }
      })
    } else {
      // Load fresh content from server
      const projectId = currentProjectId
      if (!projectId) return
      try {
        const { content } = await api.readProjectFile(projectId, filePath)
        set((s) => {
          const newTabs = new Map(s.tabs)
          const tab = newTabs.get(filePath)
          if (tab) {
            newTabs.set(filePath, { ...tab, content, dirty: false, conflictContent: null })
          }
          return { tabs: newTabs }
        })
      } catch {
        // Ignore errors
      }
    }
  },

  getActiveFile: () => {
    const state = get()
    const firstPane = state.layout.panes[0]
    return firstPane?.activeTabId || null
  },
}))
