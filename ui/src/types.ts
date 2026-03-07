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
  agent_id?: string
  no: number
  type: MessageType
  heading: string
  content: string
  kvps?: Record<string, string> | string
  timestamp: string | number
  agentno: number
  stream?: boolean
}

export type AgentType = 'standard' | 'automated'
export type AgentMode = 'direct' | 'orchestrator' | 'oneshot' | 'infinite'
export type AgentStatus = 'idle' | 'running' | 'completed' | 'failed' | 'paused'

export interface Agent {
  id: string
  project_id: string
  name: string
  type: AgentType
  mode: AgentMode
  status: AgentStatus
  parent_id?: string
  created_at: string
  running: boolean
}

export interface Project {
  id: string
  name: string
  created_at: string
  updated_at: string
  agents?: Agent[]
}

export interface Terminal {
  id: string
  project_id: string
  name: string
  created_at: string
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

export interface ArboristNode {
  id: string
  name: string
  isDir: boolean
  children?: ArboristNode[]
}

export interface Process {
  id: string
  project_id: string
  command: string
  status: 'running' | 'exited' | 'stopped'
  exit_code?: number
  started_at: string
  exited_at?: string
}

export interface ProcessLogLine {
  timestamp: string
  stream: 'stdout' | 'stderr'
  text: string
}

export type MainView =
  | { type: 'agent' }
  | { type: 'terminal' }
  | { type: 'editor' }
  | { type: 'process' }

export type DetailMode = 'collapsed' | 'list' | 'current' | 'expanded'
export type AgentWidth = '40em' | '55em' | '80em' | 'full'
export type Theme = 'dark' | 'light'
export type ConnectionStatus = 'connected' | 'degraded' | 'disconnected'
