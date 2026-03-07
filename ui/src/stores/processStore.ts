import { create } from 'zustand'
import type { Process, ProcessLogLine } from '../types'
import { api } from '../services/api'
import { ws } from '../services/websocket'

interface ProcessState {
  processes: Process[]
  selectedProcessId: string | null
  logs: Map<string, ProcessLogLine[]>
  initialized: boolean

  init: () => void
  selectProcess: (id: string | null) => void
  stopProcess: (id: string) => void
  loadProcessesForProject: (projectId: string) => Promise<void>
}

export const useProcessStore = create<ProcessState>((set, get) => ({
  processes: [],
  selectedProcessId: null,
  logs: new Map(),
  initialized: false,

  init: () => {
    if (get().initialized) return
    set({ initialized: true })

    // Handle process output
    ws.on('process_output', (payload: { process_id: string; stream: 'stdout' | 'stderr'; text: string; timestamp: string }) => {
      const { logs } = get()
      const existing = logs.get(payload.process_id) || []
      const newLine: ProcessLogLine = {
        timestamp: payload.timestamp,
        stream: payload.stream,
        text: payload.text,
      }
      const MAX_LOG_LINES = 1000
      const combined = [...existing, newLine]
      const updated = new Map(logs)
      updated.set(payload.process_id, combined.length > MAX_LOG_LINES ? combined.slice(-MAX_LOG_LINES) : combined)
      set({ logs: updated })
    })

    // Handle process started (broadcast to project)
    ws.on('process_started', (payload: { process_id: string; project_id: string; command: string }) => {
      const newProcess: Process = {
        id: payload.process_id,
        project_id: payload.project_id,
        command: payload.command,
        status: 'running',
        started_at: new Date().toISOString(),
      }
      set((s) => ({ processes: [newProcess, ...s.processes] }))
    })

    // Handle process exit — remove from list
    ws.on('process_exit', (payload: { process_id: string; exit_code: number }) => {
      set((s) => ({
        processes: s.processes.filter((p) => p.id !== payload.process_id),
        selectedProcessId: s.selectedProcessId === payload.process_id ? null : s.selectedProcessId,
      }))
    })

    // Handle process info (sent on subscribe)
    ws.on('process_info', (payload: Process) => {
      set((s) => ({
        processes: s.processes.map((p) =>
          p.id === payload.id ? { ...p, ...payload } : p
        ),
      }))
    })

    // Re-subscribe on reconnect
    ws.onReconnect(() => {
      const { selectedProcessId } = get()
      if (selectedProcessId) {
        ws.send('process_subscribe', { process_id: selectedProcessId })
      }
    })
  },

  selectProcess: (id) => {
    set({ selectedProcessId: id })
    if (id) {
      ws.send('process_subscribe', { process_id: id })
    }
  },

  stopProcess: (id) => {
    ws.send('process_stop', { process_id: id })
  },

  loadProcessesForProject: async (projectId: string) => {
    try {
      const allProcesses = await api.listProcesses(projectId)
      const processes = (allProcesses || []).filter((p: Process) => p.status === 'running')
      set({ processes, selectedProcessId: null, logs: new Map() })
    } catch {
      set({ processes: [], selectedProcessId: null, logs: new Map() })
    }
  },
}))
