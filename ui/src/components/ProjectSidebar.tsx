import { useState, useRef, useEffect } from 'react'
import { useProjectStore } from '../stores/projectStore'
import { useChatStore } from '../stores/chatStore'
import { useUIStore } from '../stores/uiStore'
import { api } from '../services/api'
import FileTree from './FileTree'
import type { ChatWidth, DetailMode } from '../types'
import './Sidebar.css'
import './ProjectSidebar.css'

function ProjectSidebar() {
  const { selectedProjectId, selectProject, projects, getFileTree, openFile, mainView, loadFiles } = useProjectStore()
  const { chats, selectedChatId, selectChat, createChat, deleteChat, loadChatsForProject } = useChatStore()
  const {
    sidebarOpen, toggleSidebar, theme, toggleTheme,
    chatWidth, setChatWidth, detailMode, setDetailMode, setSidebarOpen,
  } = useUIStore()
  const [prefsOpen, setPrefsOpen] = useState(false)
  const [showNewFile, setShowNewFile] = useState(false)
  const [newFileName, setNewFileName] = useState('')
  const newFileRef = useRef<HTMLInputElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const project = projects.find((p) => p.id === selectedProjectId)

  // Load chats and files when project changes
  useEffect(() => {
    if (selectedProjectId) {
      loadChatsForProject(selectedProjectId)
      loadFiles(selectedProjectId)
    }
  }, [selectedProjectId, loadChatsForProject, loadFiles])

  useEffect(() => {
    if (showNewFile && newFileRef.current) {
      newFileRef.current.focus()
    }
  }, [showNewFile])

  const handleCreateFile = async () => {
    const name = newFileName.trim()
    if (!name || !selectedProjectId) return
    try {
      await api.createProjectFile(selectedProjectId, name, '')
      setNewFileName('')
      setShowNewFile(false)
      loadFiles(selectedProjectId)
    } catch (err) {
      console.error('Failed to create file:', err)
    }
  }

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || !selectedProjectId) return
    try {
      await api.uploadProjectFiles(selectedProjectId, e.target.files)
      loadFiles(selectedProjectId)
    } catch (err) {
      console.error('Failed to upload files:', err)
    }
    e.target.value = ''
  }

  const handleSelectChat = (id: string) => {
    selectChat(id)
    useProjectStore.getState().setMainView({ type: 'chat' })
    if (window.innerWidth < 768) {
      setSidebarOpen(false)
    }
  }

  const handleNewChat = () => {
    if (selectedProjectId) {
      createChat(selectedProjectId)
      useProjectStore.getState().setMainView({ type: 'chat' })
    }
  }

  const editorPath = mainView.type === 'editor' ? mainView.filePath : undefined

  const widthOptions: { label: string; value: ChatWidth }[] = [
    { label: 'S', value: '40em' },
    { label: 'M', value: '55em' },
    { label: 'L', value: '80em' },
    { label: 'Full', value: 'full' },
  ]

  const detailOptions: { label: string; value: DetailMode }[] = [
    { label: 'Hide', value: 'collapsed' },
    { label: 'List', value: 'list' },
    { label: 'Current', value: 'current' },
    { label: 'All', value: 'expanded' },
  ]

  return (
    <aside className={`sidebar ${sidebarOpen ? 'open' : 'closed'}`}>
      <div className="sidebar-header">
        <button className="sidebar-brand" onClick={() => selectProject(null)}>
          <span className="brand-icon material-symbols-outlined">hub</span>
          <span className="brand-text">ZeroLoop</span>
        </button>
        <button className="icon-button" onClick={toggleSidebar} title="Toggle sidebar">
          <span className="material-symbols-outlined">menu</span>
        </button>
      </div>

      {project && (
        <div className="project-name-bar">
          <span className="material-symbols-outlined" style={{ fontSize: '18px' }}>folder</span>
          <span className="project-name-text">{project.name}</span>
        </div>
      )}

      {/* Files section */}
      <div className="sidebar-section">
        <div className="sidebar-section-header">
          <span className="sidebar-section-title">Files</span>
          <div className="sidebar-section-actions">
            <button className="icon-button" onClick={() => setShowNewFile(true)} title="New file">
              <span className="material-symbols-outlined" style={{ fontSize: '18px' }}>add</span>
            </button>
            <button className="icon-button" onClick={() => fileInputRef.current?.click()} title="Upload files">
              <span className="material-symbols-outlined" style={{ fontSize: '18px' }}>upload</span>
            </button>
            <input
              ref={fileInputRef}
              type="file"
              multiple
              style={{ display: 'none' }}
              onChange={handleUpload}
            />
          </div>
        </div>
        {showNewFile && (
          <div className="new-file-row">
            <input
              ref={newFileRef}
              className="new-file-input"
              placeholder="filename.ext"
              value={newFileName}
              onChange={(e) => setNewFileName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleCreateFile()
                if (e.key === 'Escape') setShowNewFile(false)
              }}
              onBlur={() => { if (!newFileName.trim()) setShowNewFile(false) }}
            />
          </div>
        )}
        <FileTree nodes={getFileTree()} onSelect={openFile} selectedPath={editorPath} />
      </div>

      {/* Chats section */}
      <div className="sidebar-section sidebar-section-grow">
        <div className="sidebar-section-header">
          <span className="sidebar-section-title">Chats</span>
          <button className="icon-button" onClick={handleNewChat} title="New chat">
            <span className="material-symbols-outlined" style={{ fontSize: '18px' }}>add</span>
          </button>
        </div>
        <div className="chat-list">
          {chats.length === 0 ? (
            <div className="chat-list-empty">No chats yet</div>
          ) : (
            chats.map((chat) => (
              <div
                key={chat.id}
                className={`chat-item ${selectedChatId === chat.id ? 'active' : ''}`}
                onClick={() => handleSelectChat(chat.id)}
              >
                <div className={`chat-status-dot ${chat.running ? 'running' : ''}`} />
                <span className="chat-name">{chat.name}</span>
                <button
                  className="chat-close icon-button"
                  onClick={(e) => { e.stopPropagation(); deleteChat(chat.id) }}
                  title="Remove chat"
                >
                  <span className="material-symbols-outlined">close</span>
                </button>
              </div>
            ))
          )}
        </div>
      </div>

      <div className="sidebar-bottom">
        <div className="prefs-section">
          <button className="prefs-toggle" onClick={() => setPrefsOpen(!prefsOpen)}>
            <span className="material-symbols-outlined">settings</span>
            <span>Preferences</span>
            <span className={`material-symbols-outlined chevron ${prefsOpen ? 'open' : ''}`}>
              expand_more
            </span>
          </button>

          {prefsOpen && (
            <div className="prefs-content">
              <div className="pref-row">
                <span className="pref-label">Theme</span>
                <button className="toggle-switch" onClick={toggleTheme}>
                  <span className="material-symbols-outlined" style={{ fontSize: '16px' }}>
                    {theme === 'dark' ? 'dark_mode' : 'light_mode'}
                  </span>
                  <span>{theme === 'dark' ? 'Dark' : 'Light'}</span>
                </button>
              </div>

              <div className="pref-row">
                <span className="pref-label">Width</span>
                <div className="button-group">
                  {widthOptions.map((opt) => (
                    <button
                      key={opt.value}
                      className={`btn-group-item ${chatWidth === opt.value ? 'active' : ''}`}
                      onClick={() => setChatWidth(opt.value)}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              </div>

              <div className="pref-row">
                <span className="pref-label">Detail</span>
                <div className="button-group">
                  {detailOptions.map((opt) => (
                    <button
                      key={opt.value}
                      className={`btn-group-item ${detailMode === opt.value ? 'active' : ''}`}
                      onClick={() => setDetailMode(opt.value)}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>

        <div className="sidebar-footer">
          <span className="version-text">ZeroLoop v0.1.0</span>
        </div>
      </div>
    </aside>
  )
}

export default ProjectSidebar
