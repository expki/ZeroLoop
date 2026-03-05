export type MessageType =
  | 'user'
  | 'agent'
  | 'response'
  | 'tool'
  | 'code_exe'
  | 'warning'
  | 'error'
  | 'info'
  | 'util'
  | 'hint'
  | 'progress'

export interface Message {
  id: string
  no: number
  type: MessageType
  heading: string
  content: string
  kvps?: Record<string, string>
  timestamp: number
  agentno: number
}

export interface Chat {
  id: string
  name: string
  created_at: string
  running: boolean
}

export type DetailMode = 'collapsed' | 'list' | 'current' | 'expanded'
export type ChatWidth = '40em' | '55em' | '80em' | 'full'
export type Theme = 'dark' | 'light'
export type ConnectionStatus = 'connected' | 'degraded' | 'disconnected'
