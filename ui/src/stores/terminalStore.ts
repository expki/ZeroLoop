import { create } from 'zustand'
import type { Terminal as TerminalModel } from '../types'
import { api } from '../services/api'
import { ws } from '../services/websocket'
import type { Terminal as XTerm } from '@xterm/xterm'

interface TerminalState {
  terminals: TerminalModel[]
  selectedTerminalId: string | null
  initialized: boolean
  xterms: Map<string, XTerm>
  selectTerminal: (id: string | null) => void
  createTerminal: (projectId: string) => void
  deleteTerminal: (id: string) => void
  renameTerminal: (id: string, name: string) => void
  loadTerminalsForProject: (projectId: string) => Promise<void>
  sendInput: (terminalId: string, data: string) => void
  resizeTerminal: (terminalId: string, cols: number, rows: number) => void
  registerXterm: (terminalId: string, xterm: XTerm) => void
  unregisterXterm: (terminalId: string) => void
  init: () => void
}

export const useTerminalStore = create<TerminalState>((set, get) => ({
  terminals: [],
  selectedTerminalId: null,
  initialized: false,
  xterms: new Map(),

  init: () => {
    if (get().initialized) return
    set({ initialized: true })

    // Handle terminal output from backend
    ws.on('terminal_output', (payload: { terminal_id: string; data: string }) => {
      const { xterms } = get()
      const xterm = xterms.get(payload.terminal_id)
      if (xterm) {
        xterm.write(payload.data)
      }
    })

    // Handle terminal exit
    ws.on('terminal_exit', (payload: { terminal_id: string }) => {
      const { xterms } = get()
      const xterm = xterms.get(payload.terminal_id)
      if (xterm) {
        xterm.write('\r\n\x1b[90m[Process exited]\x1b[0m\r\n')
      }
    })
  },

  registerXterm: (terminalId, xterm) => {
    const { xterms } = get()
    xterms.set(terminalId, xterm)
    set({ xterms: new Map(xterms) })
  },

  unregisterXterm: (terminalId) => {
    const { xterms } = get()
    xterms.delete(terminalId)
    set({ xterms: new Map(xterms) })
  },

  selectTerminal: (id) => {
    set({ selectedTerminalId: id })
    if (id) {
      ws.send('terminal_subscribe', { terminal_id: id })
    }
  },

  createTerminal: async (projectId: string) => {
    try {
      const terminal = await api.createTerminal(projectId)
      set((state) => ({
        terminals: [terminal, ...state.terminals],
        selectedTerminalId: terminal.id,
      }))
      ws.send('terminal_subscribe', { terminal_id: terminal.id })
    } catch (err) {
      console.error('Failed to create terminal:', err)
    }
  },

  deleteTerminal: async (id) => {
    try {
      await api.deleteTerminal(id)
      set((state) => ({
        terminals: state.terminals.filter((t) => t.id !== id),
        selectedTerminalId: state.selectedTerminalId === id ? null : state.selectedTerminalId,
      }))
    } catch (err) {
      console.error('Failed to delete terminal:', err)
    }
  },

  renameTerminal: async (id, name) => {
    try {
      const updated = await api.renameTerminal(id, name)
      set((s) => ({
        terminals: s.terminals.map((t) => (t.id === id ? { ...t, name: updated.name } : t)),
      }))
    } catch (err) {
      console.error('Failed to rename terminal:', err)
    }
  },

  sendInput: (terminalId, data) => {
    ws.send('terminal_input', { terminal_id: terminalId, data })
  },

  resizeTerminal: (terminalId, cols, rows) => {
    ws.send('terminal_resize', { terminal_id: terminalId, cols, rows })
  },

  loadTerminalsForProject: async (projectId: string) => {
    try {
      const terminals = await api.listTerminals(projectId)
      set({ terminals: terminals || [], selectedTerminalId: null })
    } catch {
      set({ terminals: [], selectedTerminalId: null })
    }
  },
}))
