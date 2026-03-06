import type { Chat, Message, Project, ProjectFile } from '../types'

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

  createProject: (name: string, description?: string) =>
    request<Project>('/projects', {
      method: 'POST',
      body: JSON.stringify({ name, description: description || '' }),
    }),

  getProject: (id: string) => request<Project>(`/projects/${id}`),

  updateProject: (id: string, data: { name?: string; description?: string }) =>
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

  // Chat endpoints (project-scoped)
  listChats: (projectId?: string) =>
    request<Chat[]>(projectId ? `/chats?project_id=${projectId}` : '/chats'),

  createChat: (projectId: string, name?: string) =>
    request<Chat>('/chats', {
      method: 'POST',
      body: JSON.stringify({ name: name || 'New Chat', project_id: projectId }),
    }),

  getChat: (id: string) => request<Chat>(`/chats/${id}`),

  deleteChat: (id: string) =>
    request<void>(`/chats/${id}`, { method: 'DELETE' }),

  getChatMessages: (id: string) => request<Message[]>(`/chats/${id}/messages`),

  renameChat: (id: string, name: string) =>
    request<Chat>(`/chats/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ name }),
    }),

  exportChat: (id: string) =>
    request<{ chat: Chat; messages: Message[] }>(`/chats/${id}/export`, {
      method: 'POST',
    }),

  branchChat: (id: string, messageNo?: number) =>
    request<Chat>(`/chats/${id}/branch`, {
      method: 'POST',
      body: JSON.stringify({ message_no: messageNo || 0 }),
    }),

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
}
