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
  | 'notification'

export interface Message {
  id: string
  chat_id?: string
  no: number
  type: MessageType
  heading: string
  content: string
  kvps?: Record<string, string> | string
  timestamp: string | number
  agentno: number
  stream?: boolean
}

export interface Chat {
  id: string
  project_id: string
  name: string
  created_at: string
  running: boolean
}

export interface Project {
  id: string
  name: string
  description: string
  created_at: string
  updated_at: string
  chats?: Chat[]
}

export interface ProjectFile {
  id: string
  project_id: string
  path: string
  name: string
  is_dir: boolean
  size: number
  mime_type: string
}

export interface FileTreeNode {
  name: string
  path: string
  isDir: boolean
  size: number
  children?: FileTreeNode[]
}

export type MainView =
  | { type: 'chat' }
  | { type: 'editor'; filePath: string }

export type DetailMode = 'collapsed' | 'list' | 'current' | 'expanded'
export type ChatWidth = '40em' | '55em' | '80em' | 'full'
export type Theme = 'dark' | 'light'
export type ConnectionStatus = 'connected' | 'degraded' | 'disconnected'
