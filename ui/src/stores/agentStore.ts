import { create } from 'zustand'
import type { Agent, AgentType, AgentMode, Message } from '../types'
import { api } from '../services/api'
import { ws } from '../services/websocket'

interface AgentState {
  agents: Agent[]
  selectedAgentId: string | null
  messages: Message[]
  childMessages: Message[]
  loading: boolean
  paused: boolean
  queueSize: number
  initialized: boolean
  selectAgent: (id: string | null) => void
  createAgent: (projectId: string, type?: AgentType, mode?: AgentMode) => void
  deleteAgent: (id: string) => void
  renameAgent: (id: string, name: string) => void
  sendMessage: (content: string) => void
  intervene: (content: string) => void
  branchAgent: (messageNo?: number) => void
  togglePause: () => void
  cancelAgent: () => void
  clearAgent: () => void
  exportAgent: () => void
  loadAgentsForProject: (projectId: string) => Promise<void>
  init: () => void
}

function parseKvps(kvps: string | Record<string, string> | undefined): Record<string, string> | undefined {
  if (!kvps) return undefined
  if (typeof kvps === 'string') {
    try {
      const parsed = JSON.parse(kvps)
      return typeof parsed === 'object' ? parsed : undefined
    } catch {
      return undefined
    }
  }
  return kvps
}

