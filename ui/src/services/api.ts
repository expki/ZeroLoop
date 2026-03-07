import type { Agent, AgentType, AgentMode, Message, Process, ProcessLogLine, Project, ProjectFile, Terminal } from '../types'

const API_BASE = '/api'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

export const api = {
  // Project endpoints
  listProjects: () => request<Project[]>('/projects'),

  createProject: (name: string) =>
    request<Project>('/projects', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  getProject: (id: string) => request<Project>(`/projects/${id}`),

  updateProject: (id: string, data: { name?: string }) =>
    request<Project>(`/projects/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  deleteProject: (id: string) =>
    request<void>(`/projects/${id}`, { method: 'DELETE' }),

  // Project file endpoints
  listProjectFiles: (projectId: string) =>
    request<ProjectFile[]>(`/projects/${projectId}/files`),

  readProjectFile: (projectId: string, path: string) =>
    request<{ content: string; file: ProjectFile }>(`/projects/${projectId}/files/${path}`),

  createProjectFile: (projectId: string, path: string, content: string) =>
    request<ProjectFile>(`/projects/${projectId}/files/${path}`, {
      method: 'POST',
      body: JSON.stringify({ content }),
    }),

  updateProjectFile: (projectId: string, path: string, content: string) =>
    request<ProjectFile>(`/projects/${projectId}/files/${path}`, {
      method: 'PUT',
      body: JSON.stringify({ content }),
    }),

  deleteProjectFile: (projectId: string, path: string) =>
    request<void>(`/projects/${projectId}/files/${path}`, { method: 'DELETE' }),

  moveProjectFile: (projectId: string, from: string, to: string) =>
    request<{ from: string; to: string }>(`/projects/${projectId}/files/_move`, {
      method: 'POST',
      body: JSON.stringify({ from, to }),
    }),

  createProjectDir: (projectId: string, path: string) =>
    request<ProjectFile>(`/projects/${projectId}/files/${path}`, {
      method: 'POST',
      body: JSON.stringify({ is_dir: true }),
    }),

  uploadProjectFiles: async (projectId: string, files: FileList): Promise<ProjectFile[]> => {
    const formData = new FormData()
    for (let i = 0; i < files.length; i++) {
      formData.append('files', files[i])
    }
    const res = await fetch(`${API_BASE}/projects/${projectId}/upload`, {
      method: 'POST',
      body: formData,
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: res.statusText }))
      throw new Error(err.error || res.statusText)
    }
    return res.json()
  },

  // Agent endpoints (project-scoped)
  listAgents: (projectId?: string) =>
    request<Agent[]>(projectId ? `/agents?project_id=${projectId}` : '/agents'),

  createAgent: (projectId: string, name?: string, type?: AgentType, mode?: AgentMode) =>
    request<Agent>('/agents', {
      method: 'POST',
      body: JSON.stringify({ name: name || 'New Agent', project_id: projectId, type, mode }),
    }),

  getAgent: (id: string) => request<Agent>(`/agents/${id}`),

  getAgentChildren: (id: string) => request<Agent[]>(`/agents/${id}/children`),

  deleteAgent: (id: string) =>
    request<void>(`/agents/${id}`, { method: 'DELETE' }),

  getAgentMessages: (id: string) => request<Message[]>(`/agents/${id}/messages`),

  renameAgent: (id: string, name: string) =>
    request<Agent>(`/agents/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ name }),
    }),

  exportAgent: (id: string) =>
    request<{ agent: Agent; messages: Message[] }>(`/agents/${id}/export`, {
      method: 'POST',
    }),

  branchAgent: (id: string, messageNo?: number) =>
    request<Agent>(`/agents/${id}/branch`, {
      method: 'POST',
      body: JSON.stringify({ message_no: messageNo || 0 }),
    }),

  // Terminal endpoints (project-scoped)
  listTerminals: (projectId?: string) =>
    request<Terminal[]>(projectId ? `/terminals?project_id=${projectId}` : '/terminals'),

  createTerminal: (projectId: string, name?: string) =>
    request<Terminal>('/terminals', {
      method: 'POST',
      body: JSON.stringify({ name: name || 'Terminal', project_id: projectId }),
    }),

  deleteTerminal: (id: string) =>
    request<void>(`/terminals/${id}`, { method: 'DELETE' }),

  renameTerminal: (id: string, name: string) =>
    request<Terminal>(`/terminals/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ name }),
    }),

  // Process endpoints
  listProcesses: (projectId: string) =>
    request<Process[]>(`/processes?project_id=${projectId}`),

  getProcessLog: (processId: string, tail?: number) =>
    request<{ process: Process; lines: ProcessLogLine[] }>(
      `/processes/${processId}/log${tail ? `?tail=${tail}` : ''}`
    ),

  stopProcess: (processId: string) =>
    request<{ status: string }>(`/processes/${processId}/stop`, { method: 'POST' }),

  searchProjectFiles: (projectId: string, query: string, max?: number) =>
    request<{ path: string; line: number; column: number; content: string }[]>(
      `/projects/${projectId}/search?q=${encodeURIComponent(query)}${max ? `&max=${max}` : ''}`
    ),

  checkLLMHealth: () => request<{ status: string }>('/health/llm'),

  codeComplete: async (
    prefix: string,
    suffix: string,
    maxTokens: number,
    signal?: AbortSignal
  ): Promise<{ text: string }> => {
    try {
      const res = await fetch(`${API_BASE}/completions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ prefix, suffix, max_tokens: maxTokens }),
        signal,
      })
      if (!res.ok) return { text: '' }
      return res.json()
    } catch {
      return { text: '' }
    }
  },

  smartCodeComplete: async (
    prefix: string,
    suffix: string,
    filename: string,
    signal?: AbortSignal
  ): Promise<{ text: string; skipped: boolean }> => {
    try {
      const res = await fetch(`${API_BASE}/completions/smart`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ prefix, suffix, filename }),
        signal,
      })
      if (!res.ok) return { text: '', skipped: true }
      return res.json()
    } catch {
      return { text: '', skipped: true }
    }
  },
}
