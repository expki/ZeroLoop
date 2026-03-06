import { create } from 'zustand'
import type { Project, ProjectFile, ArboristNode, MainView } from '../types'
import { api } from '../services/api'
import { ws } from '../services/websocket'
import { useEditorStore } from './editorStore'

interface ProjectState {
  projects: Project[]
  selectedProjectId: string | null
  files: ProjectFile[]
  mainView: MainView
  initialized: boolean

  init: () => void
  loadProjects: () => Promise<void>
  createProject: (name: string) => Promise<void>
  deleteProject: (id: string) => Promise<void>
  updateProject: (id: string, data: { name?: string }) => Promise<void>
  selectProject: (id: string | null) => void
  loadFiles: (projectId: string) => Promise<void>
  setMainView: (view: MainView) => void
  openFile: (path: string) => void
  getArboristTree: () => ArboristNode[]
}

function buildArboristTree(files: ProjectFile[]): ArboristNode[] {
  const root: ArboristNode[] = []
  const dirMap = new Map<string, ArboristNode>()

  const sorted = [...files].sort((a, b) => {
    if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
    return a.path.localeCompare(b.path)
  })

  for (const file of sorted) {
    const parts = file.path.split('/')
    const node: ArboristNode = {
      id: file.path,
      name: file.name,
      isDir: file.is_dir,
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
  mainView: { type: 'agent' },
  initialized: false,

  init: () => {
    if (get().initialized) return
    set({ initialized: true })

    get().loadProjects()

    // Listen for file events from WebSocket
    ws.on('file_event', (payload: { project_id: string; path: string; name: string; size: number; action: string; old_path?: string }) => {
      const state = get()
      if (payload.project_id !== state.selectedProjectId) return

      // Forward file events to editor store for tab management
      useEditorStore.getState().handleFileEvent(payload.action, payload.path, payload.old_path)

      if (payload.action === 'renamed' && payload.old_path) {
        // Atomically remove old entries and reload from server
        set((s) => ({
          files: s.files.filter((f) => f.path !== payload.old_path && !f.path.startsWith(payload.old_path + '/')),
        }))
        get().loadFiles(payload.project_id)
        return
      }

      set((s) => {
        let files = [...s.files]
        if (payload.action === 'deleted') {
          files = files.filter((f) => f.path !== payload.path && !f.path.startsWith(payload.path + '/'))
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

  createProject: async (name) => {
    try {
      const project = await api.createProject(name)
      set((s) => ({
        projects: [project, ...s.projects],
        selectedProjectId: project.id,
        files: [],
        mainView: { type: 'agent' },
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
        mainView: s.selectedProjectId === id ? { type: 'agent' } : s.mainView,
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
    set({ selectedProjectId: id, files: [], mainView: { type: 'agent' } })
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

  openFile: (path) => {
    set({ mainView: { type: 'editor' } })
    const projectId = get().selectedProjectId
    if (projectId) {
      useEditorStore.getState().initForProject(projectId)
    }
    useEditorStore.getState().openTab(path)
  },

  getArboristTree: () => buildArboristTree(get().files),
}))