export const useAgentStore = create<AgentState>((set, get) => ({
  agents: [],
  selectedAgentId: null,
  messages: [],
  childMessages: [],
  loading: false,
  paused: false,
  queueSize: 0,
  initialized: false,

  init: () => {
    if (get().initialized) return
    set({ initialized: true })

    // Connect WebSocket
    ws.connect()

    // Resubscribe to active agent on reconnect
    ws.onReconnect(() => {
      const { selectedAgentId } = get()
      if (selectedAgentId) {
        ws.send('subscribe', { agent_id: selectedAgentId })
      }
    })

    // Handle incoming messages
    ws.on('message', (payload: Message) => {
      const state = get()
      if (payload.agent_id && payload.agent_id !== state.selectedAgentId) return

      const msg: Message = {
        ...payload,
        kvps: parseKvps(payload.kvps),
      }

      set((s) => ({
        messages: [...s.messages, msg],
      }))
    })

    // Handle streaming chunks (append to last agent message)
    ws.on('stream', (payload: { agent_id: string; content: string; type: string; stream: boolean }) => {
      const state = get()
      if (payload.agent_id !== state.selectedAgentId) return

      set((s) => {
        const msgs = [...s.messages]
        const lastMsg = msgs[msgs.length - 1]
        if (lastMsg && lastMsg.type === 'agent' && payload.stream) {
          msgs[msgs.length - 1] = {
            ...lastMsg,
            content: lastMsg.content + payload.content,
          }
        }
        return { messages: msgs }
      })
    })

    // Handle agent updates (running state, name changes, paused state)
    ws.on('chat_update', (payload: { id: string; running?: boolean; name?: string; paused?: boolean; queue_size?: number }) => {
      set((s) => {
        const newState: Partial<AgentState> = {
          agents: s.agents.map((a) => {
            if (a.id !== payload.id) return a
            const updated = { ...a }
            if (payload.running !== undefined) updated.running = payload.running
            if (payload.name !== undefined) updated.name = payload.name
            return updated
          }),
        }
        if (payload.id === s.selectedAgentId) {
          if (payload.paused !== undefined) {
            newState.paused = payload.paused
          }
          if (payload.queue_size !== undefined) {
            newState.queueSize = payload.queue_size
          }
        }
        return newState
      })
    })

    // Handle clear
    ws.on('clear', (payload: { agent_id: string }) => {
      const state = get()
      if (payload.agent_id === state.selectedAgentId) {
        set({ messages: [], childMessages: [] })
      }
    })

    // Handle child agent messages (from orchestrator children)
    ws.on('child_message', (payload: Message & { child_agent_id?: string }) => {
      const state = get()
      if (payload.agent_id !== state.selectedAgentId) return

      const msg: Message = {
        ...payload,
        kvps: parseKvps(payload.kvps),
      }
      set((s) => ({ childMessages: [...s.childMessages, msg] }))
    })

    // Handle connection status — UI store handles this in App.tsx
    ws.on('_status', () => {})
  },

  selectAgent: async (id) => {
    set({ selectedAgentId: id, messages: [], childMessages: [], loading: !!id })

    if (id) {
      // Subscribe to agent via WebSocket
      ws.send('subscribe', { agent_id: id })

      // Load messages from API
      try {
        const messages = await api.getAgentMessages(id)
        set({
          loading: false,
          messages: (messages || []).map((m) => ({
            ...m,
            kvps: parseKvps(m.kvps),
          })),
        })
      } catch {
        set({ loading: false, messages: [] })
      }
    }
  },

  createAgent: async (projectId: string, type?: AgentType, mode?: AgentMode) => {
    try {
      const agent = await api.createAgent(projectId, undefined, type, mode)
      set((state) => ({
        agents: [agent, ...state.agents],
        selectedAgentId: agent.id,
        messages: [],
        childMessages: [],
      }))
      ws.send('subscribe', { agent_id: agent.id })
    } catch (err) {
      console.error('Failed to create agent:', err)
    }
  },

  deleteAgent: async (id) => {
    try {
      await api.deleteAgent(id)
      set((state) => ({
        agents: state.agents.filter((a) => a.id !== id),
        selectedAgentId: state.selectedAgentId === id ? null : state.selectedAgentId,
        messages: state.selectedAgentId === id ? [] : state.messages,
      }))
    } catch (err) {
      console.error('Failed to delete agent:', err)
    }
  },

  renameAgent: async (id, name) => {
    try {
      const updated = await api.renameAgent(id, name)
      set((s) => ({
        agents: s.agents.map((a) => (a.id === id ? { ...a, name: updated.name } : a)),
      }))
    } catch (err) {
      console.error('Failed to rename agent:', err)
    }
  },

  exportAgent: async () => {
    const { selectedAgentId } = get()
    if (!selectedAgentId) return
    try {
      const data = await api.exportAgent(selectedAgentId)
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${data.agent.name || 'agent'}.json`
      a.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      console.error('Failed to export agent:', err)
    }
  },

  sendMessage: (content) => {
    const { selectedAgentId } = get()
    if (!selectedAgentId || !content.trim()) return

    ws.send('send_message', {
      agent_id: selectedAgentId,
      content: content.trim(),
    })
    // Clear paused state — sending a new message supersedes any pause
    set({ paused: false, queueSize: 0 })
  },

  intervene: (content) => {
    const { selectedAgentId } = get()
    if (!selectedAgentId || !content.trim()) return

    ws.send('intervene', {
      agent_id: selectedAgentId,
      content: content.trim(),
    })
  },

  branchAgent: async (messageNo) => {
    const { selectedAgentId } = get()
    if (!selectedAgentId) return
    try {
      const newAgent = await api.branchAgent(selectedAgentId, messageNo)
      set((state) => ({
        agents: [newAgent, ...state.agents],
        selectedAgentId: newAgent.id,
        messages: [],
      }))
      ws.send('subscribe', { agent_id: newAgent.id })
      // Load the branched messages
      const messages = await api.getAgentMessages(newAgent.id)
      set({
        messages: (messages || []).map((m) => ({
          ...m,
          kvps: parseKvps(m.kvps),
        })),
      })
    } catch (err) {
      console.error('Failed to branch agent:', err)
    }
  },

  togglePause: () => {
    const { selectedAgentId, paused } = get()
    if (!selectedAgentId) return

    if (paused) {
      ws.send('resume', { agent_id: selectedAgentId })
    } else {
      ws.send('pause', { agent_id: selectedAgentId })
    }
  },

  cancelAgent: () => {
    const { selectedAgentId } = get()
    if (!selectedAgentId) return
    ws.send('cancel', { agent_id: selectedAgentId })
  },

  clearAgent: () => {
    const { selectedAgentId } = get()
    if (!selectedAgentId) return
    ws.send('clear', { agent_id: selectedAgentId })
    set({ messages: [] })
  },

  loadAgentsForProject: async (projectId: string) => {
    try {
      const agents = await api.listAgents(projectId)
      set({ agents: agents || [], selectedAgentId: null, messages: [], childMessages: [] })
    } catch {
      set({ agents: [], selectedAgentId: null, messages: [], childMessages: [] })
    }
  },
}))
