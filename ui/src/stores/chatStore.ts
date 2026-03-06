import { create } from 'zustand'
import type { Chat, Message } from '../types'
import { api } from '../services/api'
import { ws } from '../services/websocket'

interface ChatState {
  chats: Chat[]
  selectedChatId: string | null
  messages: Message[]
  loading: boolean
  paused: boolean
  queueSize: number
  initialized: boolean
  selectChat: (id: string | null) => void
  createChat: (projectId: string) => void
  deleteChat: (id: string) => void
  renameChat: (id: string, name: string) => void
  sendMessage: (content: string) => void
  intervene: (content: string) => void
  branchChat: (messageNo?: number) => void
  togglePause: () => void
  cancelChat: () => void
  clearChat: () => void
  exportChat: () => void
  loadChatsForProject: (projectId: string) => Promise<void>
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

export const useChatStore = create<ChatState>((set, get) => ({
  chats: [],
  selectedChatId: null,
  messages: [],
  loading: false,
  paused: false,
  queueSize: 0,
  initialized: false,

  init: () => {
    if (get().initialized) return
    set({ initialized: true })

    // Connect WebSocket
    ws.connect()

    // Resubscribe to active chat on reconnect
    ws.onReconnect(() => {
      const { selectedChatId } = get()
      if (selectedChatId) {
        ws.send('subscribe', { chat_id: selectedChatId })
      }
    })

    // Chats are loaded per-project via loadChatsForProject, not globally

    // Handle incoming messages
    ws.on('message', (payload: Message) => {
      const state = get()
      if (payload.chat_id && payload.chat_id !== state.selectedChatId) return

      const msg: Message = {
        ...payload,
        kvps: parseKvps(payload.kvps),
      }

      set((s) => ({
        messages: [...s.messages, msg],
      }))
    })

    // Handle streaming chunks (append to last agent message)
    ws.on('stream', (payload: { chat_id: string; content: string; type: string; stream: boolean }) => {
      const state = get()
      if (payload.chat_id !== state.selectedChatId) return

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

    // Handle chat updates (running state, name changes, paused state)
    ws.on('chat_update', (payload: { id: string; running?: boolean; name?: string; paused?: boolean; queue_size?: number }) => {
      set((s) => {
        const newState: Partial<ChatState> = {
          chats: s.chats.map((c) => {
            if (c.id !== payload.id) return c
            const updated = { ...c }
            if (payload.running !== undefined) updated.running = payload.running
            if (payload.name !== undefined) updated.name = payload.name
            return updated
          }),
        }
        if (payload.id === s.selectedChatId) {
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
    ws.on('clear', (payload: { chat_id: string }) => {
      const state = get()
      if (payload.chat_id === state.selectedChatId) {
        set({ messages: [] })
      }
    })

    // Handle connection status — UI store handles this in App.tsx
    ws.on('_status', () => {})
  },

  selectChat: async (id) => {
    set({ selectedChatId: id, messages: [], loading: !!id })

    if (id) {
      // Subscribe to chat via WebSocket
      ws.send('subscribe', { chat_id: id })

      // Load messages from API
      try {
        const messages = await api.getChatMessages(id)
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

  createChat: async (projectId: string) => {
    try {
      const chat = await api.createChat(projectId)
      set((state) => ({
        chats: [chat, ...state.chats],
        selectedChatId: chat.id,
        messages: [],
      }))
      ws.send('subscribe', { chat_id: chat.id })
    } catch (err) {
      console.error('Failed to create chat:', err)
    }
  },

  deleteChat: async (id) => {
    try {
      await api.deleteChat(id)
      set((state) => ({
        chats: state.chats.filter((c) => c.id !== id),
        selectedChatId: state.selectedChatId === id ? null : state.selectedChatId,
        messages: state.selectedChatId === id ? [] : state.messages,
      }))
    } catch (err) {
      console.error('Failed to delete chat:', err)
    }
  },

  renameChat: async (id, name) => {
    try {
      const updated = await api.renameChat(id, name)
      set((s) => ({
        chats: s.chats.map((c) => (c.id === id ? { ...c, name: updated.name } : c)),
      }))
    } catch (err) {
      console.error('Failed to rename chat:', err)
    }
  },

  exportChat: async () => {
    const { selectedChatId } = get()
    if (!selectedChatId) return
    try {
      const data = await api.exportChat(selectedChatId)
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${data.chat.name || 'chat'}.json`
      a.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      console.error('Failed to export chat:', err)
    }
  },

  sendMessage: (content) => {
    const { selectedChatId } = get()
    if (!selectedChatId || !content.trim()) return

    ws.send('send_message', {
      chat_id: selectedChatId,
      content: content.trim(),
    })
    // Clear paused state — sending a new message supersedes any pause
    set({ paused: false, queueSize: 0 })
  },

  intervene: (content) => {
    const { selectedChatId } = get()
    if (!selectedChatId || !content.trim()) return

    ws.send('intervene', {
      chat_id: selectedChatId,
      content: content.trim(),
    })
  },

  branchChat: async (messageNo) => {
    const { selectedChatId } = get()
    if (!selectedChatId) return
    try {
      const newChat = await api.branchChat(selectedChatId, messageNo)
      set((state) => ({
        chats: [newChat, ...state.chats],
        selectedChatId: newChat.id,
        messages: [],
      }))
      ws.send('subscribe', { chat_id: newChat.id })
      // Load the branched messages
      const messages = await api.getChatMessages(newChat.id)
      set({
        messages: (messages || []).map((m) => ({
          ...m,
          kvps: parseKvps(m.kvps),
        })),
      })
    } catch (err) {
      console.error('Failed to branch chat:', err)
    }
  },

  togglePause: () => {
    const { selectedChatId, paused } = get()
    if (!selectedChatId) return

    if (paused) {
      // Resume: send resume event to backend
      ws.send('resume', { chat_id: selectedChatId })
    } else {
      // Pause: send pause event to backend
      ws.send('pause', { chat_id: selectedChatId })
    }
    // State will be confirmed by chat_update from backend
  },

  cancelChat: () => {
    const { selectedChatId } = get()
    if (!selectedChatId) return
    ws.send('cancel', { chat_id: selectedChatId })
  },

  clearChat: () => {
    const { selectedChatId } = get()
    if (!selectedChatId) return
    ws.send('clear', { chat_id: selectedChatId })
    set({ messages: [] })
  },

  loadChatsForProject: async (projectId: string) => {
    try {
      const chats = await api.listChats(projectId)
      set({ chats: chats || [], selectedChatId: null, messages: [] })
    } catch {
      set({ chats: [], selectedChatId: null, messages: [] })
    }
  },
}))
