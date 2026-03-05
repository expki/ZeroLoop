import { create } from 'zustand'
import type { Project, ProjectFile, FileTreeNode, MainView } from '../types'
import { api } from '../services/api'
import { ws } from '../services/websocket'

interface ProjectState {
  projects: Project[]
  selectedProjectId: string | null
  files: ProjectFile[]
  mainView: MainView
  initialized: boolean

  init: () => void
  loadProjects: () => Promise<void>
  createProject: (name: string, description?: string) => Promise<void>
  deleteProject: (id: string) => Promise<void>
  updateProject: (id: string, data: { name?: string; description?: string }) => Promise<void>
  selectProject: (id: string | null) => void
  loadFiles: (projectId: string) => Promise<void>
  setMainView: (view: MainView) => void
  openFile: (path: string) => void
  getFileTree: () => FileTreeNode[]
}

function buildFileTree(files: ProjectFile[]): FileTreeNode[] {
  const root: FileTreeNode[] = []
  const dirMap = new Map<string, FileTreeNode>()

  // Sort files so directories come first, then alphabetically
  const sorted = [...files].sort((a, b) => {
    if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
    return a.path.localeCompare(b.path)
  })

  for (const file of sorted) {
    const parts = file.path.split('/')
    const node: FileTreeNode = {
      name: file.name,
      path: file.path,
      isDir: file.is_dir,
      size: file.size,
      children: file.is_dir ? [] : undefined,
    }

    if (parts.length === 1) {
      root.push(node)
      if (file.is_dir) dirMap.set(file.path, node)
    } else {
      const parentPath = parts.slice(0, -1).join('/')
      const parent = dirMap.get(parentPath)
      if (parent && parent.children) {
        parent.children.push(node)
      } else {
        root.push(node)
      }
      if (file.is_dir) dirMap.set(file.path, node)
    }
  }

  return root
}

export const useProjectStore = create<ProjectState>((set, get) => ({
  projects: [],
  selectedProjectId: null,
  files: [],
  mainView: { type: 'chat' },
  initialized: false,

  init: () => {
    if (get().initialized) return
    set({ initialized: true })

    get().loadProjects()

    // Listen for file events from WebSocket
    ws.on('file_event', (payload: { project_id: string; path: string; name: string; size: number; action: string }) => {
      const state = get()
      if (payload.project_id !== state.selectedProjectId) return

      set((s) => {
        let files = [...s.files]
        if (payload.action === 'deleted') {
          files = files.filter((f) => f.path !== payload.path)
        } else {
          const existing = files.findIndex((f) => f.path === payload.path)
          const fileEntry: ProjectFile = {
            id: '',
            project_id: payload.project_id,
            path: payload.path,
            name: payload.name,
            is_dir: false,
            size: payload.size,
            mime_type: '',
          }
          if (existing >= 0) {
            files[existing] = { ...files[existing], size: payload.size }
          } else {
            files.push(fileEntry)
          }
        }
        return { files }
      })
    })
  },

  loadProjects: async () => {
    try {
      const projects = await api.listProjects()
      set({ projects: projects || [] })
    } catch {
      set({ projects: [] })
    }
  },

  createProject: async (name, description) => {
    try {
      const project = await api.createProject(name, description)
      set((s) => ({
        projects: [project, ...s.projects],
        selectedProjectId: project.id,
        files: [],
        mainView: { type: 'chat' },
      }))
    } catch (err) {
      console.error('Failed to create project:', err)
    }
  },

  deleteProject: async (id) => {
    try {
      await api.deleteProject(id)
      set((s) => ({
        projects: s.projects.filter((p) => p.id !== id),
        selectedProjectId: s.selectedProjectId === id ? null : s.selectedProjectId,
        files: s.selectedProjectId === id ? [] : s.files,
        mainView: s.selectedProjectId === id ? { type: 'chat' } : s.mainView,
      }))
    } catch (err) {
      console.error('Failed to delete project:', err)
    }
  },

  updateProject: async (id, data) => {
    try {
      const updated = await api.updateProject(id, data)
      set((s) => ({
        projects: s.projects.map((p) => (p.id === id ? { ...p, ...updated } : p)),
      }))
    } catch (err) {
      console.error('Failed to update project:', err)
    }
  },

  selectProject: (id) => {
    set({ selectedProjectId: id, files: [], mainView: { type: 'chat' } })
    if (id) {
      get().loadFiles(id)
    }
  },

  loadFiles: async (projectId) => {
    try {
      const files = await api.listProjectFiles(projectId)
      set({ files: files || [] })
    } catch {
      set({ files: [] })
    }
  },

  setMainView: (view) => set({ mainView: view }),

  openFile: (path) => set({ mainView: { type: 'editor', filePath: path } }),

  getFileTree: () => buildFileTree(get().files),
}))
